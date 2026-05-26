# SPEC 063-R-M — RULE_IDENTITY_COMPUTED

**Status:** Merged (M) — develop, локальный коммит. Релизные ноты в `docs/release_notes/upcoming.md`.
**Type:** Refactor (R) — выпиливаем redundant `state.Rule.ID` поле, identity = pure function over `Rule`. Заодно расширяет issue #77 fix (закрыт ранее в `e1c9917`) — теперь identity и filename полностью разведены: identity ↔ body.name, filename ↔ URL.
**Depends on:** SPEC 058 (state shape — outbound ref+updates), SPEC 060 (state v6 unified namespace).
**Не меняет:** sing-box config.json format; preset SRS filename convention (`build.SRSTagFromURL`); семантику routing rules.

---

## Проблема

`state.Rule` в текущей schema содержит **избыточное поле** `ID`:

```jsonc
// state.json — kind=srs rule
{
  "kind": "srs",
  "id":   "rule-YT",              // ← дублирует body.name через "rule-"+sanitize(body.name)
  "enabled": true,
  "body": {
    "name":     "YT",             // ← real identity
    "srs_url":  "https://...",
    "outbound": "direct-out"
  }
}
```

`r.ID` всегда вычислим из `body.name` (для `kind=inline`/`srs`) или из `r.Ref` (для `kind=preset`). Хранение его в state — pure redundancy:

```go
// ui/configurator/models/preset_ref_sync.go::stableRuleID — текущее поведение
func stableRuleID(rs *RuleState) string {
    if rs.Rule.Label == "" { return "rule-unnamed" }
    return "rule-" + sanitizeIDPart(rs.Rule.Label)
}
```

Cвязанные грабли:

1. **Двойная identity-модель:** для `kind=preset` identity = `r.Ref`; для `kind=inline`/`srs` identity = `r.ID`. Validator (`DecodeBody`) асимметричен (preset MUST not have id; inline/srs MUST have id). Каждый callsite должен знать про эту асимметрию.

2. **Issue #77 (user-SRS filename):** `core/build/preset_merge.go::CollectSrsCachedPaths` использует `r.ID` как basename для filename:
   ```go
   out[r.ID] = execDir + "/bin/rule-sets/" + r.ID + ".srs"
   ```
   Но downloader (`services.GetSRSEntries` → `customRuleSRSEntries`) сохраняет файл под `wizard-tag` из rule_set JSON (basename URL). File path в config не совпадает с file на диске → sing-box check падает → orphan GC удаляет реально-скачанный файл. См. https://github.com/Leadaxe/singbox-launcher/issues/77.

