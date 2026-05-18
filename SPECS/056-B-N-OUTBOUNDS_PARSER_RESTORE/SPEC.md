# SPEC 056-B-N — OUTBOUNDS_PARSER_RESTORE

**Status:** New (N)
**Type:** Bug (B) — regression
**Discovered:** 2026-05-18 (user feedback после SPEC 055 — sing-box 1.12+ FATAL на rebuild)
**Related:** SPEC 053 (preset bundles), SPEC 055 (preset.outbounds)
**Last known good:** tag `v0.9.5` — там parser_config → config.outbounds[] эмит чистый sing-box, без launcher-only полей

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

Каждый раз когда я добавлял «strip ещё одного поля» — sing-box падал на
следующем. Я не разобрался в схеме раздела parser vs sing-box и латал
по одной ошибке за раз. Контекст деградировал, в коде остался хаос:

  - 4 разных strip-функции с расходящимися наборами полей
  - `applyOutboundUpdate` патчит filters/addOutbounds как dict-merge вместо
    резолва в outbounds list
  - `MergePresetsIntoOutbounds` / `ApplyPresetUpdatesToGeneratedOutbounds`
    каждый со своим post-pass'ом
  - `OutboundParserToSingbox` (последняя итерация) делает flatten options +
    strip — но дублирует логику с native `GenerateSelectorWithFilteredAddOutbounds`

## Цель

**Восстановить работоспособный outbounds emit pipeline** так как было в
`v0.9.5`, при этом сохранив необходимый минимум для SPEC 053/055 функционала
(preset.outbounds mode=add/update).

Не «добавлять ещё один strip», а **переписать с нуля** preset.outbounds
emit-путь по образу native parser_config выйти.

---

## План

### 1. Изучение (read-only)

- [ ] Прочитать sing-box outbound spec: https://sing-box.sagernet.org/configuration/outbound/
  - Какие поля допустимы на selector / urltest / direct / shadowsocks etc.
  - Какие nested поля (tls.*, options.*) валидны.
- [ ] Прочитать `core/config/outbound_generator.go::GenerateSelectorWithFilteredAddOutbounds`
  - Какие поля native parser выгружает в финал.
  - Как обрабатывает options-flattening, filters, addOutbounds.
  - Как обрабатывает comment (видно `// %s\n` префиксом, не в JSON).
- [ ] Прочитать `core/config/configtypes/types.go::OutboundConfig`
  - Какие поля parser-internal, какие sing-box.

### 2. Откат хаоса

- [ ] Откатить `OutboundParserToSingbox` + `SanitizeMap`/`SanitizeMapFinal`
  как «единая точка истины» — они стали свалкой.
- [ ] Удалить `finalStripLauncherFields`, `stripWizardOnlyFields` — это
  следы латаний.
- [ ] Восстановить `stripDNSWizardOnlyFields` к минимальному набору
  (`description`, `enabled` — как было до моих правок). Если sing-box 1.12
  где-то требует ещё — добавить отдельным фиксом, не размывая.

### 3. Правильная архитектура preset.outbounds emit

Принцип: **preset.outbounds entry — это parser-format**, такой же как
template.parser_config.outbounds[]. Использовать **existing** native
emit pipeline вместо реинвенции:

- [ ] Вариант A (preferred): на этапе ExpandPreset конвертировать
  `template.Preset.Outbounds[i]` в `configtypes.OutboundConfig` и
  добавить в `parser_config.outbounds[]` ДО запуска parser. Тогда
  native pipeline сделает всю работу.

- [ ] Вариант B (если A не получится из-за timing): отдельный
  `EmitPresetOutboundAsSingbox(parserOutbound, allTags) (json.RawMessage,
  error)` — wrapper над тем же кодом что используется в
  `GenerateSelectorWithFilteredAddOutbounds`, не дублируя логику.

В обоих вариантах:
- options → flatten в top-level (как native).
- filters + addOutbounds → резолв в outbounds list через `filterNodesForSelector`.
- comment → как `// %s\n` префикс (не JSON field).
- wizard / description / title / `_*` / if/if_or — drop.

### 4. mode=update

Для mode=update target (например `proxy-out` который уже сгенерирован
parser'ом):
- Если update меняет `filters` — нужно перегенерировать outbound через
  parser. Это сложно без полного re-run. Альтернатива: пред-патч
  `parser_config.outbounds[]` для соответствующего tag'а **до** запуска
  parser. Тогда parser сам сгенерирует с правильными filters.

- [ ] Перенести `applyOutboundUpdate` на parser_config-prepatching:
  Phase 0 = walk all enabled preset-refs, для каждого mode=update apply
  patch к копии `s.ParserConfig.ParserConfig.Outbounds[]` ПЕРЕД
  `buildSnapshotFromRawCache` / `BuildConfig`.

### 5. Тесты + регресс

- [ ] Golden fixture: config.json от `v0.9.5` с конкретным state.json
  (snapshot of working setup) → сравнить с тем что генерит новый код.
  Diff должен быть минимальный (только preset-добавленные outbounds).
- [ ] `sing-box check` на каждом golden fixture в CI.

### 6. Документация в коде

- [ ] В `core/build/preset_merge.go`: doc-comment описывающий **разницу
  между parser-format и sing-box-format**.
- [ ] В template authoring docs (если есть): что можно/нельзя писать в
  `preset.outbounds[]`.

---

## Acceptance

1. `sing-box check -c config.json` PASSES после `Rebuild Config` с
   реальным user state'ом (Liberty VPN + russian preset + custom rules).
2. На любой ошибке `Rebuild` показывает popup с конкретным sing-box
   error message (уже сделано в `5e56c0b` — оставить).
3. В кодовой базе **одна** функция трансформирующая preset.outbounds entry
   в sing-box format (не три, не пять).
4. Все 24 пакета тестов зелёные.
5. ru VPN 🇷🇺 selector реально содержит RU-tagged subscription nodes в
   `outbounds` array после rebuild.

## Что НЕ делать в этой таске

- НЕ добавлять «ещё один strip» когда что-то новое выпадет — сначала
  понять схему, потом фиксить.
- НЕ дублировать логику emit'а (native parser vs мой preset emit) —
  reuse existing GenerateSelectorWithFilteredAddOutbounds или его
  inner helper.
- НЕ ломать DNS server emit — он работал, не трогать без причины.

---

## Risk / rollback

Если переписывание затянется — fallback: тег `v0.9.5` рабочий. Можно
откатить SPEC 055 целиком + cherry-pick только SPEC 053 (preset bundles
без outbounds) и SPEC 054 (Xray JSON preview bloat fix).
