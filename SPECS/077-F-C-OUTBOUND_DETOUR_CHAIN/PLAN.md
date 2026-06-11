# PLAN 077 — Detour-цепочки

## Архитектура

Detour — свойство источника. Поток: `state.Source.DetourTag` → `ProxySource.DetourTag` → проставляется в `node.Outbound["detour"]` при загрузке узлов → существующая эмиссия пишет `"detour"` в `config.json`. Валидация (висячий/цикл/само) — отдельный проход в генераторе, где известен полный набор тегов. UI — дропдаун из реестра тегов.

```
Source.DetourTag ──ToProxySourceV4/sync_to_legacy──▶ ProxySource.DetourTag
                                                            │
                              LoadNodesFromSource: node.Outbound["detour"]=tag (не wg, не Jump)
                                                            │
              GenerateOutboundsFromParserConfig: validateDetours(allNodes, allTags)
                                                            │
                              GenerateNodeJSON: "detour":"<tag>" (уже есть, :331)
```

## Фазы и изменения по файлам

### Фаза 1 — модель + проброс + применение
| Файл | Изменение |
|---|---|
| `core/state/connections.go` | `Source.DetourTag string` (`json:"detour_tag,omitempty"`, для обоих типов) |
| `core/config/configtypes/types.go` | `ProxySource.DetourTag string` (`json:"detour_tag,omitempty"`) |
| `core/state/adapter_source.go` | `ToProxySourceV4`: проброс `DetourTag` для subscription и server |
| `core/state/sync_to_legacy.go` | то же в обоих ветках |
| `core/state/sync_to_connections.go` | обратный маппинг ProxySource→Source (round-trip) |
| `core/config/subscription/source_loader.go` | в `LoadNodesFromSource` после парсинга узла: если `proxySource.DetourTag != ""` и `node.Scheme != "wireguard"` и `node.Jump == nil` → `node.Outbound["detour"]=DetourTag` |

### Фаза 2 — валидация (fail-open)
| Файл | Изменение |
|---|---|
| `core/config/outbound_generator.go` | новый `sanitizeDetours(allNodes, knownTags)` перед эмиссией: собрать set всех тегов (узлы + selector'ы + endpoints); для каждого узла с `detour`: тег не в set → drop+warn; detour==self → drop+warn; обнаружить цикл по графу detour → разорвать+warn |

### Фаза 3 — UI
| Файл | Изменение |
|---|---|
| `ui/configurator/tabs/source_edit_window.go` | дропдаун «Detour server» в Settings (server и subscription); опции = `GetAvailableOutbounds` минус собственные теги источника + «(none)»; OnChanged → запись в scratch/model → `serializeParserAfterSourceEdit` |
| `ui/configurator/business/*` | хелпер: применить выбранный detour к scratch ProxySource; список опций с фильтром собственных тегов |
| `internal/locale/*` (или где строки) | `wizard.source.label_detour`, `wizard.source.detour_none`, hint |

### Фаза 4 — тесты + докуменация
| Файл | Изменение |
|---|---|
| `core/state/*_test.go` | round-trip Source↔ProxySource с DetourTag |
| `core/config/*_test.go` | генерация server+detour и subscription+detour; sanitizeDetours (висячий/само/цикл/wg-skip/Jump-приоритет); golden-фрагмент |
| `docs/ParserConfig.md` | раздел про detour источника |
| `docs/release_notes/upcoming.md` | EN/RU |
| `IMPLEMENTATION_REPORT.md` | отчёт |

## Ключевые решения
- **Эмиссия не трогается** — `node.Outbound["detour"]` уже сериализуется. Новый код только проставляет и валидирует.
- **Fail-open** на всех проверках detour: узел работает напрямую, генерация не падает.
- **WireGuard и Xray-Jump исключены** из применения detour (debug-лог).
- Валидация в генераторе, а не при загрузке: только там известен полный набор тегов (нужно для висячих/циклов).

## Порядок
Фаза 1 (модель+проброс+применение, с тестами маппинга и генерации) → Фаза 2 (валидация+тесты) → Фаза 3 (UI+локали) → Фаза 4 (докуменация, отчёт). Каждая фаза оставляет `go build/test/vet` зелёными.
