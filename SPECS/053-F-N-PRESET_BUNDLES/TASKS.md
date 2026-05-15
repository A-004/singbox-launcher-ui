# SPEC 053 — Tasks

## Phase 1 — Pure data types

### Template-side
- [ ] `core/template/preset_types.go`
  - [ ] `Preset` struct (ID, Label, Description, DefaultEnabled, Vars, RuleSet, DNSServers, Rule, DNSRule)
  - [ ] `PresetVar` (Name, Type, Default, Title, Tooltip, Options json.RawMessage, Select, If, IfOr)
  - [ ] `OptionEntry` (Title, Value) — для enum
  - [ ] `PresetRuleSet` (Tag, Type, Format, Rules, URL, If, IfOr)
  - [ ] `PresetDNSServer` (Tag, Type, Server, ServerPort, Path, TLS, Detour, Title, Description, If, IfOr)
  - [ ] Custom UnmarshalJSON для Options (object form для enum, []string для dns_server/outbound)
- [ ] `core/template/preset_types_test.go`
  - [ ] Round-trip Form 1 (inline match)
  - [ ] Round-trip Form 2 (single rule_set)
  - [ ] Round-trip Form 3 (multi rule_set)
  - [ ] Round-trip Form 4 (selective dns_rule)
  - [ ] Round-trip Form 5 (reject sentinel)
  - [ ] Round-trip ru-direct (real-world from SPEC)
  - [ ] Options decoder: enum vs dns_server vs outbound

### State-side
- [ ] `core/state/v6/rule_types.go`
  - [ ] `Rule` header (Kind, Ref, ID, Enabled, Body json.RawMessage)
  - [ ] `PresetBody` (Vars map[string]string)
  - [ ] `InlineBody` (Name, Match, Outbound)
  - [ ] `SrsBody` (Name, SrsURL, Outbound)
  - [ ] `DNSConfig` (Strategy, IndependentCache, Final, DefaultDomainResolver, TemplateServers, ExtraServers, ExtraRules)
  - [ ] `TemplateServerOvr` (Enabled)
  - [ ] `DecodeRule(raw) (Rule, error)` dispatcher
- [ ] `core/state/v6/rule_types_test.go`
  - [ ] Round-trip preset-ref
  - [ ] Round-trip user inline
  - [ ] Round-trip user srs
  - [ ] Unknown kind → error
  - [ ] kind=preset с лишним `id` → strip + warning
  - [ ] kind=inline без `id` → error

## Phase 2 — Template parser + validation

