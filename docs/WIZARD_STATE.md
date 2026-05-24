# Wizard state (state.json)

Декларативная модель Configurator: где лежит, как загружается, как сохраняется,
куда уходит при build. Файл переписан под schema v6 (SPEC 053 + SPEC 056-R-N
+ SPEC 057-R-N), v5 описан только в разделе «Миграции».

---

## 1. Файлы и расположение

- **`bin/wizard_states/state.json`** — текущий снимок. Единственный файл,
  читаемый при старте Configurator'а и при headless rebuild config.json.
- **`bin/wizard_states/<id>.json`** — именованные снимки (Save As).
  Структурно идентичны `state.json`; при Read копируются поверх `state.json`.
- **`bin/subscriptions/<source_id>.raw`** — per-source raw body cache подписки
  (atomic .tmp + rename). Read-path парсит .raw напрямую без сети.

ExecDir resolve описан в SPECS/022 (macOS app support directories). На macOS
release-сборке это `~/Library/Application Support/singbox-launcher/bin/...`,
в dev-сборке — рядом с бинарём.

---

## 2. Top-level schema v6 (canonical)

```jsonc
{
  "meta": {
    "version": 6,
    "schema":  "presets_v1",
    "comment": "...",
    "created_at": "RFC3339 UTC",
    "updated_at": "RFC3339 UTC"
  },

  "connections": {
    "sources":   [ ... ],     // per-source subscription / server entries
    "outbounds": [ ... ],     // global outbound selectors / urltests
    "defaults":  { "reload": "4h", "max_nodes": 3000 }
  },

  "rules": [
    { "kind": "preset", "ref": "...",  "enabled": true, "body": { "vars": {} } },
    { "kind": "inline", "id":  "...",  "enabled": true, "body": { "name": "...", "match": {}, "outbound": "..." } },
    { "kind": "srs",    "id":  "...",  "enabled": true, "body": { "name": "...", "srs_url": "...", "outbound": "..." } }
  ],

  "vars": [
    { "name": "tun",          "value": "true" },
    { "name": "dns_strategy", "value": "prefer_ipv4" },
    ...
  ],

  "dns_options": {
    "strategy":                "...",   // optional fallback; source of truth — vars[]
    "final":                   "...",
    "default_domain_resolver": "...",
    "servers": [
      { "kind": "template", "tag": "...",        "enabled": true  },
      { "kind": "preset",   "ref": "<pid>:<tag>", "enabled": true },
      { "kind": "user",     "tag": "...", "type": "...", "server": "...", "enabled": true, ... }
    ],
    "rules": [
      { "kind": "preset", "ref": "<pid>", "enabled": true },
      { "kind": "user",   "enabled": true, ... }
    ]
  }
}
```

Top-level keys, отсутствующие в v6 (vs предыдущих ревизий):
`id` (snapshot-имя живёт в имени файла), `config_params`, `custom_rules`,
`selectable_rule_states`, `rules_library_merged`, `dns_options.independent_cache`.

---

## 3. Per-block storage rules

| Секция | Содержит | Источник истины | Кто пишет | Кто читает |
|--------|----------|-----------------|-----------|------------|
| `connections.sources` | Source entries (subscription URL или server URI), per-source meta (profile_title, userinfo, last_status), update spec | state | UI Sources tab (`source_tab`), Update flow (после fetch) | parser pipeline, UI dashboard, build |
| `connections.outbounds` | Global selectors/urltest entries, в т.ч. preset-bound (`ref`) и preset-patched (`updates[]`) | state | UI Outbounds tab, `SyncOutboundsWithActivePresets`, presenter `CreateStateFromModel` | build (`ResolveOutbounds` + `MergeOutboundUpdatesInPlace`), UI render |
| `connections.defaults` | reload interval, max_nodes per source default | state | UI Settings/Sources | parser pipeline |
| `rules` | Routing rules через kind discriminator (preset/inline/srs) — единый упорядоченный массив | state | UI Rules tab (drag, library add, edit) | build (`MergeRouteSection` + `MergePresetsIntoRoute`), UI render |
| `vars` | Overrides для всех объявленных в template vars: tun, route_final, dns_*, clash_secret, etc. | state (значения) + template (объявления) | UI Settings tab, скрытые синхронизаторы (`SyncDNSModelToSettingsVars`) | build (`@var` substitute) |
| `dns_options.servers` | Entries kind=template / preset / user; body для template/preset резолвится из template, для user — flat в entry | state (что включено) + template (тело) | UI DNS tab, `SyncDNSOptionsWithActivePresets`, presenter | build (`ResolveDNS` → `MergeDNSSection`), UI render |
| `dns_options.rules` | Entries kind=preset / user. preset = thin ref на `template.presets[].dns_rule`, user = flat body | state + template | UI DNS tab, lifecycle sync, presenter | build (`ResolveDNS`), UI render |

