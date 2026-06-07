# Template reference (wizard_template.json)

Архитектурная сводка: что лежит в `bin/wizard_template.json`, где это потом
всплывает в runtime / state / UI. Туториал для авторов template'ов —
[CREATE_WIZARD_TEMPLATE.md](CREATE_WIZARD_TEMPLATE.md). Этот файл — reference
для разработчиков лаунчера и для понимания связи template ↔ state.

---

## 1. Файл

- **`bin/wizard_template.json`** — единственный template для всех ОС.
  Платформозависимые куски через секцию `params` + `if`/`if_or` поверх vars.
- **Pinned ref:** `internal/constants.RequiredTemplateRef` хранит SHA коммита
  в репозитории, под который собран launcher. CI ldflags инжектит реальный
  hash на релизе; в dev-сборке используется source-default (последний known-good
  merge commit).
- **Lifecycle на upgrade:** `core/template_migration.go::InvalidateTemplateIfStale`
  сравнивает `Settings.LastTemplateLauncherVersion` (записан после последнего
  успешного «Download Template» через `MarkTemplateInstalled`) с
  `constants.AppVersion`. При несовпадении удаляет `bin/wizard_template.json`
  — на следующем запуске UI показывает «Download Template», после успешной
  скачки `bin/settings.json` получает новый `last_template_launcher_version`.
  Dev-сборки (`v-local-test`, `unnamed-dev`, `*-dirty`) пропускают
  invalidation — иначе локальная разработка ломалась бы на каждом запуске.
  См. SPEC 046 (механизм) и SPEC 067 (breaking template format — `#if` +
  `@`-only outer `if[]` — триггерится тем же bump `AppVersion`).

---

## 2. Top-level shape

```jsonc
{
  "parser_config":   { ... },        // ParserConfig wrapper для subscription parser
  "config":          { ... },        // sing-box config skeleton (log/dns/inbounds/outbounds/route/experimental)
  "params":          [ ... ],        // platform-conditional patches на config (replace/prepend/append)
  "dns_options":     {               // dns tab library
    "servers": [ ... ],              // template DNS server entries (+ required:true для local/direct resolver)
    "rules":   [ ... ]
  },
  "selectable_rules":[ ... ],        // legacy rules library — kept for back-compat, replaced by presets[]
  "presets":         [ ... ],        // SPEC 053 self-contained preset bundles
  "vars":            [ ... ]         // typed template variables (UI Settings tab + @var substitution)
}
```

---

## 3. Per-section storage / usage

| Top-level key | Содержит | Куда попадает в runtime | UI tab где видно |
|---------------|----------|--------------------------|------------------|
| `parser_config` | Default ParserConfig skeleton: outbounds (`proxy-out`, `direct-out`, `auto-proxy-out`) с top-level `required: true` маркерами (см. §5). После SPEC 058 — **live source-of-truth** для body referenced template outbound'ов (state хранит только thin `{tag, ref: "#TEMPLATE#"}`). | На fresh install — в `model.ParserConfigJSON`. При LoadState body для `ref="#TEMPLATE#"` entries резолвится отсюда на каждый render/build. | Outbounds tab (renders model.GlobalOutbounds) |
| `config` | Sing-box config skeleton: `log`, `dns`, `inbounds`, `outbounds`, `route`, `experimental`. Содержит `@var` плейсхолдеры. | После `applyParams(GOOS) + substitute @vars` → `TemplateData.Config` (по секциям). При build merge'ится с state-derived sections. | Никакая напрямую; преview через Edit dialog | 
| `params` | Platform-conditional patches (`if`/`if_or` + `replace`/`prepend`/`append`) | Применяются в `LoadTemplateData` (GetEffectiveConfig) — продьюсят `Config` под текущий runtime.GOOS | — |
| `dns_options.servers` | Library template DNS servers (cloudflare, google, yandex, ...) + mandatory `required:true` entries (`local_dns_resolver`, `direct_dns_resolver`) | `TemplateData.DNSOptionsRaw` → используется `ResolveDNS` для резолва body kind=template entries в state | DNS tab (renders kind=template entries) |
| `dns_options.rules` | Default template DNS rules (опционально) | Auxiliary fill для DNS rules editor если state пустой | DNS tab |
| `selectable_rules` | **Legacy.** Library из v3+ времён. Полностью заменён `presets[]`. | Сохранено для back-compat: загружается в `TemplateData.SelectableRules`, фильтруется по platforms | Не показывается (Library показывает только `presets[]`) |
| `presets` | Self-contained preset bundles: vars / rule_set / dns_servers / dns_rule / rule / outbounds (SPEC 053 + 055 + 057) | `TemplateData.Presets`. На enable preset → создаются `kind=preset` entries в `state.rules`, `state.dns_options.servers/rules`, `state.connections.outbounds` через Sync* функции | Library dialog (add to Rules) → Rules tab (preset rows) |
| `vars` | Объявления типизированных template-переменных (name, type, default, options, if/if_or, ui_meta) | `TemplateData.Vars`. Дефолты применяются если в `state.vars[]` нет override. Литералы `@var` в `config`/`params` подставляются на build. | Settings tab (auto-rendered) + DNS scalars (`dns_*` hidden vars) |