- [ ] `core/template/preset_loader.go`
  - [ ] `LoadPresets(raw) ([]Preset, []Warning)` — non-fatal warnings
  - [ ] Uniqueness validators (preset.id, vars[].name, rule_set[].tag, dns_servers[].tag)
  - [ ] Reference resolvers (rule.rule_set, dns_rule.rule_set/server)
  - [ ] If/IfOr type+existence check (только bool vars из этого preset'а)
  - [ ] Default value validator (∈ options для enum/dns_server/outbound с whitelist)
  - [ ] Select value validator (∈ {"local","global"}, только для type=dns_server)
  - [ ] Vars scope collision check (vs template.vars[*]) — warning
- [ ] `core/template/preset_loader_test.go`
  - [ ] Каждый validation case
  - [ ] ru-direct без warnings
  - [ ] Намеренно-сломанные пресеты дают expected warnings
- [ ] `core/template/loader.go` (модификация)
  - [ ] Parse `presets[]` секции из template
  - [ ] Log warnings через `debuglog`
  - [ ] Сохранить legacy `selectable_rules[]` parsing (одновременно)

## Phase 3 — Preset expansion engine

- [ ] `core/build/preset_expand.go`
  - [ ] `_substitute(obj, vars) any` — рекурсивный текстовый замен
  - [ ] `evalIf(vars, if, if_or) bool` — переиспользует `ParamBoolVarTrue`
  - [ ] `filterByIf(fragments, vars)` — фильтр условных фрагментов
  - [ ] `prefixTags(preset_id, fragments)` — `<preset_id>:<local_tag>`
  - [ ] `filterDnsServers(preset, varsMap)` — только использованные через @dns_server / литерал
  - [ ] `applyOutboundSentinels(rule, outbound)` — wraps ApplyOutboundToRule
  - [ ] `cleanDanglingRuleSetRefs(rule, emittedTags)` — убрать ссылки на отсутствующие
  - [ ] `directOutDetourStrip(dnsServer)` — `detour: direct-out` → strip
  - [ ] `ExpandPreset(preset, body PresetBody, ctx) (Fragments, []Warning)` — public API
- [ ] `core/build/preset_expand_test.go`
  - [ ] Case 1: default varsValues → ожидаемый emit
  - [ ] Case 2: `use_dns_override: false` → no DNS bundle
  - [ ] Case 3: `geoip_enabled: false` → rule_set дроп + ref clean
  - [ ] Case 4: юзер сменил bundled DNS → filter swap
  - [ ] Broken: unresolved @var → skip preset + warning
  - [ ] Broken: unknown var в varsValues (template var удалён) → warning + use default
  - [ ] `direct-out` detour strip из dns_server

## Phase 4 — State v5 → v6 migration

- [ ] `core/state/v6/migration.go`
  - [ ] `MigrateV5ToV6(oldState) (newState, []Warning)` pure func
  - [ ] `custom_rules[]` → `rules[]` с kind detect (inline/srs heuristic по rule_set[0].type)
  - [ ] Generate ULID для user-defined правил
  - [ ] `selectable_rule_states` (legacy) → preset-refs если ID совпадает по label
  - [ ] `dns_options.servers[]` split:
    - [ ] tag совпадает с template-defined → `template_servers[tag] = {enabled}` если != default
    - [ ] tag user-added → `extra_servers[]`
  - [ ] `dns_options.rules[]` → `state.dns.extra_rules[]`
- [ ] `core/state/v6/migration_test.go`
  - [ ] Real v5 fixture (`testdata/state_v5_real.json`) → expected v6
  - [ ] Идемпотентность: v6 → migrate (noop) → identical v6
  - [ ] Round-trip: v5 → v6 → JSON → v6 (no drift)
- [ ] `core/state/load.go` (модификация)
  - [ ] Detection branch: `meta.version == 5/6/unknown`
  - [ ] Backup `state.json.v5.bak` (atomic copy) при первом upgrade'е
- [ ] `core/state/save.go` (модификация)
  - [ ] Always write `meta.version = 6`, `meta.schema = "presets_v1"`

## Phase 5 — Build pipeline integration

- [ ] `core/build/rules_pipeline.go`
  - [ ] `BuildRulesAndDNS(template, state, ctx) (RouteSection, DNSSection)` — pure func
  - [ ] preset-ref → ExpandPreset → append fragments
  - [ ] user inline → emit headless rule_set + route rule
  - [ ] user srs → emit local rule_set (если cached) + route rule
  - [ ] Hijack-dns rule в начале route.rules[]
  - [ ] Merge: identical-skip / first-wins для rule_set и dns_servers по tag
  - [ ] DNS section: `template.dns_defaults.servers.filter(effective_enabled) + bundled + extras`
  - [ ] `effective_enabled(tag, state)` resolver
- [ ] `core/build/rules_pipeline_test.go`
  - [ ] Mixed: preset + inline + srs → корректный merged config
  - [ ] Identical-skip сценарий (два preset'а с одинаковым tag/контентом)
  - [ ] First-wins сценарий (один tag, разный контент) → warning
  - [ ] effective_enabled override (state.dns.template_servers)
- [ ] `core/rebuild.go` (модификация)
  - [ ] Замена legacy applyCustomRules + DNS merge на новый pipeline
- [ ] Golden fixtures `core/build/testdata/golden/`:
  - [ ] `preset_ru_direct_default.json`
  - [ ] `preset_ru_direct_no_dns.json`
  - [ ] `preset_ru_direct_no_geoip.json`
  - [ ] `preset_ru_direct_yandex_doh.json`
  - [ ] `mixed_preset_inline_srs.json`

## Phase 6 — UI: Rules tab refactor

- [ ] `ui/configurator/models/rule_state.go` (модификация)
  - [ ] Заменить `RuleState{Rule: TemplateSelectableRule, Enabled, SelectedOutbound}` на новый `Rule{Kind, Ref, ID, Enabled, Body}` (typed body)
  - [ ] Adapter методы для legacy callsite'ов (на переходный период)
- [ ] `ui/configurator/tabs/rules_tab.go` (модификация)
  - [ ] Tile rendering для всех 3 kind'ов
  - [ ] Summary preset-ref: non-default varsValues
  - [ ] Summary inline: match-resume
  - [ ] Summary srs: `· srs` marker + download кнопка
  - [ ] Broken preset marker `⚠ Broken preset`
- [ ] `ui/configurator/dialogs/edit_preset_rule_dialog.go` (новый)
  - [ ] Универсальный vars renderer по type
  - [ ] outbound picker
  - [ ] dns_server grouped picker (3 секции) или whitelist
  - [ ] enum dropdown с {title, value}
  - [ ] bool checkbox с if/if_or зависимостями (live show/hide зависимых vars)
  - [ ] text/number entry
  - [ ] Preview секция (показывает emit'нутые fragments)
  - [ ] Broken preset: warning баннер + Delete only
- [ ] `ui/configurator/dialogs/add_rule_dialog.go` (модификация)
  - [ ] Удалить legacy selectable_rule import path
  - [ ] Только для inline/srs создания
- [ ] `ui/configurator/tabs/library_rules_dialog.go` (модификация)
  - [ ] "Add to Rules" создаёт preset-ref `{kind: "preset", ref, body: {vars: {}}}`
  - [ ] Disabled state если ref уже в state.rules[]

## Phase 7 — UI: DNS tab refactor

- [ ] `ui/configurator/business/dns_resolve.go` (новый)
  - [ ] `ResolveDNSServers(template, state, activePresets) [](tag, source, effectiveEnabled, raw)` — для UI
  - [ ] `effectiveEnabledForTemplate(tag, state) bool` — override resolver
- [ ] `ui/configurator/tabs/dns_tab.go` (модификация)
  - [ ] Секция "Default DNS servers (from template)" — checkbox per template-сервер
  - [ ] `(overridden)` marker если значение != default_enabled
  - [ ] Секция "From active presets (read-only)" — bundled DNS-серверы
  - [ ] Секция "Extra servers (user-defined)" — `state.dns.extra_servers[]`
  - [ ] "Extra rules" JSON editor — `state.dns.extra_rules[]`
- [ ] Checkbox handler пишет в `state.dns.template_servers[tag].enabled`

## Phase 8 — Content + cleanup + docs

- [ ] `bin/wizard_template.json`:
  - [ ] Добавить `presets[]` секцию с private-ips-direct, ru-direct (real-world), block-ads
  - [ ] Удалить `route.rule_set[]` (rule_set'ы переехали в preset'ы)
  - [ ] `dns_options.servers[].enabled` → `default_enabled`
  - [ ] Удалить `dns_options.rules[]` (теперь только из presets/extras)
  - [ ] Удалить `selectable_rules[]` если все мигрированы в `presets[]`
- [ ] `core/template/loader.go`:
  - [ ] Deprecated handling старой `selectable_rules[]` (parse но warning) — для legacy template'ов в кеше
- [ ] `docs/RELEASE_PROCESS.md`:
  - [ ] §5.2 bump RequiredTemplateRef теперь критично — preset content distributed via pin
- [ ] `docs/ARCHITECTURE.md`:
  - [ ] Раздел "SPEC 053 — Preset bundles"
- [ ] `docs/release_notes/upcoming.md`:
  - [ ] SPEC 053 entry (EN + RU): thin-ref пресеты, parametrized DNS, conditional fragments
- [ ] `SPECS/053-F-N-PRESET_BUNDLES/IMPLEMENTATION_REPORT.md`:
  - [ ] Final implementation report
- [ ] Rename SPEC dir: `053-F-N-` → `053-F-C-`
- [ ] CI: `go build ./... && go test ./...` зелёные

## Out of scope (отдельные SPEC'и в будущем)

- [ ] **SPEC 054 PRESET_IMPORT_EXPORT** — JSON import/export пользовательских пресетов
- [ ] **SPEC 055 LIVE_VARS_RECONFIGURE** — изменение varsValues без полного reconfigure sing-box (требует sing-box runtime API)
- [ ] **SPEC 056 PRESET_DIAGNOSTICS_UI** — debug view раскрытого preset'а с expand trace
