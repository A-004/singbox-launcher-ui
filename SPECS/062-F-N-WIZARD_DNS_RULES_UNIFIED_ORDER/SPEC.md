# SPEC 062-F-N — WIZARD_DNS_RULES_UNIFIED_ORDER

**Status:** New (N)
**Type:** Feature (F) — единый упорядоченный список DNS-правил в Wizard'е (preset + user) с drag ↑↓ контролами, аналогично Rules tab (SPEC 053).
**Depends on:** SPEC 056-R-N (DNS schema redesign — `state.DNS.Rules` уже flat list с `kind` discriminator), SPEC 053 (Rules tab unified ordering — даёт референс-паттерн `RuleOrder + RuleSlot`).
**Не меняет:** wire format `state.DNS.Rules[]` (тот же flat `[]DNSRule{Kind, Ref|Body, Enabled}`), `core/build` emit (уже эмитит в порядке slice'а).
**Closes:** Follow-up из `SPECS/056-R-N-DNS_SCHEMA_REDESIGN/SPEC.md:303` («Preset DNS reorder аналогично»).

---

## Проблема

DNS tab в визарде (`ui/configurator/tabs/dns_tab.go`, `dns_user_rules.go`, `dns_preset_bundled.go`) сейчас рендерит DNS-правила **двумя разделёнными секциями** в фиксированном порядке:

1. **Preset DNS rules** — сверху, под заголовком «Rules (JSON object with "rules" array)». Чекбокс toggle + лок-иконка 🔒 для template-required, View JSON, без перемещения.
2. **User DNS rules** — снизу, под «+ Add», edit/delete контролы, без перемещения.

Sing-box применяет DNS-правила по first-match wins (см. [sing-box DNS rules docs](https://sing-box.sagernet.org/configuration/dns/rule/)). Текущая укладка означает: **preset rules всегда выигрывают у user rules** для пересекающихся match-условий. У юзера нет способа сказать «мой rule для `mysite.ru` должен сработать ДО preset'ового rule на `*.ru`».

Реальный кейс (репорт): юзер добавил user rule `{"domain":"mysite.ru","server":"Test server"}`. Активен preset «Russian domains & IPs» с правилом `{"domain_suffix":["ru"],"server":"direct_dns"}`. Поведение sing-box — domain match идёт по preset'у (он стоит выше), user rule никогда не triggered'ит.

**Похожая проблема** была в Rules tab (route rules) до SPEC 053 — там решили через единый ordered list `model.RuleOrder []RuleSlot{Kind: SlotKindCustom|SlotKindPresetRef, Index}` + drag ↑↓ контролы. Сейчас делаем то же для DNS.

---

## Текущее состояние

### State (`core/state/dns_options.go`)

```go
type DNSOptions struct {
    Servers []DNSServer
    Rules   []DNSRule   // ← УЖЕ flat list с kind discriminator
    ...
}

type DNSRule struct {
    Kind    DNSRuleKind   // "preset" | "user"
    Ref     string        // только для kind=preset (≈ preset.id)
    Enabled bool
    Body    map[string]interface{}  // только для kind=user
}
```

**Хорошая новость:** state-схема уже поддерживает unified ordering. Save/Load round-trip сохраняет slice order. `core/build/dns_merge.go::MergePresetsIntoDNS` emit'ит в том же порядке.

**Плохая новость:** UI не уважает этот order — рендерит kind=preset и kind=user в двух разных секциях, и Save синхронизирует их фиксированно (преsets сверху из template, user rules снизу из `model.DNSRulesText`).

### UI (текущая фрагментация)

- `dns_preset_bundled.go::buildDNSPresetRulesBlock` — рендер preset DNS rules. Читает `model.PresetRefs` + `template.presets[*].dns_rule`.
- `dns_user_rules.go::buildDNSUserRulesBlock` — рендер user DNS rules. Читает `model.DNSRulesText` (JSON-текст), парсит, рендерит по одной строке.
- Между ними — `widget.NewSeparator()`.

### Save flow (sync model → state)

`ui/configurator/models/preset_ref_sync.go::SyncDNSFullToStateV6`:
1. Сначала emit'ит kind=preset entries для активных preset-rule'ов (порядок — из `model.PresetRefs`).
2. Потом emit'ит kind=user entries из `model.DNSRulesText` (порядок — из текста).

`state.DNS.Rules` всегда имеет «presets first, users second» layout — независимо от того, как юзер видит их в UI.

---

## Что меняем

### 1. Storage / model

**Добавить** `model.DNSRuleOrder []DNSRuleSlot` (зеркало `model.RuleOrder` из SPEC 053):

```go
// ui/configurator/models/dns_rule_slot.go (new)
type DNSRuleSlotKind int

const (
    DNSSlotKindUser    DNSRuleSlotKind = iota  // model.DNSUserRules[Index]
    DNSSlotKindPresetRef                       // model.PresetRefs[Index].dns_rule
)

type DNSRuleSlot struct {
    Kind  DNSRuleSlotKind
    Index int
}
```

`model.DNSUserRules []DNSUserRule` (replaces parser-of-DNSRulesText) — отдельная типизированная list вместо JSON-строки. Cтарый `model.DNSRulesText` либо deprecated в пользу typed list, либо остаётся как derived view для JSON-editor mode (одной кнопкой переключаем «list view» ↔ «raw JSON»).

### 2. UI — единый список

В `dns_tab.go` блок «Rules» становится одной VBox-секцией, обходящей `model.DNSRuleOrder` и dispatching по slot.Kind:

```
[↑][↓] ☑  🔗 Russian domains & IPs    [✏][🗑]
[↑][↓] ☑  → Test server · domain=mysite.ru    [✏][🗑]
[↑][↓] ☑  → Block ads · domain_suffix=ads.*    [✏][🗑]
[↑][↓] ☐  🔗 IL local DNS (disabled)    [✏][🗑]
              + Add Rule
```

- 🔗 префикс — preset-row (read-only body, View JSON через клик)
- → префикс — user-row (editable inline или через ✏ диалог)
- 🔒 на preset где `required:true` в template → checkbox grey'нут (toggle блокирован, но drag разрешён)
- Drag ↑↓ работает на индексах `DNSRuleOrder`, не на underlying lists

### 3. Save → state

`SyncDNSFullToStateV6` (rename → `SyncDNSByOrderToState`) меняет шаг сборки `Rules[]`:

```go
out := make([]DNSRule, 0, len(model.DNSRuleOrder))
for _, slot := range model.DNSRuleOrder {
    switch slot.Kind {
    case DNSSlotKindPresetRef:
        pr := model.PresetRefs[slot.Index]
        if pr.dns_rule_disabled { continue }
        out = append(out, DNSRule{
            Kind: DNSRuleKindPreset,
            Ref:  pr.Ref,
            Enabled: pr.Enabled,
        })
    case DNSSlotKindUser:
        ur := model.DNSUserRules[slot.Index]
        if !ur.Enabled { continue } // sing-box не имеет per-rule enabled, skip
        out = append(out, DNSRule{
            Kind:    DNSRuleKindUser,
            Enabled: true,
            Body:    ur.Body,
        })
    }
}
state.DNS.Rules = out
```

State теперь хранит юзерский порядок, не «presets first».

### 4. Load → model

`restoreDNS` (`ui/configurator/presentation/presenter_state.go`):
1. Из `state.DNS.Rules` восстанавливаем `model.DNSUserRules` (kind=user entries) + `model.DNSRuleOrder` (slot per entry, mapping ref→PresetRefs[i] для preset entries).
2. Если в state нет ordering (legacy v6 файл с preset-first-layout) — `RebuildDNSRuleOrder(model)` строит default: всё что в state.DNS.Rules в порядке slice'а.
3. Если в state НЕТ preset-rule'а который активен в model.PresetRefs (новый preset включён через Rules tab но DNS-часть его ещё не sync'нута) — `ReconcileDNSRuleOrder` добавит slot в конец.

### 5. Build pipeline

**Не меняется.** `core/build/dns_merge.go::MergePresetsIntoDNS` уже эмитит `state.DNS.Rules` в порядке slice'а в финальный `config.json::dns.rules[]`. Sing-box first-match wins даст ожидаемое поведение.

### 6. JSON editor mode (раскрытие)

`model.DNSRulesText` остаётся для advanced raw-JSON edit. Кнопка-toggle в UI:
- **List view** (default) — unified ordered list (см. выше)
- **Raw JSON** — текстовый редактор, в котором содержится **только** kind=user правила в текущем порядке (preset-rule'ы остаются нередактируемыми по определению, в raw view скрываются).

Toggle между view'ами:
- list → raw: serialize `model.DNSUserRules` в JSON `{"rules":[...]}` + show editor.
- raw → list: parse `model.DNSRulesText`, replace `model.DNSUserRules` + перестроить `DNSRuleOrder` (user slots — в порядке из текста, preset slots — после них; юзер потом drag'ом перемешает).

---

## Изменения в коде (по файлам)

| Файл | Что |
|---|---|
| `ui/configurator/models/dns_rule_slot.go` (new) | `DNSRuleSlot` + `DNSRuleSlotKind` enum + `RebuildDNSRuleOrder` + `ReconcileDNSRuleOrder` + `CompactDNSRuleOrderIndices`. Зеркало `rule_slot.go`. |
| `ui/configurator/models/wizard_model.go` | Новые поля: `DNSUserRules []DNSUserRule`, `DNSRuleOrder []DNSRuleSlot`. Оставить `DNSRulesText` (deprecated, для JSON editor mode). |
| `ui/configurator/models/preset_ref_sync.go` | `SyncDNSFullToStateV6` → `SyncDNSByOrderToState` (новая сигнатура, читает `DNSRuleOrder`). Старая обёртка вокруг новой для callsite'ов которые ещё не мигрировали. |
| `ui/configurator/presentation/presenter_state.go::restoreDNS` | Восстановить `DNSUserRules` + `DNSRuleOrder` из `state.DNS.Rules`. Fallback `RebuildDNSRuleOrder` для legacy state. |
| `ui/configurator/tabs/dns_tab.go` | «Rules» блок переписан с двух секций на единый ordered list (зеркало `rules_tab.go::buildUnifiedRuleRows`). |
| `ui/configurator/tabs/dns_unified_rules.go` (new) | `buildUnifiedDNSRuleRows` — обход `DNSRuleOrder`, dispatch по slot.Kind. Из `dns_preset_bundled.go` + `dns_user_rules.go` забрать row builders + адаптировать сигнатуры. |
| `ui/configurator/tabs/dns_preset_bundled.go` | Уменьшить — оставить только `buildSinglePresetDNSRuleRow` (renderer одной preset-row для unified list). Остальное удалить. |
| `ui/configurator/tabs/dns_user_rules.go` | Аналогично — `buildSingleUserDNSRuleRow` + toggle list↔raw. |

---

## Migration

- **Legacy state.json (SPEC 056 layout)** — `state.DNS.Rules` уже flat. `restoreDNS` строит `model.DNSRuleOrder` через `RebuildDNSRuleOrder` (preset entries в порядке появления, user entries после, как было). Юзер видит тот же порядок что и раньше — но теперь может перемешать ↑↓.
- **Legacy state.json (pre-SPEC-056)** — `state.DNSOptions.Rules` (`*LegacyDNSOptionsV5`) — обрабатывается тем же путём что v5→v6 migration, никаких новых шагов не требуется.
- **Round-trip совместимость** — Save пишет state.DNS.Rules в юзерском порядке. Старый клиент (без 062) прочитает list, отрендерит presets первыми (хотя они могут быть после user), но build pipeline на старом клиенте всё-равно эмитит в порядке slice'а → правильно работает. Только UI на старом клиенте «соврёт» что preset стоит выше.

---

## Тестирование

- **Unit** — `dns_rule_slot_test.go`: Rebuild / Reconcile / Compact паттерны (зеркало `rule_slot_test.go`).
- **Round-trip** — `presenter_state_test.go`: Save → Load preserves DNSRuleOrder с перемешанными preset/user slots.
- **Build emit** — `core/build/dns_merge_test.go`: rule order в `state.DNS.Rules` → identical order в эмитнутом `config.json::dns.rules[]`.
- **UI smoke** — добавь user rule, drag перед preset → Save → Restart sing-box → user rule wins на match.

---

## Open questions

1. **JSON editor mode для preset-rule'ов** — в текущем UI юзер может посмотреть JSON через «View JSON» в preset row. После 062 это останется как modal. В Raw JSON editor mode — preset rules скрыты по определению (они read-only). Согласен?
2. **Drag handle ergonomics** — Fyne не имеет native drag-and-drop, мы делаем ↑↓ кнопки как в Rules tab. На больших списках (>10 rules) это медленно. Не делаем оптимизацию в первой итерации.