«UI tab где видно» — где юзер взаимодействует с этой секцией. Output в
`config.json` — это всегда build pipeline; ни одна секция template не идёт
в config.json напрямую без прохождения через `state + resolve*`.

---

## 4. Presets (SPEC 053 + SPEC 055 + SPEC 057-R-N)

Preset — параметризованный self-contained bundle. Каждый компонент имеет
свой ref-механизм в state.

### 4.1 `presets[].outbounds[]` — SPEC 055 + SPEC 057-R-N

Entries с `mode` discriminator:

| `mode` | Эффект на state | Эффект на config.json |
|--------|------------------|------------------------|
| `add` (или omit) | На enable preset → entry в `state.connections.outbounds[]` с `ref = preset.id`. Body резолвится из entry. | Эмитится как обычный outbound через `GenerateOutboundsFromParserConfig`. |
| `update` | На enable preset → `OutboundUpdate{ref, patch}` push в `state.connections.outbounds[<target_tag>].updates[]`. Target tag должен существовать в state (find by Tag; не найден → warning, no-op). | `MergeOutboundUpdatesInPlace` применяет patches до generator'а (base + apply patches в order). |

Lifecycle: `core/build/sync_outbounds.go::SyncOutboundsWithActivePresets`.
Adopt-on-first-sync: pre-SPEC-057 globals без `Ref`, совпадающие по tag с
expected preset add — adopt'ятся (preserve body, add `Ref`).

### 4.2 `presets[].dns_servers[]` — SPEC 053 + SPEC 056-R-N

Bundled DNS server defs с локальными tag'ами. На enable preset →
`SyncDNSOptionsWithActivePresets` создаёт entries в `state.dns_options.servers[]`
с `kind=preset, ref="<preset_id>:<local_tag>"`. Body резолвится из template
+ `@var` substitute из `preset.body.vars` каждый раз на build/render.

