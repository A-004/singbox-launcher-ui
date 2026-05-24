# Data flow

Сводные диаграммы load / save / build / preset-toggle для Configurator'а
после SPEC 053 + 056-R-N + 057-R-N. Дополняет [WIZARD_STATE.md](WIZARD_STATE.md)
и [TEMPLATE_REFERENCE.md](TEMPLATE_REFERENCE.md) (там — спецификация секций
state и template; здесь — как они вместе двигаются по времени).

---

## 1. Load flow

`state.json` + `wizard_template.json` → `model.WizardModel` (in-memory) → UI.

```
launcher start
     │
     ▼
core/template_migration.InvalidateTemplateIfStale(execDir)
     │   compare RequiredTemplateRef vs cached marker
     │   mismatch → unlink bin/wizard_template.json
     ▼
extractEmbeddedTemplate (if file missing)
     │
     ▼
core/template.LoadTemplateData(execDir)
     │   read JSON
     │   ApplyParams(runtime.GOOS) → effective Config sections
     │   substitute @vars defaults
     │   ParsePresets + filter platforms
     ▼
model.TemplateData (immutable for session)
     │
     ▼─── path A: state.json exists ─────────────────────────┐
     │                                                       │
     │   core/state.Load(path)                               │
     │     probe meta.version                                │
     │     parseV6 / parseV5 / parseLegacyAndMigrate         │
     │     legacyDevDNSToOptions (if dev-shape `dns.{...}`)  │
     │   → state.State {Connections, RulesV6, DNS, Vars}     │
     │                                                       │
     │   presenter.LoadState(stateFile)                      │
     │     restoreParserConfig (legacy view)                 │
     │     MigrateSettingsVarsFromConfigParams (one-shot)    │
     │     restoreConfigParams + restoreDNS                  │
     │     ApplyRulesLibraryMigration (idempotent)           │
     │     restoreCustomRules + restorePresetRefs            │
     │     build.SyncOutboundsWithActivePresets             │  ◄── adopt-on-first-sync
     │       (state.RulesV6, &model.GlobalOutbounds, presets)│      legacy → preset-bound
     │     RefreshDerivedParserConfig                        │
     │                                                       │
     │   model.WizardModel populated                         │
     │                                                       │
     ▼─── path B: state.json missing (fresh install) ───────┤
     │                                                       │
     │   business.LoadConfigFromFile                         │
     │     prefer config.json @ParserConfig block            │
     │     fallback → template.parser_config                 │
     │   initializeWizardContent                             │
     │     InitializeTemplateState                           │
     │     ApplyWizardDNSTemplate (if DNS empty)             │
     │                                                       │
     ▼───────────────────────────────────────────────────────┘
     │
     ▼
SyncModelToGUI + RefreshOutboundOptions
     │
     ▼
UI renders (Sources / Outbounds / Rules / DNS / Settings tabs)
```

Ключевой момент: `SyncOutboundsWithActivePresets` на Load включает
**adopt-on-first-sync** — pre-SPEC-057 state (где preset-add outbounds
жили как обычные globals) получает корректный `Ref` без юзерского
вмешательства.

---

## 2. Save flow

`model.WizardModel` → `state.json` (atomic write).

```
trigger: Save button / autosave hook
     │
     ▼
presenter.CreateStateFromModel(comment, id)
     │   SyncGUIToModel                       — flush GUI widget values into model
     │   build WizardStateFile                — legacy ParserConfig + canonical Connections
     │   extractConfigParams                  — empty in v6 (vars moved to state.vars)
     │
     │   ReconcileRuleOrder(model)            — collapse RuleOrder vs PresetRefs/CustomRules
     │   SyncRulesByOrderToStateRulesV6       — produces state.RulesV6 (preserves UI order)
     │
     │   extractTemplateDNSTags(TemplateData)
     │   SyncDNSFullToStateV6(...)            — DNS UI list → flat state.DNS.servers/rules
     │
     │   v6.SyncDNSOptionsWithActivePresets   — ensure kind=preset DNS entries match active preset-refs
     │     (state.RulesV6, &state.DNS, presetMap)
     │   applyPresetEnabledOverrides          — UI toggle for kind=preset → entry.Enabled
     │
     │   build.SyncOutboundsWithActivePresets — TWICE: on both views
     │     ×1: state.Connections.Outbounds
     │     ×2: state.ParserConfig.ParserConfig.Outbounds   ◄── обязательно!
     │
     ▼
state.State.Save(path)
     │   syncConnectionsFromLegacy             — copies ParserConfig.Outbounds → Connections
     │                                          (synced version wins; не затирает updates[])
     │   hasPresetRefs(RulesV6) ? marshalDiskV6 : marshalDisk
     │     marshalDiskV6 = v6 layout (meta.version=6, schema=presets_v1)
     │     marshalDisk   = v5 layout (auto-pick if no kind=preset rules)
     │   maybeBackupV5(path)                   — one-shot state.json.v5.bak on first v5→v6
     │
     │   atomic write: open .tmp, write+fsync, Rename .tmp → path, fsync(dir)
     ▼
disk: bin/wizard_states/state.json
```

**Почему Sync на обе view'а?** `state.Save → syncConnectionsFromLegacy`
копирует `ParserConfig.Outbounds → Connections.Outbounds`. Если sync
наложили только на `Connections` — адаптер затрёт sync'нутые `updates[]`.
Решение: sync обе view'а в `CreateStateFromModel`, тогда адаптер копирует
уже-корректную версию.

Format auto-pick: `hasPresetRefs(RulesV6)` решает v5 vs v6. Юзеры с pure
inline/srs rules остаются на v5 пока не добавят первый preset.

---

## 3. Build flow

`state` + `template` → `bin/config.json` (sing-box-compatible).

