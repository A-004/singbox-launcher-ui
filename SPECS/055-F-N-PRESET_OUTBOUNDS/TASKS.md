# SPEC 055 — Tasks

**Status:** Все галочки сброшены. Реализация была сделана с
архитектурной ошибкой (post-merge поверх native pipeline → launcher-only
поля утекали в финал). Переписывается через `SPECS/056-B-N-OUTBOUNDS_PARSER_RESTORE`
(pre-patch parser_config). SPEC.md / PLAN.md 055 остаются как
product-level спецификация желаемой семантики — они корректны.

См. **`SPECS/056-B-N-OUTBOUNDS_PARSER_RESTORE/PLAN.md`** для актуального
плана реализации. Текущий чек-лист 055 оставлен как справочный (что
было запланировано feature-wise), но **выполняется в рамках 056**.

## Phase 1 — Types & loader

- [ ] `core/template/preset_types.go`
  - [ ] `Outbounds []PresetOutbound` field в `Preset`
  - [ ] `PresetOutbound` struct (Mode, Tag, Type, Options, Filters, AddOutbounds, PreferredDefault, Comment, Wizard, If, IfOr)
- [ ] `core/template/preset_loader.go::validatePresetOutbounds`:
  - [ ] mode ∈ {"", "add", "update"} (empty → "add"; unknown → strip)
  - [ ] tag non-empty
  - [ ] mode=add → type required
  - [ ] mode=update → type warned
  - [ ] tag uniqueness within preset
- [ ] `core/template/preset_outbounds_test.go` — 7 cases

## Phase 2 — Expand engine

- [ ] `core/build/preset_outbounds.go::ExpandPresetOutbounds(preset, vars)`
  - [ ] Substitute `@var` в options/filters/addOutbounds
  - [ ] Filter by if/if_or
  - [ ] Convert to `configtypes.OutboundConfig` (parser-format)
  - [ ] Drop control fields (mode, if, if_or) из output
  - [ ] Drop type для mode=update

## Phase 3 — Pre-patch pipeline (заменяет старую Phase 3)

- [ ] `core/build/preset_outbounds.go::ApplyPresetOutboundsToParserConfig`
  - [ ] Deep-clone parser_config.Outbounds[]
  - [ ] Алгоритм per-preset emit in RuleOrder
  - [ ] mode=add: identical-skip / first-wins
  - [ ] mode=update: lookup target, apply patch
  - [ ] `applyOutboundUpdate(target, patch)` типизированный helper:
    - [ ] filters → replace
    - [ ] addOutbounds → union (`unionStringList`)
    - [ ] options.* → replace per-field
    - [ ] wizard → replace
    - [ ] type → drop
    - [ ] tag → drop
    - [ ] comment → replace
    - [ ] preferredDefault → replace
- [ ] `cleanDanglingOutboundRefInRule(rule, finalTags, fallback)`:
  - [ ] sentinel reject/drop preserved
  - [ ] outbound ref не в finalTags → fallback или drop
- [ ] `CleanDanglingOutboundsInRouteRules`

## Phase 4 — Build integration

- [ ] `core/build/build.go::BuildContext.ParserConfig`
- [ ] `core/config_service.go::buildContextFromState` — вызов `ApplyPresetOutboundsToParserConfig`
- [ ] `ui/configurator/business/create_config.go::BuildPreviewConfig` — то же для preview
- [ ] case "route": `CleanDanglingOutboundsInRouteRules` после `MergePresetsIntoRoute`
- [ ] Integration через existing test suite — все 24 пакета зелёные

## Phase 5 — UI

- [ ] `ui/configurator/business/outbound.go`
  - [ ] `collectActivePresetOutboundTags(model) []string`
  - [ ] `GetAvailableOutbounds(model)` — append preset tags
- [ ] `ui/configurator/tabs/rules_unified_rows.go`
  - [ ] Refresh tab on outbound-affecting preset enable/disable toggle (с anti-loop защитой)

## Phase 6 — Template content + cleanup

- [ ] `bin/wizard_template.json`:
  - [ ] Удалить `filters: !RU` из global `proxy-out` и `auto-proxy-out`
  - [ ] Удалить `ru VPN 🇷🇺` global selector
  - [ ] Добавить в `ru-inside` preset:
    - [ ] `mode: "update"` для proxy-out + auto-proxy-out → `filters: !RU`
    - [ ] `mode: "add"` для `ru VPN 🇷🇺` selector
  - [ ] `russian` preset: `mode: "update"` proxy-out + auto-proxy-out
  - [ ] `ru-blocked` preset: то же

## Phase 7 — Docs

- [ ] `docs/release_notes/upcoming.md` — SPEC 056 entry (EN + RU)
- [ ] `SPECS/056-B-N-OUTBOUNDS_PARSER_RESTORE/IMPLEMENTATION_REPORT.md` — финальный отчёт
- [ ] Rename SPEC dir: `055-F-N-` → `055-F-C-` после QA-теста
- [ ] Rename SPEC dir: `056-B-N-` → `056-B-C-` после QA-теста

## Out of scope (future)

- [ ] SPEC 057 — explicit cross-preset dependencies
- [ ] SPEC 058 — `mode: "replace"` (destructive full-replace)
- [ ] SPEC 059 — preset.inbounds (per-preset inbound configuration)
