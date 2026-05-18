# SPEC 056-B-N — OUTBOUNDS_PARSER_RESTORE

**Status:** New (N)
**Type:** Bug (B) — regression / rewrite SPEC 055 implementation
**Discovered:** 2026-05-18 (user feedback после SPEC 055 — sing-box 1.12+ FATAL на rebuild)
**Related:** SPEC 053 (preset bundles, shipped), SPEC 055 (preset.outbounds — feature spec остаётся в силе, переписывается реализация)
**Last known good:**
- **Pipeline-level**: tag `v0.9.5` — там `parser_config → config.outbounds[]` эмит чистый sing-box, без launcher-only полей.
- **Feature-level**: коммит `f665c27` (post-SPEC 053 hot-fixes) — preset bundles работают, outbound pipeline ещё не тронут, **outbound_generator.go идентичен v0.9.5**.

---

## Симптомы

После серии «фиксов» начиная с `bcde8fc` (SPEC 055) серия regression'ов
в `config.outbounds[]` emit pipeline:

  - `outbounds[N].options: unknown field "options"` — parser-internal wrapper
    утекает в финал.
  - `outbounds[N].filters: unknown field "filters"` — launcher-only поле.
  - `outbounds[N].addOutbounds: unknown field "addOutbounds"` — то же.
  - `outbounds[N].comment: unknown field "comment"` — то же.
  - `outbounds[N].wizard: unknown field "wizard"` — launcher metadata.
  - `dns.servers[N].description: unknown field "description"`.

Каждый раз когда добавлялось «strip ещё одного поля» — sing-box падал на
следующем. Корневая причина (см. ниже) не была устранена, латали по
одной ошибке за раз. Контекст деградировал, в коде остался хаос:

  - 4 разных strip-функции с расходящимися наборами полей
    (`stripWizardOnlyFields`, `SanitizeMap`, `SanitizeMapFinal`,
    `finalStripLauncherFields`, `OutboundParserToSingbox`)
  - `applyOutboundUpdate` патчит filters/addOutbounds как dict-merge,
    плюс `resolveAddFiltersIntoOutbounds` дублирует резолв на уровне map
  - `MergePresetsIntoOutbounds` (template-only) +
    `ApplyPresetUpdatesToGeneratedOutbounds` (cache) — каждый со своим
    post-pass'ом и не покрывают все случаи (parser-generated outbounds
    типа `vpn ②` патчатся только через второй путь)
  - `OutboundParserToSingbox` дублирует логику native
    `GenerateSelectorWithFilteredAddOutbounds`

## Цель

**Переписать реализацию SPEC 055** (preset.outbounds) с post-merge
архитектуры на **pre-patch parser_config**, используя existing native
emit pipeline без модификаций. Восстановить инвариант: финальный
`config.outbounds[]` содержит **только** валидные sing-box поля, без
strip-проходов.

Не «добавлять ещё один strip», а **устранить корневую причину**:
preset.outbounds — это parser-format, такой же как
`template.parser_config.outbounds[]`, и обрабатываться должен **через
один и тот же** code-path.

---

## Корневая причина (диагноз)

Native outbound emit pipeline (`core/config/outbound_generator.go`) в
v0.9.5 и в `f665c27` **идентичен** — SPEC 053 (preset bundles) встроился
как **post-merge** для route/dns sections, а case `"outbounds"` в
`buildSection` не трогался. Native 3-pass generator работает с
**parser-format** (`configtypes.OutboundConfig{Tag, Type, Options,
Filters, AddOutbounds, Comment, Wizard}`) и **сам** выполняет:

- `options.*` flatten в top-level (как требует sing-box);
- `filters` → `filterNodesForSelector` (резолв в outbounds list);
- `addOutbounds` → union с filtered nodes (резолв в outbounds list);
- `comment` → `"// %s\n"` префикс (не JSON-поле);
- никаких launcher-only полей в финале.

Предыдущая итерация SPEC 055 применила **тот же post-merge паттерн**
к outbounds: добавляла `Fragments.Outbounds[]` (parser-формат с
`options/filters/addOutbounds/comment/wizard`) поверх **уже сгенерированного**
`outbounds[]` (sing-box-формат). Из-за этого launcher-only поля стали
утекать в финал → каскад «strip ещё одного поля» (`stripWizardOnlyFields`,
`SanitizeMap`, `SanitizeMapFinal`, `finalStripLauncherFields`,
`OutboundParserToSingbox`) — лечение симптомов, а не корня.

