# IMPLEMENTATION_REPORT 077 — Detour-цепочки для server / subscription

**Статус:** реализовано, тесты зелёные, проверено на реальном ядре. Дата: 2026-06-11.

## Что сделано

Источник (одиночный сервер и подписка) получил выбор **detour-сервера** — все его узлы dial'ятся через указанный outbound, строя proxy-цепочку (`A через B`). Переиспользован существующий механизм эмиссии `detour` (SPEC 036, Xray dialerProxy) — генератор почти не менялся.

### Фаза 1 — модель + проброс + применение
- `Source.DetourTag` ([core/state/connections.go](../../core/state/connections.go)) и `ProxySource.DetourTag` ([configtypes/types.go](../../core/config/configtypes/types.go)).
- Проброс в обе стороны: `ToProxySourceV4` ([adapter_source.go](../../core/state/adapter_source.go)), `sync_to_legacy.go`, `sync_to_connections.go`.
- `applySourceDetour` ([source_loader.go](../../core/config/subscription/source_loader.go)): проставляет `node.Outbound["detour"]` каждому узлу источника. Пропускает WireGuard (endpoint, не dial-based) и узлы с Xray-Jump (подписка навязала свою цепочку — она побеждает).

### Фаза 2 — валидация (fail-open)
- `sanitizeNodeDetours` ([outbound_generator.go](../../core/config/outbound_generator.go)) перед эмиссией: убирает самоссылку (`detour==self`) и разрывает циклы среди узлов (DFS-раскраска по единственному out-ребру). Узел переходит на прямое соединение, генерация не падает.
- **Висячий detour на тег шаблонной/preset-группы НЕ отбрасывается** — теги этих групп известны только при финальной сборке `config.json`, дроп был бы ложным. UI предлагает только валидные теги; ручной невалидный тег ядро отвергнет с явной ошибкой.

### Фаза 3 — UI
- Дропдаун «Detour server (chain)» в Settings-вкладке окна редактирования источника ([source_edit_window.go](../../ui/configurator/tabs/source_edit_window.go)) — для server и subscription.
- `DetourOptions` ([business/detour.go](../../ui/configurator/business/detour.go)): «(none)» + `GetAvailableOutbounds` минус собственные группы источника; висячий прежний выбор остаётся видимым/сбрасываемым.
- Персистенция: `applyProxyEditToSource` + `ToProxySourceV4` пробрасывают DetourTag, выбор переживает переоткрытие окна.
- Локали `wizard.source.label_detour` / `detour_none` / `detour_hint` (en + ru).

## Проверки

- `go build ./...`, `go test ./...`, `go vet ./...` — зелёные.
- **Реальное ядро `1.13.13-lx.6`** (`sing-box check`): detour-цепочка (`node-A` → `hop-B`) **принимается**; циклический detour (`A↔B`) **отвергается ядром** — что подтверждает необходимость `sanitizeNodeDetours` (мы разрываем цикл до того, как он дойдёт до ядра).
- Тесты: маппинг round-trip (detour_mapping_test.go), применение/eligibility (detour_test.go), валидация self/2-cycle/3-cycle/external-kept (detour_sanitize_test.go), UI-опции (business/detour_test.go), UI-персистенция (tabs/detour_persist_test.go).

## Ограничения / Assumptions

- **Ссылка по тегу**, не по `Source.ID` — консистентно со всем кодом (правила/селекторы/`AddOutbounds`). Нестабильность тега server-цели при переименовании страхуется fail-open валидацией; переход на ref-by-ID — возможный follow-up.
- **WireGuard-узлы** detour не получают (MVP; endpoint-dialer семантика отличается).
- **Узел с Xray-Jump** игнорирует пользовательский detour (Jump приоритетнее).
- Висячий detour на шаблонную группу не валидируется на этапе генерации (см. Фаза 2).