Юзер может toggle per-server (preserve'ится в state.entries.Enabled).
На disable preset → entries удаляются (re-enable → свежие дефолты).

### 4.3 `presets[].dns_rule` — SPEC 053

Опциональный объект (один на preset). На enable preset → entry в
`state.dns_options.rules[]` с `kind=preset, ref="<preset_id>"`. Body
резолвится из template + vars + tag-prefix (`@dns_server` var может
ссылаться на bundled `dns_servers[].tag`).

### 4.4 `presets[].rule` — SPEC 053

Routing rule preset'а. На enable → entry в `state.rules[]` с
`kind=preset, ref="<preset_id>"`. На build `MergePresetsIntoRoute`
expand'ит ref в `template.presets[id].rule`: substitute vars, prefix
local rule_set tag'и, resolve sentinels (`reject` / `drop` → `action`),
эмитит в `route.rules[]` в том же порядке, как entry в `state.rules[]`.

### 4.5 `presets[].vars[]` — SPEC 053 + SPEC 048

Типизированные локальные переменные preset'а.

| `type` | UI control | Substitution value |
|--------|------------|--------------------|
| `outbound` | Dropdown: outbound tags + `reject` + `drop` (опц. whitelist через `options`) | Tag-строка |
| `dns_server` | Grouped dropdown (3 секции) или whitelist (`options`/`select`) | Tag-строка (build prefix'ует bundled tag'и при substitute) |
| `enum` | Dropdown по `options[]` (object `{title, value}`) | `value`-строка |
| `text` | Text entry | Строка |
| `number` | Numeric entry | Строка-число |
| `bool` | Checkbox | `"true"` / `"false"` |

Substitute механизм: build-time recursive walk по `rule` / `dns_rule` /
`dns_servers` / `rule_set` фрагментам — каждая строка `"@name"`
заменяется на `varsMap[name]`. Если var отфильтрована через `if`/`if_or`
— substitute её литерала фейлится → preset skip + warning.

State хранит **только diff** от template defaults в `rule.body.vars`
(пустой `vars: {}` = preset на template defaults).

---

## 5. Template-owned vs user-editable

Маркер `"required": true` (SPEC 056-R-N Phase C/E) — template-only флаг,
state не персистит. Применим к:

| Где | Эффект в UI |
|-----|-------------|
| `parser_config.outbounds[].required` | Outbounds tab: Up/Down + Edit + Reset; Del не рендерится |
| `dns_options.servers[].required` | DNS tab: enabled+lock (toggle blocked); Edit/Del заблокированы |

**Shape (после SPEC 058):** `required` — top-level поле прямо на outbound
entry, не вложенное в обёртку:

```jsonc
"parser_config": {
  "outbounds": [
    { "tag": "auto-proxy-out", "type": "urltest", "required": true,
      "options": { "url": "@urltest_url", "interval": "@urltest_interval" } },
    { "tag": "proxy-out", "type": "selector", "required": true,
      "options": { "outbounds": ["auto-proxy-out", "direct-out"] } },
    { "tag": "direct-out", "type": "direct" }
  ]
}
```

**DEPRECATED:** старая форма `{ "wizard": { "required": 1 } }` всё ещё
парсится через legacy fallback в `td.RequiredOutboundTags()` —
исключительно для обратной совместимости со старыми template-форками.
Новые template'ы должны использовать top-level `required: true`.

Read live на каждый UI render через helpers:
- `wizardbusiness.DNSTagLocked(model, tag)` — для DNS
- `templateRequiredTags(model)` → используется `ResolveOutbounds` для outbound

Если template author снимает `required:true` в новой версии template'а —
эффект мгновенный (state не помнит stale значение).

---

## 6. Data flow

```
bin/wizard_template.json (pinned via RequiredTemplateRef)
         │
         ▼
LoadTemplateData(execDir)
         │   read JSON
         │   ApplyParams(runtime.GOOS) → effective Config
         │   Substitute @vars в Config (через TemplateData.Vars defaults)
         │   ParsePresets → []Preset (фильтр по platforms)
         │   ParseSelectableRules → []SelectableRule (legacy, фильтр platforms)
         ▼
model.TemplateData (in-memory, immutable)
         │
         ├──► UI render (Library dialog, DNS tab, Settings tab, Outbounds tab)
         │
         ├──► build pipeline:
         │     ResolveDNS(state, template, vars)
         │     ResolveRoute(state, template, vars)
         │     ResolveOutbounds(state, template)
         │
         └──► presenter Sync* (на каждый preset toggle):
                SyncDNSOptionsWithActivePresets(rules, &state.DNS, presets)
                SyncOutboundsWithActivePresets(rules, &state.outbounds, presets)
```

TemplateData immutable после load; модификация template requires app restart.

**SPEC 058 — template body как live source.** Outbound entries в
`state.connections.outbounds[]` хранятся как **thin refs**
(`{tag, ref: "#TEMPLATE#", updates: [...]}`) — body отсутствует. На
каждый render/build body резолвится из `template.parser_config.outbounds[tag]`
через `ResolveOutbounds`. Template-author эффект: правка
`parser_config.outbounds[].options` / `addOutbounds` / `comment` в новом
билде доезжает до юзера автоматически (без manual Reset на каждой
референсной entry). User edits хранятся как field-level diff в
`updates[].patch` с `ref="#USER#"`. См. SPEC 058 + [DATA_FLOW.md](DATA_FLOW.md)
для подробностей resolver pipeline'а.

---

## 7. `vars` mechanism

**Объявление** (template):
```jsonc
"vars": [
  { "name": "tun", "type": "bool", "default": "true",
    "ui_meta": { "tab": "Settings", "title": "Enable TUN" },
    "platforms": ["windows", "darwin"] }
]
```

**Override** (state.json):
```jsonc
"vars": [
  { "name": "tun", "value": "false" }
]
```

**Substitute** (build): литералы `"@tun"` в `config` / `params` / preset
фрагментах заменяются на эффективное значение (state override ИЛИ template
default). Условия `if`/`if_or` на params/presets/vars проверяются по тому
же varsMap.

**Scope**:
- Глобальные template vars (`template.vars[]`) — видны в top-level `config`
  / `params` (НЕ видны внутри preset'а).
- Preset-local vars (`preset.vars[]`) — видны только внутри своего preset'а
  (rule/dns_rule/dns_servers/rule_set). Cross-scope доступ запрещён —
  preset должен быть self-contained.

**Special: DNS scalars.** `dns_strategy`, `dns_final`, `dns_default_domain_resolver`
объявлены как hidden vars `dns_*`. UI DNS tab пишет в `model.SettingsVars`,
`SyncDNSModelToSettingsVars` копирует в `state.vars[]` перед Save. Build
substitute'ит `@dns_*` литералы в `config.dns`.

**Special: `route_final`.** UI Rules tab dropdown «Final outbound» →
`model.SelectedFinalOutbound` → `SettingsVars["route_final"]` →
`state.vars[]`. Template имеет `"final": "@route_final"` в `config.route`.

Реализация: `core/template/vars_resolve.go` + `core/template/substitute.go`.

---

## 8. Pinned templates

Template вшит в репозиторий → embedded в бинарь → распакован в `bin/` на
first run. Каждая релизная сборка пиннит конкретный commit:

| Source | Когда используется | Where |
|--------|---------------------|-------|
| **CI inject** | Release сборка: GitHub Actions подставляет SHA merge commit'а через `-ldflags '-X singbox-launcher/internal/constants.RequiredTemplateRef=<sha>'` | `.github/workflows/release.yml` |
| **Source default** | Dev-сборка (`go build` без ldflags) | `internal/constants/constants.go::RequiredTemplateRef` константа |

**Bump процесс** (на каждом релизе):
1. Merge `develop` → `main` (создаётся merge commit с обновлённым `bin/wizard_template.json`)
2. Tag merge commit (`vX.Y.Z`)
3. На `develop` обновить source-default `RequiredTemplateRef` на SHA merge commit'а

**Lifecycle на launch**:
```
launcher start
     │
     ▼
InvalidateTemplateIfStale(execDir)
     │   compare Settings.LastTemplateLauncherVersion vs constants.AppVersion
     │   stale (LastTemplateLauncherVersion < AppVersion) → unlink bin/wizard_template.json
     │   (dev AppVersion skip: v-local-test / unnamed-dev / *-dirty)
     ▼
UI shows «Download Template» (если файл отсутствует)
     │   юзер кликает → скачивается с raw.githubusercontent.com под pinned ref
     │   MarkTemplateInstalled → bin/settings.json::last_template_launcher_version = AppVersion
     ▼
LoadTemplateData
```

Реализация: `core/template_migration.go::InvalidateTemplateIfStale` +
`internal/locale/settings.go::LastTemplateLauncherVersion` /
`MarkTemplateInstalled` + `core/template/loader.go::LoadTemplateData`.

Breaking template format changes (например SPEC 067 — `#if` + `@`-only outer
`if[]`) триггерятся этим же механизмом: после bump `AppVersion` на первом
запуске старый кеш удаляется → юзер скачивает новый шаблон одним кликом.

---

## 9. `#if` construct (SPEC 067) — desktop only

Template expressions v1 — declarative conditional field inclusion прямо в
шаблоне, без post-substitute Go-хуков. Реализован в
`core/template/substitute.go::SubstituteVarsInJSON` (walker) и
`core/template/template_validate.go::validateIfConstruct` (load-time
validation). Покрывает кейсы вида «одно поле внутри уже эмиченного объекта
зависит от bool var / runtime platform».

> **Mobile parity:** все `#*` constructs (`#if`, потенциальные
> `#for_each` / `#include`) — **desktop only** до подтяжки реализации в
> LxBox. Шаблоны, шарящиеся между лаунчерами, должны helmet'ить
> платформы которых поддержка ещё нет.

### 9.1 Naming discipline — `#` vs bare vs `@`

| Префикс | Где | Зачем маркер |
|---------|-----|--------------|
| `#` | Construct gateway (`#if`) + predicates в `and`/`or` (`#in`, `#not`, `#notEmpty`, …) | Scope-switch: walker отличает control-key от data-key в произвольном объекте; predicate-имя от string literal в predicate list |
| bare | Inner keys тела `#if` (`and`, `or`, `value`, `else`) + outer legacy keys (`params[].if`, `params[].if_or`, `params[].value`, `params[].mode`) | Walker уже в known scope, маркер избыточен |
| `@` | Var-ref (только имя из `vars[]`; bare `"var"` → loader error) + runtime globals `@platform` / `@arch` (только в `#if` predicates) | Унифицированная нотация var-ref'ов везде; неоднозначность «literal vs var name» устранена |

Forward compatibility: неизвестный ключ начинающийся с `#` → walker
логирует warn и удаляет (graceful degradation). Это позволяет добавлять
новые constructs (`#for_each`, `#include`, …) без breaking change для
старых лаунчеров.

### 9.2 Форма

```jsonc
"#if": {
  "and":   [<predicate>, <predicate>, ...],  // mutually exclusive с `or`
  "or":    [<predicate>, <predicate>, ...],  // mutually exclusive с `and`
  "value": <any JSON>,                        // обязателен, then-ветка
  "else":  <any JSON>                         // опциональный else-ветка
}
```

Правила (validation на load):
* Ровно один из `and` / `or` непустым списком. Нет / оба / пустой list → loader error.
* `value` обязателен (не nil).
* `else` опционален; null в `value`/`else` → error в map-spread (нельзя merge), legal в array-element.

### 9.3 Два режима размещения

**Map-spread mode** — `#if` как ключ внутри объекта:

```jsonc
{
  "type": "mixed",
  "tag": "proxy-in",
  "#if": {
    "and": ["@proxy_in_auth_enabled"],
    "value": {"users": [{"username": "@proxy_in_username", "password": "@proxy_in_password"}]}
  }
}
```

* condition true → `value` обязан быть объектом; его поля мерджатся в
  родительский объект (collision → branch overrides). Ключ `#if` удаляется.
* condition false: при наличии `else` мерджатся его поля; без `else`
  ключ просто удаляется (parent unchanged).

**Array-element mode** — `#if` как единственный ключ объекта-элемента
массива:

```jsonc
"options": [
  "always",
  {"#if": {"and": ["@dark_mode"], "value": "extra-dark", "else": "extra-light"}},
  "regular"
]
```

* condition true → элемент заменяется на `value` (любой тип).
* condition false: при наличии `else` — заменяется на `else`; без `else`
  — элемент **удаляется** из массива (длина -1).

Detection rule: элемент — `#if` wrapper, если это объект из РОВНО одного
ключа `#if`. Иначе обычный элемент (с возможным spread-mode `#if` внутри).

### 9.4 Expression language — predicates

Каждый элемент `and` / `or` — predicate. Восемь форм:

| Форма | Семантика |
|---|---|
| `"@var"` | bool template var → `scalar == "true"` (только bool var; **не** `@platform` / `@arch`) |
| `{"@var": "literal"}` | equality: `trim(scalar) == "literal"` (literal **не** начинается с `#`) |
| `{"@var": "#notEmpty"}` | text → `len(trim(scalar)) > 0`; text_list → `len(list) > 0`; bool → `scalar == "true"` |
| `{"@var": "#isEmpty"}` | инверсия `#notEmpty` |
| `{"@var": {"#in":      ["a","b","c"]}}` | `trim(scalar)` присутствует в списке (`["..."]` или `@text_list_var`) |
| `{"@var": {"#notIn":   ["a","b","c"]}}` | `trim(scalar)` отсутствует в списке |
| `{"@var": {"#matches": "^[a-z]+$"}}` | `trim(scalar)` match'ит Go-regexp |
| `{"#not": <predicate>}` | унарная негация (recursive inner predicate) |

Substitution внутри predicate args: literal в equality, элементы `#in` /
`#notIn`, regex pattern в `#matches` могут содержать `@var` — walker
substitute'ит их **до** оценки predicate. ИСКЛЮЧЕНИЕ: bare `"@var"` в
predicate list и ключ `"@var"` в single-key object'е walker не
substitute'ит (иначе var-reference потеряется).

Пример:

```jsonc
"and": [
  "@flag_a",                                       // bool true
  {"#not": "@flag_b"},                             // bool false
  {"@platform": {"#in": ["darwin", "linux"]}},     // runtime GOOS
  {"@arch": "amd64"},                              // runtime GOARCH
  {"@protocol": {"#in": ["vless", "trojan"]}},
  {"#not": {"@hostname": {"#matches": "^test-"}}}
]
```

### 9.5 Runtime globals — `@platform` / `@arch`

Зарезервированные pseudo-var'ы, доступные **только** в `#if.and` /
`#if.or` predicates:

| Global | Runtime source | Значения |
|---|---|---|
| `@platform` | `runtime.GOOS` | `"darwin"`, `"windows"`, `"linux"` |
| `@arch` | `runtime.GOARCH` | `"amd64"`, `"arm64"`, `"386"` |

Семантика — те же predicate-формы, что у text-var (equality, `#in`,
`#notIn`, `#matches`, `#notEmpty` / `#isEmpty`). Bare `"@platform"` /
`"@arch"` в predicate list (bool-form) → validation error: они не bool.

Case-sensitive lower-case (как `runtime.GOOS` / `runtime.GOARCH`).
**Reserved:** `vars[].name == "platform"` или `"arch"` → loader error
(collision с globals). **Outer `if` / `if_or`** runtime globals
**не принимают** — там только bool template vars; platform-gate на уровне
param по-прежнему через `params[].platforms[]`.

Win7-сборка (`windows/386`): `{"@platform": "windows"}` + `{"@arch": "386"}`
в одном `and` — эквивалент «только win7-bin».

### 9.6 Outer `if` / `if_or` — канонический `@`-only

`params[].if` / `params[].if_or`, `vars[].if` / `vars[].if_or`,
`presets[].if` / `presets[].if_or` принимают **только** `@`-prefixed
var-ref'ы. Bare `"tun"` → loader error на template load:

```
template: params[N].if has bare var-ref "tun" in if[]; use canonical "@tun" form
```

Var должна существовать в `vars[]` и иметь `type: "bool"`. Runtime globals
(`@platform` / `@arch`) в outer `if[]` **запрещены** — только в `#if`
predicates.

### 9.7 Реальный пример — TUN inbound без дублирования

Было (две `params[].name="inbounds"` entries, различающиеся **только**
наличием `interface_name`):

```jsonc
{ "name": "inbounds", "platforms": ["windows", "linux"], "if": ["@tun"],
  "value": [{ "type": "tun", "tag": "tun-in", "interface_name": "singbox-tun0",
              "address": ["@tun_address"], "mtu": "@tun_mtu",
              "auto_route": true, "strict_route": "@strict_route",
              "stack": "@tun_stack" }] },
{ "name": "inbounds", "platforms": ["darwin"], "if": ["@tun"],
  "value": [{ "type": "tun", "tag": "tun-in",
              "address": ["@tun_address"], "mtu": "@tun_mtu",
              "auto_route": true, "strict_route": "@strict_route",
              "stack": "@tun_stack" }] }
```

Стало (одна entry, platform-conditional поле инкапсулировано в map-spread
`#if`):

```jsonc
{
  "name": "inbounds",
  "if": ["@tun"],
  "value": [{
    "type": "tun", "tag": "tun-in",
    "address": ["@tun_address"], "mtu": "@tun_mtu",
    "auto_route": true, "strict_route": "@strict_route",
    "stack": "@tun_stack",
    "#if": {
      "and": [{"@platform": {"#in": ["windows", "linux"]}}],
      "value": {"interface_name": "singbox-tun0"}
    }
  }]
}
```

Подробности и edge cases — `SPECS/067-F-N-TEMPLATE_EXPRESSIONS/SPEC.md`.

---

## 10. Где лежит реализация

| Файл | Что |
|------|-----|
| `core/template/loader.go` | `LoadTemplateData` (entry point) + `TemplateData` struct |
| `core/template/preset_loader.go` | `LoadPresets` + validation |
| `core/template/preset_types.go` | Preset / PresetVar / PresetRuleSet / PresetDNSServer / PresetOutbound types |
| `core/template/preset_lite.go` | `PresetLite` interface + `PresetLiteMap` (для sync_dns без cyclic deps) |
| `core/template/vars_resolve.go` | varsMap build + outer `if`/`if_or` eval (strict `@`-prefix, SPEC 067) |
| `core/template/substitute.go` | recursive `@var` substitution + `#if` walker / predicate engine / runtime globals `@platform`/`@arch` (SPEC 067) |
| `core/template/template_validate.go` | template-side validation (uniqueness, refs resolvable, `#if` construct + outer `@`-only refs — SPEC 067) |
| `internal/constants/constants.go` | `RequiredTemplateRef` + `WizardTemplateFileName` |
| `core/template_migration.go` | `InvalidateTemplateIfStale` (stale template invalidation) |
| `core/build/preset_expand.go` | preset expand at build time (substitute + tag prefix + filter) |

См. также: [WIZARD_STATE.md](WIZARD_STATE.md) — как state взаимодействует
с template, формат `state.json` v6, lifecycle Sync*. [DATA_FLOW.md](DATA_FLOW.md)
— расширенные load/save/build/toggle диаграммы. [CREATE_WIZARD_TEMPLATE.md](CREATE_WIZARD_TEMPLATE.md)
— туториал для авторов preset'ов и template-vars.