Параллельно `eb382cc feat(spec-055): mode=update now patches parser-
generated outbounds` сделал шаг в правильную сторону, но патч применялся
к JSON-строкам в `ctx.Cache` **после** парсинга, что снова требует
дублирующего резолва `filters` / `addOutbounds` (см. `15b217c`,
`resolveAddFiltersIntoOutbounds`, `applyOutboundUpdate(filters)`).

---

## Финальная архитектура (pre-patch parser_config до парсинга)

**Принцип: одна точка истины — native pipeline.** Преобразование
`template.Preset.Outbounds[i]` в `configtypes.OutboundConfig` происходит
**ДО** вызова `GenerateOutboundsFromParserConfig`, на типизированной
структуре. Native pipeline дальше сам всё разрулит.

```
template.Preset.Outbounds[i]   (launcher type)
        │
        │  ExpandPresetOutbounds(preset, vars):
        │    • @var substitution в options/filters/addOutbounds
        │    • filter by if/if_or
        │    • convert to configtypes.OutboundConfig
        ▼
[]configtypes.OutboundConfig   (parser-format, типизированный)
        │
        │  ApplyPresetOutboundsToParserConfig(parserCfg, presetRefs, ruleOrder):
        │    • deep-clone parserCfg.ParserConfig.Outbounds[]
        │    • для каждого active preset-ref (по RuleOrder):
        │        for each expanded outbound:
        │           mode="add"    → append to clone
        │             • tag-collision (с globals или earlier preset)
        │               → first wins + warning
        │             • identical body → silent skip
        │           mode="update" → in-place patch entry в clone
        │             • Filters     → replace
        │             • AddOutbounds → union
        │             • Options.*   → per-field replace (только заданные)
        │             • Wizard      → replace
        │             • Type        → drop + warning (immutable)
        │             • Tag         → drop (immutable)
        │             • Comment     → replace
        │             • PreferredDefault → replace
        │           target отсутствует → skip + warning, no auto-create
        │    • return patched ParserConfig (immutable original)
        ▼
patched ParserConfig
        │
        ▼
GenerateOutboundsFromParserConfig (НАТИВНЫЙ, unchanged from v0.9.5)
        │  • options.* flatten
        │  • filters → filterNodesForSelector (резолв против snapshot.Proxies)
        │  • addOutbounds → resolved against full tag list
        │  • comment → "// %s\n" префикс
        │  • никаких launcher-полей в JSON
        ▼
config.outbounds[]   (clean sing-box JSON, passes `sing-box check`)
        │
        ▼
[route.rules post-pass]
        │
        │  cleanDanglingOutboundRefInRule(rule, finalTags, fallback):
        │    • если rule.outbound ∉ finalTags → fallback на route.final
        │    • если final пуст → drop rule
        │    • sentinel reject/drop preserved
        │  При ctx.ForPreview=true — пропускается (наследие 0c3dce5).
        ▼
config.route.rules[]   (финальный)
```

**Преимущества подхода:**

- mode=update **тривиально** работает с parser-generated outbounds
  (`proxy-out`, `auto-proxy-out`, `vpn ②`, `AL:auto`) — потому что мы
  патчим parser_config до парсинга.
- `addOutbounds` union между preset и template считается **до** native
  pipeline → нативный `filterNodesForSelector` видит правильный merged
  set. Никаких отдельных `resolveAddFiltersIntoOutbounds` /
  `collectAllNodeTagsFromCache`.
