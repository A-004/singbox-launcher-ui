# SPEC 070 — Живой отчёт о прогрессе

> Обновляется по ходу работы. Источник правды о том, что сделано и что осталось.
> Стартовая точка: HEAD `f9f2d06`, ветка `develop`, дерево чистое.

## Текущая фаза: **P0 — Инспекция**

## Лог

### 2026-06-08 — старт
- Заведена таска #101, SPEC 070, 15-мин safety-loop.
- Базовая карта репо: ~81k LOC, ~50 пакетов. Монолиты (LOC): core_dashboard_tab 1482,
  clash_api_tab 1422, add_rule_dialog 1146, outbound_generator 1086, edit_dialog 1071,
  config_service 1066, share_uri_encode 883, source_tab 790, source_edit_window 781,
  process_service 759, controller 747.
- Запускаю Phase 0 workflow (параллельные ридеры → карта архитектуры + decision-sheet).

## Сделано (коммиты)
- `ffb6f7a` docs(spec070): план + трекер.
- `acaae74` refactor(spec070) P1 safe fixes: dedup matchesPlatform→VarAppliesOnGOOS;
  source delete-handler MarkAsChanged; preview cap → const previewNodeCap; gofmt loader.go.
- _(pending commit)_ inline osStatLocal → os.Stat в core/build/preset_merge.go.

### P0 — статус
Workflow `wf_5c40ebf9-185` (9 zone-readers + synthesis) запущен, ждём результат для
систематического P2–P6. Параллельно сделаны независимые P1-items выше.

## UI-изменения для ревью пользователя
1. **Удаление источника теперь зажигает кнопку Save** (`source_tab.go`): раньше после
   удаления строки источника состояние не помечалось dirty и Save не активировался —
   приходилось делать ещё одно изменение. Теперь delete сразу помечает изменения.

## Открытые риски / решения
- gofmt-дрейф в ~60 файлах (CI не гейтит) — сделаю единым sweep-коммитом в конце,
  чтобы не пересекаться с декомпозицией.
- SetToolTip-дедуп, EN→locale.T, чистка исторических комментариев — отложены до
  synthesis (нужен точный перечень мест; пересекаются с P4-декомпозицией).
