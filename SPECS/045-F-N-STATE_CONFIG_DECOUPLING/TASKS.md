# TASKS 045 — STATE_CONFIG_DECOUPLING

Каждый блок — самостоятельный коммит. После каждого `go build ./... && go test ./... && go vet ./...` должны быть зелёными. Все блоки идут в одной фича-ветке (текущая работа — на `develop` без коммитов; финальная серия закоммитится одним merge'ом).

**Текущий прогресс:** см. `IMPLEMENTATION_REPORT.md`. В двух словах: фазы 0–4 + 5.A завершены; 5.B в процессе (build зелёный, runtime untested); 5.C/5.D/6/7/8 ⏸.

## ✅ Фаза 0 — LxBox deep-dive
- [x] Прочитать `/Users/macbook/projects/LxBox/`, написать отчёт в `LXBOX_NOTES.md`.

## ✅ Фаза 1 — карта текущей архитектуры лаунчера
- [x] Перечислить все writes `config.json` / `state.json`, callsites broadcast, dirty-маркер. Отчёт в `CURRENT_ARCH_NOTES.md`.

## ✅ Фаза 2 — финальный PLAN
- [x] PLAN.md с архитектурными решениями.

## ✅ Фаза 3 — Фундамент (новые независимые пакеты)

### ✅ 3.1 `core/events` — typed event-bus (SPEC 047)
- [x] SPEC 047 заведён.
- [x] Пакет `core/events/`: `Bus` интерфейс, `MemoryBus` реализация (sync dispatch), типы событий, `Cancel` token.
- [x] Тесты: subscribe/publish, multiple handlers, cancel, panic-isolation handler'а.
- [x] Зелёный `go build ./core/events && go test ./core/events`.

### ✅ 3.2 `core/state` — модель state + миграции
- [x] Пакет `core/state/`: тип `State` без UI-зависимостей.
- [x] `Load(path) → *State, error`: читает v3/v4 (v5 не введён — schema не bump'ался; см. ADR в IMPLEMENTATION_REPORT).
- [x] `Save(path) → error`: атомарная запись (`.tmp` → rename).
- [x] `DiffStates(prev, cur) → Diff{ProxiesChanged, VarsChanged, ...}` для dirty-логики.
- [x] `testdata/` с примерами v3, v4.
- [x] Тесты: round-trip, миграция v3→v4, Diff на типичных правках.

### ✅ 3.3 `core/outboundscache` — snapshot outbounds
- [x] Пакет `core/outboundscache/`: `Snapshot` (StateID, Outbounds, Endpoints, SourceTags, SourceStats, UpdatedAt).
- [x] `Load(path)`, `Save(path)` — атомарно.
- [x] `IsEmpty()`, `IsForState()`.
- [x] Тесты: round-trip, missing-file → empty snapshot, corrupt-file → IsCorrupt error.

### ✅ 3.4 `core/build` — BuildConfig
- [x] Пакет `core/build/`: функция `BuildConfig(BuildContext) (Result, error)`.
- [x] Format helpers (`Indent`, `IndentMultiline`, `FormatSectionJSON`, `FormatCompactJSON`).
- [x] `NormalizeParserConfigText`.
- [x] `MaterializeClashSecretInVars`.
- [x] `BuildOutboundsSection`/`BuildEndpointsSection` с `PreviewStats`.
- [x] `MergeDNSSection` + `DNSConfig` + `ParseDNSRulesText` + helpers.
- [x] `MergeRouteSection` + `RouteConfig` + `RouteRule` + `convertRuleSetToLocalIfNeeded`.
- [x] `ApplyDNSScalarsToVars` + `DNSScalars`.
- [x] **Orchestrator `BuildConfig`** объединяет всё.
- [x] Golden-test harness `core/build/testdata/golden/` + `real-v088` сценарий + helpers (parseGoldenTemplate, dnsConfigFromState, routeConfigFromState).
- [x] **`real-v088` byte-equal parity** под `GOLDEN_RUN_REAL=1`.
- ⏸ Sing-box check на ConfigJSON (deferred to фаза 5.D enhancement).

### Бонусом — попутно

- [x] `ui/wizard/template/` → `core/template/` (move pure-data вниз; 26 callsites обновлены).
- [x] `core/snapshot/` + `core/debugapi/snapshot.go` — `GET /debug/snapshot` (SUB_SPEC_SNAPSHOT.md).

## ✅ Фаза 4 — Расширение StateService

### ✅ 4.1 Два dirty-флага
- [x] `UpdateDirty`, `RestartDirty` (mutex-protected) рядом с `TemplateDirty`.
- [x] Setter'ы: `MarkUpdateDirty/RestartDirty`, `ClearUpdateDirty/RestartDirty`.
- [x] Getter'ы: `IsUpdateDirty/RestartDirty`.
- [x] `ApplyDiff(state.Diff)` мапит Diff на флаги.
- [x] `TemplateDirty` сохранён deprecated до фазы 6.3.
- [x] Тесты на concurrency и event-publication-on-transition (9 кейсов).

### ✅ 4.2 Публикация `StateChanged` events через bus
- [x] StateService держит `EventBus events.Bus`.
- [x] `MarkUpdate/Clear`/`MarkRestart/Clear` публикуют `StateChanged{Changed: [...]}`.
- [x] AppController (`controller.go`) создаёт `events.NewMemoryBus()` и инжектит в StateService при создании.

## Фаза 5 — Перевод pipeline'ов

### ✅ 5.A `config_service.UpdateConfigFromSubscriptions` через новый pipeline
- [x] Pipeline: `loadParserConfigForUpdate (state→fallback config.json)` → `SubstituteParserConfigPlaceholders` → `GenerateOutboundsFromParserConfig` → `outboundscache.Save` → `template.LoadTemplateData` → `buildContextFromState` → `build.BuildConfig` → `atomicWriteConfig`.
- [x] `core.GetController` event-bus `Publish(ConfigBuilt)`.
- [x] Старый `config.UpdateConfigFromSubscriptions + WriteToConfig` НЕ вызывается из этого callsite (но физически в `core/config/updater.go` пока остался — фаза 5.D).
- [x] Build зелёный.
- ⏸ Runtime-tested на dev-машине (требует rebuild лаунчера).

### 🟡 5.B Configurator Save → state-only — **в процессе**
- [x] `presenter_save.go::executeSaveOperation` переписан: state-only, ensureOutboundsParsed убран, buildConfigForSave убран, saveConfigFile убран.
- [x] Save теперь: `saveStateOnly` → `MarkUpdateDirty + MarkRestartDirty + SetTemplateDirty (legacy)` → `EventBus.Publish(StateChanged)` → `showSaveSuccessDialog`.
- [x] Build зелёный.
- ⏸ **РИСК:** UX-регрессия без 5.C — после Save→Run sing-box стартанёт со старым config'ом до первого Update.
- ⏸ `state.Diff(prev, cur)` — пока поднимаем оба маркера; точная per-domain логика (только Update OR только Restart) ждёт хранение `prev_state` в `WizardPresenter`.
- ⏸ Старые функции (`ensureOutboundsParsed`, `buildConfigForSave`, `saveConfigFile`, `populateCheckText callback`, `showValidationErrorDialog`) физически в файле, не вызываются — удалить в фазе 5.D.
- ⏸ Тест: Save не пишет config.json (mtime не меняется).
- ⏸ Runtime-tested на dev-машине.

### ⏸ 5.C Restart pre-rebuild — компенсация UX-регрессии от 5.B
- [ ] Перед kill в `controller.KillSingBoxForRestart`: если `IsRestartDirty()` или `IsUpdateDirty()` → `BuildConfig` (state + cache + template) → atomic-write config.json → `ClearRestartDirty/UpdateDirty`.
- [ ] Аналогично перед `Run` после Save.
- [ ] Тест: после `Save → Restart` config.json содержит изменения из state.

### ⏸ 5.D Снос legacy
- [ ] `core/config/updater.go::WriteToConfig` — удалить (нет callsites).
- [ ] `core/config/updater.go::PopulateParserMarkers` — удалить (нет callsites вне legacy save, который тоже мёртв).
- [ ] `ui/wizard/business/saver.go::SaveConfigWithBackup` — удалить.
- [ ] `ui/wizard/business/saver.go::ValidateConfigWithSingBox` — переехать в `core/build/validate.go` или `core/services/`.
- [ ] `ui/wizard/business/create_config.go::BuildTemplateConfig` — расщепить на preview-обёртку или удалить.
- [ ] Шимы `formatting.go`, остатки в `wizard_dns.go`, `dns_settings_vars.go`, `create_config.go` — удалить.
- [ ] Старые функции в `presenter_save.go` (`ensureOutboundsParsed` и т.д.) — физически удалить.
- [ ] Sing-box check добавить в новый pipeline (validate temp file перед atomic-rename).

## ⏸ Фаза 6 — UI два маркера

### ⏸ 6.1 Кнопка Update в Core Dashboard
- [ ] Маркер `*` показывается, если `IsUpdateDirty()` AND есть подписки.
- [ ] `updateConfigInfo` пересчитывает по `events.Subscribe(ConfigBuilt | StateChanged)`, а не по `UpdateConfigStatusFunc`.

### ⏸ 6.2 Маркер на Restart
- [ ] Новый виджет / индикатор рядом с кнопкой Restart.
- [ ] Показывается, если `IsRestartDirty()` AND `RunningState.IsRunning()`.
- [ ] Tooltip: «Шаблон изменился, перезапустите sing-box чтобы применить».
- [ ] Сбрасывается на successful Restart.
- [ ] i18n: `core_dashboard.restart_dirty_marker`, `core_dashboard.restart_dirty_tooltip` (en+ru).

### ⏸ 6.3 Удалить `TemplateDirty` deprecated
- [ ] Все callsites переведены на `Update/RestartDirty`. Убрать поле и API из `StateService`.
- [ ] Обновить `state_service_test.go`.

## ⏸ Фаза 7 — Rename Wizard → Configurator

### ⏸ 7.1 Переименование папок и пакетов
- [ ] `git mv ui/wizard ui/configurator`.
- [ ] Все `package wizard` → `package configurator`.
- [ ] Все import пути → `singbox-launcher/ui/configurator/...`.
- [ ] Структуры: `WizardModel` → `ConfiguratorModel`, `WizardPresenter` → `ConfiguratorPresenter`, etc.

### ⏸ 7.2 i18n
- [ ] Все ключи `wizard.*` → `configurator.*` в `internal/locale/en.json`, `internal/locale/ru.json`.
- [ ] Тесты `TestAllKeysPresent` проходят.

### ⏸ 7.3 Документация
- [ ] `docs/CREATE_WIZARD_TEMPLATE.md` → `docs/CREATE_CONFIGURATOR_TEMPLATE.md` (+`_RU`).
- [ ] `docs/WIZARD_CHILD_WINDOWS.md`, `docs/WIZARD_STATE.md` → переименовать.
- [ ] README.md + README_RU.md (упоминания «Config Wizard»).
- [ ] `bin/wizard_template.json` имя файла **остаётся** (legacy migration cost > UX win).

## ⏸ Фаза 8 — Документация и release

### ⏸ 8.1 ARCHITECTURE
- [ ] Обновить `docs/ARCHITECTURE.md`: новый слой state, BuildConfig, outboundscache, events bus, два маркера.

### ⏸ 8.2 Release notes
- [ ] `docs/release_notes/upcoming.md` (EN + RU): «Configurator (renamed from Wizard) + state/config decoupling».
- [ ] Migration notes: state.json формат не меняется; на чистой установке без подписок sing-box стартанёт после первого Update.

### ⏸ 8.3 SPEC closure
- [x] `IMPLEMENTATION_REPORT.md` создан, ведётся.
- [ ] Финальная сводка в `IMPLEMENTATION_REPORT.md` после всех фаз.
- [ ] Переименовать папку `SPECS/045-F-N-STATE_CONFIG_DECOUPLING` → `SPECS/045-F-C-STATE_CONFIG_DECOUPLING`.
- [ ] SPEC 047 (events) — закрытие отдельным `IMPLEMENTATION_REPORT.md` в его папке + rename `047-F-N-` → `047-F-C-`.

## ✅ Фаза 9 — Восстановление local-only rule-sets + auto-rebuild на close (v0.9.2 hotfix)

**Контекст регрессии:** SPEC 045/052 убрали из старого pipeline'а шаг «rule_set type:remote → emit type:local если файл скачан». В результате config.json уезжал с `type: remote, url: https://raw.githubusercontent.com/runetfreedom/...`, sing-box при старте пытался скачать через `route.final = auto-proxy-out` (VPN-прокси), httpupgrade-эндпоинт прокси возвращал 404, sing-box падал с FATAL. Старый процесс из v0.8.x: при выборе rule-set'а из UI скачивался файл в `bin/rule-sets/<tag>.srs`, в config.json уезжал `type: local` — sing-box стартовал оффлайн.

**Решение:** восстановить старый контракт «sing-box получает только локальные SRS», но архитектурно чисто.

### ✅ 9.1 Build pipeline — local-only emit
- [x] `core/build/route_merge.go::convertRuleSetToLocalRequired` (rename из `convertRuleSetToLocalIfNeeded`):
  - `inline` → as-is
  - `local` + файл по path есть → as-is; файла нет → **error** «open Configurator → Rules and re-download»
  - `remote` + `bin/rule-sets/<tag>.srs` есть → переписываем на `{type: local, format: binary, path: ...}`; нет → **error** (тот же месседж)
  - неизвестный type → as-is (sing-box разберётся)
- [x] `MergeRouteSection` пробрасывает error → `BuildConfig` → `RebuildConfigIfDirty` не пишет config.json. Sing-box не получает невалидный config никогда.

### ✅ 9.2 State остаётся декларативным
- [x] `custom_rules[i].rule_set[j]` хранит `{type: remote, url}` без изменений после download. Download — операция кеша на диске, не пользовательский edit, dirty marker не дёргает.
- [x] Build pipeline единственный, кто решает что попадает в config.json (поэтому и проверки тоже там).
- [x] Преимущества: state портативен между машинами, нет «хитрых таймингов» когда state мутируется.

### ✅ 9.3 UI gate (уже было)
- [x] `ui/configurator/tabs/rules_tab.go::createRuleEnableCheckbox` (lines 322-331): при клике на enable если SRS файлы не скачаны — триггер download, checkbox блокируется до успеха. Существовало до фазы 9, проверено.

### ✅ 9.4 Content-addressed tag для user-added rules
- [x] `ui/configurator/dialogs/add_rule_dialog.go::srsTagFromURL`: `<filename-без-srs>-<hash8(sha256(url))>`. Same URL → same tag (дедуп через seen-set). Different URL с одинаковым filename → разные tags (collision impossible).
- [x] Template-shipped tags (`ru-blocked-main`, `ads-all`, `games`, ...) не трогаем — handcrafted, уникальность гарантирует автор шаблона.

### ✅ 9.5 Orphan GC `bin/rule-sets/`
- [x] `core/services/srs_downloader.go::DeleteOrphanRuleSets(execDir, knownTags)`: walk `bin/rule-sets/`, для каждого файла tag = name без `.srs`, не в knownSet → `os.Remove`. Папка целиком launcher-managed, без exception на расширение.
- [x] `core/config_service.go::collectAllStageRuleSetTags(execDir)`: union по всем `bin/wizard_states/*.json`, для каждого state walk `CustomRules[].RuleSet[]`, выдрать `tag`. И enabled, и disabled (пользователь может перетоггнуть, лишний download раздражает).
- [x] Вызов из `core/rebuild.go::RebuildConfigIfDirty` после `atomicWriteConfig`. Multi-stage safety тот же что для `bin/subscriptions/` orphan GC из SPEC 052.

### ✅ 9.6 Auto-rebuild на close Configurator
- [x] `ui/configurator/configurator.go::handleCloseButton`: ВСЕ ветки (Save / Discard / SaveInProgress / no-changes) заканчиваются `triggerAutoRebuildOnClose()`. `RebuildConfigIfDirty` сам no-op'нется если ничего не dirty.
- [x] `triggerAutoRebuildOnClose` запускает `ac.RebuildConfigIfDirty()` в goroutine (best-effort, окно уже закрыто). Ошибка логируется, в Core Dashboard статус обновится через `UpdateConfigStatusFunc`.
- [x] Workflow упростился: Open Configurator → enable rules (auto-download) → close → Start. Без явного Update / Rebuild.

### ⏸ 9.7 Тесты (отдельная итерация)
- [ ] `core/build/route_merge_test.go`: переписать `TestConvertRuleSetToLocal*` под новые семантики; добавить `TestConvertRuleSetToLocalRequired_LocalMissing`, `TestConvertRuleSetToLocalRequired_LocalPresent`, `TestConvertRuleSetToLocalRequired_RemotePromoted`, `TestConvertRuleSetToLocalRequired_RemoteMissing`.
- [ ] `core/services/srs_downloader_test.go`: `TestDeleteOrphanRuleSets_RemovesUnknown`, `TestDeleteOrphanRuleSets_PreservesKnown`, `TestDeleteOrphanRuleSets_HandlesMissingDir`.
- [ ] `core/config_service_test.go`: `TestCollectAllStageRuleSetTags_UnionAcrossStates`, `TestCollectAllStageRuleSetTags_IncludesDisabled`.
- [ ] `ui/configurator/dialogs/add_rule_dialog_test.go`: `TestSrsTagFromURL_ContentAddressed`, `TestSrsTagFromURL_DifferentURLsDifferentTags`, `TestSrsTagFromURL_SameURLSameTag`.

### ⏸ 9.8 Документация
- [ ] `docs/ARCHITECTURE.md`: подсекция «SRS local-only» с описанием контракта build pipeline.
- [ ] `docs/release_notes/upcoming.md` (EN + RU): hotfix v0.9.2.

## Гейты качества (на каждом коммите/итерации)

- [x] `go build ./...` — зелёный (текущее состояние).
- [x] `go test ./...` — зелёный (текущее состояние).
- [x] `go vet ./...` — чисто.
- [ ] `golangci-lint run` — не запускался.
- [x] `go test -race ./core/state ./core/build ./core/events ./core/services ./core/outboundscache ./core/snapshot ./core/template` — зелёный.
- [ ] **Runtime тест на dev-машине** — НЕ выполнен; нужно после фазы 5.B+5.C.