- Нет ни одной strip/sanitize/transform функции для outbound JSON
  (acceptance #3 выполняется автоматически — таких функций **ноль**).
- Preview path и save path используют одну функцию → нет
  «двух источников истины» для available outbound tags.
- emoji-теги (`ru VPN 🇷🇺`) идут через тот же путь что и template
  static — без edge-кейсов.

**Не-цели / явно отрезаем:**

- НЕ создаём отдельную функцию emit'а `preset.outbounds → sing-box JSON`.
- НЕ работаем с `map[string]interface{}` для outbound полей (всё
  типизированно через `OutboundConfig`).
- НЕ делаем strip-проходы по финальному JSON.
- НЕ трогаем native `outbound_generator.go` — он остаётся как был
  в v0.9.5 / `f665c27`.
- НЕ меняем DNS emit pipeline (это вне scope 056).

---

## Acceptance

1. `sing-box check -c config.json` PASSES после `Rebuild Config` с
   реальным user state'ом (Liberty VPN + russian preset + custom rules).
2. На любой ошибке `Rebuild` показывает popup с конкретным sing-box
   error message (наследие `5e56c0b` + sing-box check из `15b217c`).
3. В кодовой базе **ноль** функций для трансформации preset.outbounds
   entry в sing-box format — вся работа делается native pipeline'ом.
4. Все 24 пакета тестов зелёные.
5. `ru VPN 🇷🇺` selector реально содержит RU-tagged subscription nodes
   в `outbounds` array после rebuild.
6. mode=update патч `proxy-out` с `filters: { tag: "!/RU/i" }` от
   `ru-inside` / `russian` preset'а действительно фильтрует RU-ноды
   из глобального селектора (видно в `config.outbounds[]::proxy-out::outbounds`).
7. Disable preset → effect полностью исчезает (parser_config copy
   мутировал, original не тронут).

## Что НЕ делать в этой таске

- НЕ добавлять «ещё один strip» если что-то выпадет в sing-box check —
  сначала понять, **почему** unknown field попал в финал (через какой
  путь), и устранить корневую причину. В новой архитектуре launcher-полей
  в финале быть не может.
- НЕ дублировать логику emit'а (native parser vs кастомный preset emit) —
  вся transform происходит на типизированном `OutboundConfig` до native
  pipeline.
- НЕ ломать DNS server emit — он работал в `f665c27` после фикса
  `b03fd5b` (drop bare-tag override markers). DNS pipeline вне scope.
- НЕ трогать параллельные правки P1–P10 (SPEC 054, force-rebuild,
  preview re-render, rules-tab loops, DNS bare-tag, source-overview UX,
  textnorm emoji fix).

---

## Параллельные правки в диапазоне `bcde8fc..HEAD` (НЕ трогать)

| # | Коммит(ы) | Суть | Действие |
|---|---|---|---|
| P1 | `1019144` | SPEC 054 — Xray JSON bodies no longer bloat state.json | Сохранить |
| P2 | `5e56c0b` | UI Rebuild button forces rebuild + sing-box check | Сохранить |
| P3 | `842df2c` | Preview always re-renders (убраны stale-state checks) | Сохранить |
| P4 | `d36a257` | Wizard Preview tab теперь применяет preset-refs к config.json | Сохранить |
| P5 | `dc4cf09` | Infinite `MarkAsChanged` loop на preset-ref render | Сохранить |
| P6 | `0ecc403` | Inline outbound select triggered on initial render | Сохранить |
| P7 | `b03fd5b` | DNS bare-tag override markers — drop перед эмитом | Сохранить |
| P8 | `0c3dce5` | Skip dangling outbound cleanup в preview mode | Сохранить (после pre-patch станет dead-branch'ем, но условие остаётся) |
| P9 | `1019144`, `93fbf00`, `6a04221`, `02c9ee1`, `ab2ab57`, `8436555`, `e367b79`, `03d2931`, `ff20fcc`, `23eeb26`, `738ba5e`, `e7120e8` | source-overview/raw-body UX набор | Сохранить |
| P10 | `9fe0e77` | Replace Fyne-unrenderable colored shape emojis with `*` | Сохранить |

**Особый случай `15b217c`** (mixed commit):
- Сохранить: `validateConfigViaSingBox`, `stripANSI`, Step 5.4 в
  `RebuildConfigIfDirty` (sing-box check + popup). P2 (`5e56c0b`)
  опирается на это.
- Снести: `applyOutboundUpdate(filters/addOutbounds)`,
  `resolveAddFiltersIntoOutbounds`, `PresetMergeContext.AllNodeTags`,
  `collectAllNodeTagsFromCache(Local)` — латание 055 архитектурного
  изъяна, в pre-patch архитектуре не нужно.

**Особый случай `eb382cc`**: попытка patch parser-generated outbounds
**после** парсинга (на cache strings). Архитектурное направление верно,
но реализация неверная — заменяется на pre-patch parser_config **до**
парсинга.

---

## Risk / rollback

- Если pre-patch упирается в timing constraints (parser_config
  загружается до того как доступны preset-refs) — fallback: создать
  обёртку которая берёт `*template.TemplateData + state.RulesV6` и
  возвращает patched ParserConfig перед вызовом `BuildConfig`. Это
  тривиально, т.к. оба объекта уже доступны в `AppController` /
  `BuildPreviewConfig`.
- Если переписывание затянется — fallback позиция: «отключить SPEC 055
  фичу» (preset.outbounds игнорится loader'ом → пресеты ru-inside /
  russian / ru-blocked временно без !RU фильтра). Это можно сделать
  одним if-guard'ом в loader. v0.9.5 как rollback target использовать
  нельзя — потерям SPEC 053 (preset bundles) + SPEC 054.