«Источник истины» = откуда берётся семантика записи. «Кто пишет» = в каких
точках кода mutates state. «Кто читает» = consumers при build/render.

---

## 4. Outbound preset binding (SPEC 057-R-N)

Outbound entries в `connections.outbounds[]` могут быть тремя типами:
обычный global (template или user), **preset-add** (entry создан preset'ом),
и **target of preset-update** (на entry накладывается стек patches).

### 4.1 Schema на `OutboundConfig`

| Поле | Тип | Семантика |
|------|-----|-----------|
| `ref` | `string` (omitempty) | Не пусто → entry создан через `preset.outbounds[mode=add]`. Значение = `preset.id` владельца. UI: row read-only (View вместо Edit, без Del). На disable preset → entry удаляется. |
| `updates` | `[]OutboundUpdate` (omitempty) | Стек patches от `preset.outbounds[mode=update]` от разных preset'ов. Каждая запись — `{ref: <preset_id>, patch: <fields>}`. Финальное body на emit = base + apply patches в order. |
| `required` | (не в struct) | Template-only флаг, читается live из `template.parser_config.outbounds[]` на render. State.json не персистит — иначе нельзя было бы снять флаг в template и увидеть эффект. |

`OutboundUpdate{Ref, Patch map[string]interface{}}` определён рядом
(`core/config/configtypes/types.go`).

### 4.2 Lifecycle: `SyncOutboundsWithActivePresets`

Единая точка добавления/удаления preset entries. Idempotent.

Вызывается:
- На Load после `parseV6` (presenter `LoadState`)
- На каждый preset toggle в Rules tab (через `RefreshAfterPresetToggle`)
- Перед Save в `CreateStateFromModel` — на **обе view'а**
  (`state.Connections.Outbounds` и `state.ParserConfig.ParserConfig.Outbounds`),
  потому что `syncConnectionsFromLegacy` копирует legacy view → canonical,
  иначе sync'нутые `updates[]` затирались бы адаптером
- В headless runtime path'ах: `rebuild_raw_cache`,
  `UpdateConfigFromSubscriptions`, `parseAndPreview`

Семантика: для каждого активного `state.rules[kind=preset, enabled=true]`
прогоняет `preset.outbounds[]`; mode=add → ensure entry с `ref=preset.id`,
mode=update → ensure `OutboundUpdate{ref, patch}` в `updates[]` target'а.
Entries и patches с ref на disabled/missing preset удаляются.

### 4.3 Adopt-on-first-sync (legacy state)

При первом sync на состоянии pre-SPEC-057 (или после ручного promote-to-global)
existing global без `Ref` с tag, совпадающим с expected preset add — adopt'ится:
ему проставляется `Ref = preset.id`, body preservируется. Без этого
шиппнутые юзеры теряли бы preset binding на апгрейде.

### 4.4 Runtime merge: `MergeOutboundUpdatesInPlace`

Native generator (`GenerateOutboundsFromParserConfig`) не знает про
поле `Updates`. Перед его вызовом build pipeline вызывает
`MergeOutboundUpdatesInPlace(parserCfg)` — walks `parserCfg.Outbounds[]`,
для каждой entry с непустым `Updates[]` стеком заменяет body на merged
(base + apply patches в order). Mutates in-place (через deep-copy на
сайт-edge, model не trash'ится).

Места вызова — те же 3 headless path'а из 4.2 (см. `core/build/resolve_outbounds.go`
комментарии). UI-preview flow разделяет unmerged (для model) и merged
(`parserConfigForGen`) — Save пишет правильный state shape с `updates[]`,
generator получает flat'нутую копию.

---

## 5. DNS preset binding (SPEC 056-R-N)

Симметрично outbound binding. `dns_options.servers[]` и `dns_options.rules[]`
— flat array с `kind` discriminator.

### 5.1 `dns_options.servers[]` — kind

| `kind` | Identity | Body |
|--------|----------|------|
| `template` | `tag` (ссылка в `template.dns_options.servers[tag]`) | резолвится из template на build/render |
| `preset` | `ref = "<preset_id>:<local_tag>"` (ссылка на `template.presets[id].dns_servers[local_tag]` + `vars` substitute) | резолвится из template + apply preset vars |
| `user` | `tag` + flat body (type/server/server_port/tls/...) — полная sing-box DNS server spec | self-contained |

Toggle `enabled` доступен для всех трёх kind'ов; edit body — только для user;
delete — только для user (template/preset управляются template'ом и preset
toggle'ом).

### 5.2 `dns_options.rules[]` — kind

| `kind` | Identity | Body |
|--------|----------|------|
| `preset` | `ref = "<preset_id>"` (один dns_rule на preset максимум) | резолвится из `template.presets[id].dns_rule` |
| `user` | flat body (rule_set/server/domain_*/ip_cidr/port/network/...) | self-contained |

### 5.3 Lifecycle: `SyncDNSOptionsWithActivePresets`

Единая точка lifecycle для kind=preset entries. Аналогично outbound sync.

Вызывается из presenter'а на тех же триггерах: Load, preset toggle, перед Save.
Семантика: enable preset → создаются entries `{kind:preset, ref}` для каждого
`template.presets[id].dns_servers[]` + (если есть) `dns_rule`. Default
`Enabled=true`. Disable preset → все entries с ref на этот preset удаляются.
Per-server toggle внутри активного preset (юзер может скрыть отдельный
сервер из bundle) преserve'ится при sync.

Реализация: `core/state/v6/sync_dns.go::SyncDNSOptionsWithActivePresets`.

### 5.4 Required entries (template)

`template.dns_options.servers[]` может пометить entry как `"required": true`
(например, `local_dns_resolver` / `direct_dns_resolver`). Render показывает
галку enabled + lock на toggle/edit/del; build всегда эмитит. Флаг — template-only,
state не персистит — читается live на каждый render через
`wizardbusiness.DNSTagLocked(model, tag)`.

### 5.5 Удалённые поля (sing-box 1.14)

`independent_cache` — deprecated в sing-box 1.14 (кэш теперь всегда
per-transport). Legacy state c этим ключом парсится без ошибок (silently
dropped через `_ = raw.IndependentCache` в `legacyDevDNSToOptions`),
новые saves поле не пишут.

---

## 6. Rule preset binding (SPEC 053)

`rules[]` — единый упорядоченный массив через `kind` discriminator.

| `kind` | Header | Body |
|--------|--------|------|
| `preset` | `{ref, enabled}` (ref = `<preset_id>`) | `{vars: {<name>: <value>, ...}}` — только diff от template defaults; пустой map = всё дефолтное |
| `inline` | `{id (ULID), enabled}` | `{name, match (sing-box match-объект), outbound (tag|"reject"|"drop")}` |
| `srs` | `{id (ULID), enabled}` | `{name, srs_url, outbound}` |

Order = order рендера в UI Rules tab (включая drag-reordering) = order эмита
в `config.json::route.rules[]`. Сохраняется через
`SyncRulesByOrderToStateRulesV6(model.RuleOrder, ...)` в `CreateStateFromModel`.

Match-поля и rule_set'ы для kind=preset живут **в template** — bump
`RequiredTemplateRef` → юзеры автоматически получают новые match-поля.
Body хранит только diff vars; пустой `vars: {}` = preset на template defaults.

См. `core/state/v6/rule_types.go` (DecodeBody dispatcher) +
`core/build/preset_expand.go` (build-time substitute + tag-prefix).

---

## 7. Data flow

### 7.1 Load: `state.json` → model

```
disk: bin/wizard_states/state.json
        │
        ▼
core/state.Load(path)
        │   probe meta.version  →  parseV6 (или parseV5 / parseLegacy)
        │   legacyDevDNSToOptions if старый dev-shape `dns.{template_servers,extras}`
        ▼
state.State{Connections, RulesV6, DNS, Vars, ...}
        │
        ▼
presenter.LoadState(stateFile)
        │   restoreParserConfig (legacy view)
        │   restoreConfigParams + restoreDNS
        │   ApplyRulesLibraryMigration (legacy v3→v5 idempotent)
        │   restoreCustomRules + restorePresetRefs (kind=preset)
        │   SyncOutboundsWithActivePresets(model.GlobalOutbounds)   ← adopt-on-first-sync
        │   RefreshDerivedParserConfig
        ▼
model.WizardModel  (Sources, GlobalOutbounds, CustomRules, PresetRefs,
                    DNSServers, DNSRulesText, SettingsVars, RuleOrder)
        │
        ▼
SyncModelToGUI + RefreshOutboundOptions
```

### 7.2 Save: model → `state.json`

```
model.WizardModel
        │
        ▼
presenter.CreateStateFromModel(comment, id)
        │   SyncGUIToModel
        │   build WizardStateFile (legacy view + Connections canonical)
        │   ReconcileRuleOrder + SyncRulesByOrderToStateRulesV6  → state.RulesV6
        │   SyncDNSFullToStateV6                                  → state.DNS
        │   v6.SyncDNSOptionsWithActivePresets(state.RulesV6, &state.DNS, presets)
        │   applyPresetEnabledOverrides (UI toggle → entry.Enabled)
        │   build.SyncOutboundsWithActivePresets ×2 view (Connections + ParserConfig)  ◄── обязательно на обе!
        ▼
state.State.Save(path)
        │   syncConnectionsFromLegacy (ParserConfig → Connections; уже sync'нутая версия побеждает)
        │   hasPresetRefs(RulesV6) ? marshalDiskV6 : marshalDisk (v5)
        │   atomic write (.tmp + Rename) + fsync
        ▼
disk: bin/wizard_states/state.json
```

Двойной sync на обе view (`Connections.Outbounds` + `ParserConfig.Outbounds`)
— ключевой момент: без него `syncConnectionsFromLegacy` затирал бы только что
вычисленные `updates[]` стеки.

### 7.3 Build/Emit: state → `bin/config.json`

```
state.State (после Load или после CreateStateFromModel)
        │
        ▼
core/build (entry: BuildAndWriteConfig / ApplyTemplate)
        │   ResolveDNS(state, template, vars)        ◄── pure func
        │   ResolveRoute(state, template, vars)      ◄── pure func
        │   ResolveOutbounds(state, template)        ◄── pure func
        │   MergeOutboundUpdatesInPlace(parserCfg)   ◄── материализует Updates[] в body для generator'а
        │   GenerateOutboundsFromParserConfig
        │   MergeDNSSection + MergeRouteSection
        │   MergePresetsIntoRoute (per-preset expand: substitute + tag prefix)
        ▼
disk: bin/config.json (sing-box-compatible)
```

Resolve* функции — single source of truth для UI и build (нет divergence
между preview и финальным config).

---

## 8. Required vs preset-locked entries

Три класса entries в UI с разной семантикой управления:

| Класс | Где маркер | Толкование | UI controls |
|-------|------------|------------|-------------|
| **Required (template)** | `template.*.entries[].required = true` (live read, в state не персистится) | Mandatory entry — нельзя toggle/del. Body editable. | Reset (откат body к template defaults), Edit. **Del не рендерится.** |
| **Preset-locked** | `entry.ref != ""` (или `kind=preset` для DNS/rules) | Entry создан preset'ом, body резолвится из template + preset vars. | Toggle enabled (юзер может скрыть отдельный bundle item), View (read-only modal). **Edit / Del не рендерятся.** |
| **User / template global** | `ref == ""` + tag отсутствует в required set | Полный контроль. | Toggle, Edit, Up/Down, Del |

«Required» — про **lock на удаление и toggle**; «preset-locked» — про
**lock на edit body** (управляется через preset toggle в Rules tab).

---

## 9. Миграции

| From → To | Что мигрирует | Backup |
|-----------|---------------|--------|
| v2/v3/v4 → v5 | `selectable_rule_states` + `custom_rules` → единый `custom_rules[]` (rules library merge); `parser_config` wrapped → simplified; `enable_tun_macos` → `vars["tun"]`; `route.default_domain_resolver` → `vars["dns_default_domain_resolver"]` | нет (in-memory; пишется v5 при первом Save) |
| v5 → v6 | `custom_rules[]` → `rules[]` (kind=inline/srs derive из rule_set type); `dns_options.servers/rules` legacy → `dns_options.servers/rules` flat kind discriminator; meta bump | **`state.json.v5.bak`** на первом upgrade (когда появляется хотя бы один kind=preset rule) |
| v6 dev-shape → v6 flat | `dns.{template_servers, extra_servers, extra_rules}` (SPEC 053 промежуточный shape) → `dns_options.servers[]/rules[]` flat (SPEC 056-R-N) | нет (lossless, dev-only, не релизился) |
| sing-box 1.14 | `dns_options.independent_cache` silently dropped (legacy state читается, новый не пишется) | нет |

Save выбирает формат автоматически: `hasPresetRefs(RulesV6)` → v6, иначе v5.
Юзеры с pure inline/srs rules остаются на v5 пока не добавят первый preset.

---

## 10. Где лежит реализация

| Файл | Что |
|------|-----|
| `core/state/load.go` | `Load` / `Parse` / `parseV6` / `parseV5` / `parseLegacyAndMigrate` / `legacyDevDNSToOptions` |
| `core/state/save.go` | `Save` / `marshalDisk` (v5) / `marshalDiskV6` / `maybeBackupV5` |
| `core/state/adapter.go` | `syncConnectionsFromLegacy` / `syncLegacyFromConnections` (обмен legacy ParserConfig ↔ canonical Connections) |
| `core/state/v6/state.go` | v6 State struct + MetaSection |
| `core/state/v6/rule_types.go` | Rule + PresetBody/InlineBody/SrsBody + DecodeBody |
| `core/state/v6/dns_options.go` | DNSServer + DNSRule + flat Marshal/Unmarshal |
| `core/state/v6/sync_dns.go` | `SyncDNSOptionsWithActivePresets` |
| `core/state/v6/migration.go` | `MigrateV5ToV6` (pure func) |
| `core/build/sync_outbounds.go` | `SyncOutboundsWithActivePresets` (lifecycle) + `outboundConfigToPatchMap` |
| `core/build/resolve_outbounds.go` | `ResolveOutbounds` + `MergeOutboundUpdatesInPlace` (runtime helper) |
| `core/build/resolve_dns.go` | `ResolveDNS` (pure DNS view для UI + build) |
| `core/build/resolve_route.go` | `ResolveRoute` (pure route view) |
| `core/template/loader.go` | `LoadTemplateData` + `TemplateData` struct |
| `core/template/preset_types.go` | Preset / PresetVar / PresetRuleSet / PresetDNSServer / PresetOutbound |
| `ui/configurator/presentation/presenter_state.go` | `LoadState` + `CreateStateFromModel` (entry points для save/load) |
| `ui/configurator/presentation/presenter_sync.go` | `RefreshAfterPresetToggle` (presenter-level eager sync после Rules toggle) |

См. также: [TEMPLATE_REFERENCE.md](TEMPLATE_REFERENCE.md) — что лежит в
`wizard_template.json` и куда оно попадает в state/runtime/UI.
[DATA_FLOW.md](DATA_FLOW.md) — расширенные диаграммы load/save/build/toggle.
[CREATE_WIZARD_TEMPLATE.md](CREATE_WIZARD_TEMPLATE.md) — туториал для авторов
template'ов (формат preset'ов, vars, substitute, if/if_or).