```
trigger: app start / config dirty / explicit rebuild
     │
     ▼
core/build entry (BuildAndWriteConfig)
     │
     ├─► ResolveDNS(state, template, vars)        — pure func
     │     walk state.dns_options.servers[] kind switch
     │       template → resolve body из template.dns_options.servers[tag]
     │       preset   → resolve body из template.presets[id].dns_servers[local_tag] + substitute vars
     │       user     → body уже flat в entry
     │     attach metadata: Source / Required / Locked / Active / Enabled
     │
     ├─► ResolveRoute(state, template, vars)      — pure func
     │     walk state.rules[] kind switch
     │       preset → resolve через template.presets[id].rule (expand + tag prefix)
     │       inline → emit body.match + outbound
     │       srs    → emit body.srs_url + outbound (downloaded .srs path)
     │
     ├─► ResolveOutbounds(state, template)        — pure func
     │     walk state.connections.outbounds[]
     │     для каждой: mergeOutboundUpdates(base, Updates[]) → merged body
     │     attach metadata: IsPreset / HasPresetUpdates / Required / PresetLabel
     │
     ├─► (headless paths only) ────────────────────────────────────
     │   SyncOutboundsWithActivePresets(rules, &parserCfg.Outbounds, presets)
     │     ensures parserCfg view синхронизирована (defensive — UI-paths
     │     уже sync'нули в CreateStateFromModel)
     │   MergeOutboundUpdatesInPlace(parserCfg)
     │     материализует Updates[] стек в base body
     │     (generator не знает поле Updates)
     │
     ▼
GenerateOutboundsFromParserConfig
     │     consume merged parserCfg.Outbounds[]
     │     resolve filters / addOutbounds / preferredDefault
     │     append per-source proxies (parsed from .raw cache)
     ▼
MergeDNSSection + MergeRouteSection + MergePresetsIntoRoute
     │     emit final dns / route sections в порядке state.rules[]
     ▼
atomic write: bin/config.json
```

**Resolver pattern** — `ResolveDNS` / `ResolveRoute` / `ResolveOutbounds`
— pure funcs без I/O. UI render и build emit consume один и тот же
resolved view → нет divergence между preview и финальным config.

**Headless vs UI paths.** В UI-сессии `CreateStateFromModel` уже sync'нул
state перед Save, и build читает только. В headless path'ах
(`rebuild_raw_cache`, `UpdateConfigFromSubscriptions`, `parseAndPreview`) —
state читается с диска, sync вызывается defensively, потом
`MergeOutboundUpdatesInPlace` для generator'а.

---

## 4. Preset toggle flow

User clicks checkbox на preset row в Rules tab → eager state mutation +
UI refresh без полного re-render.

```
UI: Rules tab — checkbox toggle на preset row
     │   handler в rules_unified_rows.go (one-liner после рефактора)
     ▼
mutate model:
     state.RulesV6 = update Enabled flag
     PresetRefs[i].Enabled = new value
     │
     ▼
presenter.RefreshAfterPresetToggle()
     │
     ├─► RefreshDNSListAndSelects
     │     v6.SyncDNSOptionsWithActivePresets(rules, &state.DNS, presetMap)
     │     re-render DNS tab list (если открыт)
     │     refresh DNS dropdown'ы (Final / DefaultDomainResolver / per-rule server)
     │
     ├─► build.SyncOutboundsWithActivePresets — на обе view
     │     ×1: model.GlobalOutbounds
     │     ×2: model.ParserConfig.Outbounds (через RefreshDerivedParserConfig)
     │
     ├─► refresh Outbounds tab UI
     │     collectRowsForUI читает state directly (после SPEC 057)
     │     preset rows показываются с 🔒 + preset label
     │     globals с обновлённой filters показывают «⚠ modified by N preset(s)»
     │
     └─► RefreshOutboundOptions
           rebuild per-rule outbound dropdown'ы в Rules tab
           (новые preset-add tag'и появляются; disabled — исчезают)

  ▲
  │
  MarkAsChanged → Save кнопка enable
```

Eager sync (а не lazy на Save) — потому что юзеру нужно сразу видеть
эффект: добавился DNS-сервер в список, появился новый outbound, выпадайки
правил обновились. Без eager sync DNS tab и Outbounds tab показывали бы
устаревшее состояние до Save.

---

## 5. Cross-references

| Аспект | Документ |
|--------|----------|
| Что лежит в state.json, какие kind'ы, schema v6 | [WIZARD_STATE.md](WIZARD_STATE.md) |
| Что лежит в wizard_template.json, presets / vars / required | [TEMPLATE_REFERENCE.md](TEMPLATE_REFERENCE.md) |
| Туториал — как написать новый preset / template var | [CREATE_WIZARD_TEMPLATE.md](CREATE_WIZARD_TEMPLATE.md) |
| Общая архитектура приложения (а не storage) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Release notes v0.9.6 (терминология preset binding) | [release_notes/0-9-6.md](release_notes/0-9-6.md) |

| Source SPEC | Что покрывает |
|-------------|---------------|
| SPECS/052-F-C-CONNECTIONS_REDESIGN | v5 connections layout (sources / outbounds / defaults) |
| SPECS/053-F-N-PRESET_BUNDLES | Preset bundles, `kind` discriminator на rules, RequiredTemplateRef integration |
| SPECS/055-F-S-PRESET_OUTBOUNDS | `preset.outbounds[]` design (add/update modes) |
| SPECS/056-R-N-DNS_SCHEMA_REDESIGN | Flat `dns_options.servers/rules[]` kind discriminator + Resolver pattern |
| SPECS/057-R-N-OUTBOUNDS_PRESET_BINDING | Outbound `Ref` + `Updates[]` schema + lifecycle Sync |