3. **Filename ≠ identity:** filename файла на диске должна быть привязана к URL (чтобы 2 разных правила на одинаковый URL дедуплицировались в один файл). Identity правила — к label/name (чтобы юзер мог иметь 2 правила на разных URL с разными outbound'ами). Conflate'нуты в `r.ID` сейчас — отсюда #77.

---

## Целевая модель

### Identity

`state.Rule` идентифицируется через **чистую функцию** `state.StableRuleID(r) string`:

```go
// core/state/rule_identity.go (новый файл)
//
// StableRuleID — pure-function identity правила. Не stored. Source of truth.
//
//   preset    → r.Ref           (template preset_id)
//   inline    → sanitize(body.name)
//   srs       → sanitize(body.name)
//   ошибка/unknown → "unnamed"
//
// sanitize: alphanumeric + ['-', '_', '.'], остальное → '_', бэкап trim.
// Преобразование совпадает с прежним sanitizeIDPart (вынесено в этот же файл).
func StableRuleID(r Rule) string {
    if r.Kind == RuleKindPreset {
        return r.Ref
    }
    body, err := r.DecodeBody()
    if err != nil { return "unnamed" }
    var name string
    switch b := body.(type) {
    case *InlineBody: name = b.Name
    case *SrsBody:    name = b.Name
    default:          return "unnamed"
    }
    if name == "" { return "unnamed" }
    return sanitizeIDPart(name)
}
```

### Schema change

Поле `ID string` **удаляется** из `state.Rule` struct. JSON unmarshal Go игнорирует unknown fields по дефолту — legacy state.json с `"id":"rule-YT"` загружается без error, поле просто не используется. Save больше не эмитит `"id"` field.

`state.Rule.Ref` остаётся — для `kind=preset` это и есть identity (lookup в template).

### Validator (DecodeBody)

После refactor:

| kind | r.Ref | body |
| --- | :---: | :---: |
| preset | required | optional (для preset Vars) |
| inline | MUST be empty | required, `Name != ""` |
| srs | MUST be empty | required, `Name != "" && SrsURL != ""` |

Уникальность: state validator проверяет что `StableRuleID(r)` уникален среди rules (вместо текущей проверки уникальности `r.ID`).

### Filename — separate concept

Filename для SRS файлов остаётся URL-derived, как сейчас для preset SRS:

```go
filename = build.SRSTagFromURL(srs_url) + ".srs"
//   = "<sanitize(URL-basename)>-<sha256(rawURL)[:8]>.srs"
```

Это **issue #77 fix**. Применяется ТРИ местам:

| Site | Сейчас | После |
| --- | --- | --- |
| `core/build/preset_merge.go::CollectSrsCachedPaths` | `out[r.ID] = .../<r.ID>.srs` (label-based, mismatched) | decode body → `build.SRSTagFromURL(SrsURL)`, `out[StableRuleID(r)] = .../<url-tag>.srs` |
| `ui/configurator/tabs/rules_tab.go::customRuleSRSEntries` | `SRSEntry.Tag` из wizard's rule_set.tag JSON | override `SRSEntry.Tag = build.SRSTagFromURL(URL)` |
| `core/config_service.go::collectAllStageRuleSetTags` | tags из preset rule_sets (URL-derived) | + добавить URL-derived tags для kind=srs rules |

Преимущества URL-derived filename:
- 2 разных rules на одинаковый URL → один файл на диске, скачивается раз
- Rule rename (изменение label/name) не ломает кеш SRS файла
- Семантически согласуется с preset SRS naming (уже URL-derived)
- Невозможен #77-style mismatch — single source of truth (URL)

---

## Что меняется по компонентам

### `core/state/`

- **Новый файл `rule_identity.go`:** `StableRuleID(r) string` + `sanitizeIDPart(s) string` (перенесён из `ui/configurator/models/preset_ref_sync.go`).
- **`rule_types.go::Rule` struct:** удалить поле `ID string`.
- **`rule_types.go::DecodeBody`:** для kind=inline/srs убрать проверку `r.ID != ""`, добавить `body.Name != ""` (а для srs — также `body.SrsURL != ""`).
- **State uniqueness validator** (если есть в `load.go`/`validate.go`): replace `r.ID` checks → `StableRuleID(r)`.
- **`diff.go::IDChanged`:** rename → `IdentityChanged`, compare via `StableRuleID(prev) != StableRuleID(cur)`.

### `core/build/`

- **`preset_merge.go::CollectSrsCachedPaths`:**
  ```go
  for _, r := range rules {
      if r.Kind != state.RuleKindSrs { continue }
      body, err := r.DecodeBody()
      if err != nil { continue }
      sb, ok := body.(*state.SrsBody)
      if !ok || sb.SrsURL == "" { continue }
      tag := srsTagFromURLLocal(sb.SrsURL)  // URL-derived, не r.ID
      out[state.StableRuleID(r)] = execDir + "/bin/rule-sets/" + tag + ".srs"
  }
  ```
  Key map'а = `StableRuleID(r)` (для совместимости с lookup в `rules_pipeline.go`). Value path = URL-derived filename.

- **`rules_pipeline.go:157`:** `tag := "user:" + rule.ID` → `tag := "user:" + state.StableRuleID(rule)`. (StableRuleID для inline/srs возвращает sanitize(name) — sing-box-tag-валидно.)
- **`resolve_route.go`:** все `SrsID: rule.ID` / `InlineID: rule.ID` → через `state.StableRuleID(rule)`.

### `ui/configurator/`

- **`models/preset_ref_sync.go::stableRuleID`:** функция удаляется. Конверсия legacy CustomRule → state.Rule больше не writes `ID` field (его нет в struct).
- **`models/preset_ref_sync.go` slot lookup:** `crByID[r.ID]` → `crByName[StableRuleID(r)]`. Key = identity.
- **`tabs/rules_tab.go::customRuleSRSEntries`:** override `SRSEntry.Tag = build.SRSTagFromURL(URL)` для всех entries.

### `core/`

- **`config_service.go::collectAllStageRuleSetTags`:** новая ветка — для каждого kind=srs rule добавить `addTag(build.SRSTagFromURL(SrsBody.SrsURL))` в knownTags. Иначе orphan GC удалит реально-скачанный файл.

### Тесты

- **Все `state.Rule{ID: "..."}`** в test fixtures → `state.Rule{Body: jsonMarshal(InlineBody{Name: "..."} or SrsBody{Name: "...", SrsURL: "..."})}`.
- **`StableRuleID` unit tests:** все 3 kinds + edge cases (empty name, unknown kind, undecodable body).
- **State load tests:** legacy state.json с `"id":"rule-YT"` загружается без error, identity = "YT" (а не "rule-YT").
- **Issue #77 integration test:** добавить kind=srs rule с URL → build config.json → `rule_set[].path` указывает на тот же файл, который downloader save'нёт (по URL-derived tag).

---

## Migration

**Гладкая, без version bump'а:**

- **Legacy state.json с `"id"` полем:** Go JSON unmarshal игнорирует unknown fields — `"id"` просто не записывается никуда. Identity вычисляется на лету через `StableRuleID`. На следующем save поле не эмитится (его нет в struct).
- **Старые .srs файлы в `bin/rule-sets/` под label-based именами** (например `rule-YT.srs` от bug #77): орфаны после load — будут удалены GC при следующем rebuild. Re-download автоматический (`runSRSDownloadAsync` спавнится из rules tab open).
- **Никаких backup'ов state.json.pre-063.bak** — изменение wire-format minor (drop одного поля), reversible: если откатить версию, legacy launcher просто recompute `r.ID = "rule-" + sanitize(body.name)` при следующем save.

---

## Edge cases

1. **Два правила с одинаковым `body.name`:** state uniqueness validator падает с дуплицирующейся identity. Та же семантика как сейчас (два правила с одинаковым `r.ID = "rule-" + sanitize(label)` тоже дают collision). UX блокировка — wizard валидирует label на add/edit.
2. **`body.name` пуст** (юзер добавил правило без label): `StableRuleID = "unnamed"` (как было). Уникальность ограничивается одним «unnamed» правилом — поведение наследует legacy.
3. **`body.name` содержит unicode/special chars** (например «Мой YT» или «youtube|porno»): `sanitizeIDPart` сводит к ASCII (alphanumeric + `-_.`), остальное → `_`. Сохраняется prefix human-recognizable.
4. **Race: 2 одновременных load'а** (один с legacy id, второй с новым) — каждый вычисляет identity независимо, результат идентичен. Atomic file writes (SPEC 041) гарантируют что save видит full state.
5. **Wire-format breaking для external tools:** state.json больше не содержит `"id"` для kind=inline/srs. Если есть внешние consumers (Debug API responses, MCP-обёртки) — они должны вычислять identity сами через тот же алгоритм. Внутренний код пользуется `state.StableRuleID(r)`.

---

## Tests (acceptance criteria)

1. **`TestStableRuleID_AllKinds`** — для каждого kind возвращает корректную identity (preset → Ref, inline → sanitize(body.name), srs → sanitize(body.name)).
2. **`TestStableRuleID_EdgeCases`** — empty name → "unnamed"; unknown kind → "unnamed"; undecodable body → "unnamed"; unicode/special chars → ASCII-sanitized.
3. **`TestLoadState_LegacyIDIgnored`** — state.json с `"id":"rule-YT"` + body.name="YT" → after load, identity = "YT" (не "rule-YT").
4. **`TestSaveState_DoesNotEmitID`** — после save state.json НЕ содержит `"id"` field для kind=inline/srs.
5. **`TestSaveLoadRoundTrip_IdentityStable`** — save → load → identity unchanged.
6. **`TestUniquenessValidator_DuplicateName`** — два rules с одинаковым `body.name` → load reject.
7. **`TestIssue77_SRSFilenameMatchesDisk`** — integration: создать kind=srs rule с URL → build config.json → assert `config.route.rule_set[].path` совпадает с filename, который выдаст downloader (`build.SRSTagFromURL(URL) + ".srs"`).
8. **`TestCollectSrsCachedPaths_URLDerived`** — `CollectSrsCachedPaths` для kind=srs rule возвращает path с URL-derived filename, не с `r.ID`.
9. **`TestCollectAllStageRuleSetTags_IncludesUserSRS`** — для state с kind=srs rule, `collectAllStageRuleSetTags` включает `build.SRSTagFromURL(SrsURL)` в knownTags (иначе GC бы удалил файл).

---

## Phases

### Phase 1 — `StableRuleID` + sanitize move
- Создать `core/state/rule_identity.go` с `StableRuleID` + `sanitizeIDPart` (перенос из UI).
- Unit tests `TestStableRuleID_*`.
- **Acceptance:** `go test ./core/state/...` green.

### Phase 2 — Drop `state.Rule.ID` field + validator update
- Удалить `ID string` из `state.Rule` struct.
- `DecodeBody`: убрать проверки `r.ID != ""`, добавить `body.Name != ""`.
- State load: validator uniqueness через `StableRuleID(r)`.
- Tests: `TestLoadState_LegacyIDIgnored`, `TestSaveState_DoesNotEmitID`, `TestSaveLoadRoundTrip_IdentityStable`, `TestUniquenessValidator_DuplicateName`.
- **Acceptance:** state package tests green, save→load round-trip stable.

### Phase 3 — Callsite refactor
- `core/state/diff.go::IDChanged` → `IdentityChanged`.
- `core/build/rules_pipeline.go:157`, `core/build/resolve_route.go` SrsID/InlineID — все `r.ID` → `state.StableRuleID(r)`.
- `core/build/preset_merge.go::CollectSrsCachedPaths`: key = `StableRuleID(r)`, value = URL-derived filename.
- `core/config_service.go::collectAllStageRuleSetTags`: добавить ветку для kind=srs (URL-derived tags).
- `ui/configurator/models/preset_ref_sync.go`: удалить `stableRuleID`, slot lookup ключи на identity.
- `ui/configurator/tabs/rules_tab.go::customRuleSRSEntries`: override `SRSEntry.Tag = build.SRSTagFromURL(URL)`.
- Tests: `TestCollectSrsCachedPaths_URLDerived`, `TestCollectAllStageRuleSetTags_IncludesUserSRS`, `TestIssue77_SRSFilenameMatchesDisk`.
- **Acceptance:** full test suite green; manual test issue #77 reproducer.

### Phase 4 — Test fixtures cleanup
- Все test files с `state.Rule{ID: "..."}` → `state.Rule{Body: jsonMarshal(...)}`.
- Verify нет remaining `r.ID` references вне `StableRuleID` impl.
- **Acceptance:** `grep -rn "\.ID\b" core/state/ core/build/ ui/configurator/` показывает только StableRuleID-related uses.

### Phase 5 — Build + reinstall + manual repro
- Build via `./build/build_darwin.sh -i arm64`.
- Manual test: add user SRS rule (URL `.../geosite-youtube.srs`, label "YT") → Save → verify в `bin/rule-sets/` есть файл с URL-derived именем, sing-box стартует, route работает.
- Manual test: rename rule label → Save → verify SRS файл переcкачался под новым identity (или нет — если filename only URL-derived, то старый файл переиспользуется; check expected).

### Phase 6 — Release notes + commit
- `docs/release_notes/upcoming.md` — entry в EN/RU: «Issue #77 fix: user-SRS rules больше не теряют скачанный файл; identity refactor — `r.ID` поле выпилено».
- Commit: `refactor(state): drop redundant Rule.ID, compute identity from body.name (closes #77)`.

---

## Out of scope

- **Wire-format version bump.** Refactor mild — drop одного поля, JSON unmarshal forgiving. Не bump'аем v6 → v7.
- **External Debug API breaking change handling.** API consumers сами по-новому считают identity (через тот же алгоритм). Не вводим compatibility shim.
- **Migration .bak файлов.** Никаких state.json.pre-063.bak — изменение reversible на load.
- **UI changes.** Identity-rename только меняет internal model; UI всё равно показывает `body.name` (human label). Ничего нового на экране.
- **Refactor preset SRS path.** `build.SRSTagFromURL` для presets уже работает корректно. Этот SPEC расширяет ту же конвенцию на user-SRS.

---

## Кредит

Bug report: [@Deuvos](https://github.com/Deuvos) в [issue #77](https://github.com/Leadaxe/singbox-launcher/issues/77) — конкретный repro + лог.
Архитектурное замечание о redundancy `r.ID` ↔ `body.name`: maintainer review.
