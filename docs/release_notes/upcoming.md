# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

**Не добавлять** сюда мелкие правки **только UI** (порядок виджетов, выравнивание, стиль кнопок без смены действия и т.п.). Писать **новое поведение**: данные, форматы, сохранение, заметные для пользователя возможности.

## EN
### Highlights
- **Proxy chains (detour) for servers and subscriptions.** A source's Settings tab now has a **Detour server (chain)** picker: choose another outbound (a group, a built-in, or another server) and every node of that source dials through it — building a chain (client → hop → node → internet). Works for both single servers and whole subscriptions. Self-referential and cyclic chains are detected and broken automatically (the node falls back to a direct dial) so the core never rejects the config. (SPEC 077)
- **Fixed: subscriptions from panels that route the response by User-Agent.** Some panels (Remnawave/Marzban-style) match a substring of the client User-Agent and served the launcher a full sing-box client-config JSON instead of the base64/URI subscription list — which the launcher couldn't ingest (add-subscription failed/crashed on older builds). The User-Agent is now `LxBox/<v> (desktop; <os>)`: a bare `singbox` substring — the failure trigger that made panels serve JSON — is gone.

### Technical / Internal
- Detour: `Source.DetourTag`/`ProxySource.DetourTag` plumbed both mapping directions; `applySourceDetour` stamps `node.Outbound["detour"]` (skips WireGuard and Xray-Jump nodes); `sanitizeNodeDetours` drops self-refs and breaks node cycles (fail-open) before emission; reuses the existing `GenerateNodeJSON` detour path (SPEC 036). UI picker via `DetourOptions`. Validated against the real `1.13.13-lx.6` core (`sing-box check` accepts a chain, rejects a cycle). (SPEC 077)
- `BuildSubscriptionUserAgent` (and the GitHub/core/SRS download UAs) now emit the `LxBox` brand token with a `desktop` variant tag; no bare `singbox` substring. Regression tests: `core/config/configtypes/useragent_test.go`, updated `TestFetchSubscription_UserAgentFormat`.

## RU
### Основное
- **Цепочки прокси (detour) для серверов и подписок.** На вкладке Settings источника появился выбор **Detour-сервер (цепочка)**: указываете другой outbound (группу, встроенный или другой сервер), и все узлы этого источника идут через него — строится цепочка (клиент → хоп → узел → интернет). Работает и для одиночного сервера, и для целой подписки. Самоссылка и циклы детектируются и разрываются автоматически (узел переходит на прямое соединение), поэтому ядро никогда не отвергнет конфиг. (SPEC 077)
- **Исправлено: подписки от панелей, которые роутят ответ по User-Agent.** Некоторые панели (Remnawave/Marzban-типа) матчат подстроку в User-Agent клиента и отдавали лаунчеру полный sing-box JSON-конфиг вместо base64/URI-списка подписки — а его лаунчер переварить не мог (добавление подписки падало/крашилось на старых сборках). User-Agent теперь — `LxBox/<v> (desktop; <os>)`: bare `singbox`, который и был триггером (панель отдавала JSON), убран.

### Техническое / Внутреннее
- Detour: `Source.DetourTag`/`ProxySource.DetourTag` проброшены в обе стороны маппинга; `applySourceDetour` проставляет `node.Outbound["detour"]` (пропуская WireGuard и узлы с Xray-Jump); `sanitizeNodeDetours` убирает самоссылки и разрывает циклы среди узлов (fail-open) до эмиссии; переиспользует существующий путь `GenerateNodeJSON` (SPEC 036). UI-выбор через `DetourOptions`. Проверено на реальном ядре `1.13.13-lx.6` (`sing-box check` принимает цепочку, отвергает цикл). (SPEC 077)
- `BuildSubscriptionUserAgent` (и UA для скачивания с GitHub/ядра/SRS) теперь выдают бренд-токен `LxBox` с меткой варианта `desktop`; подстроки `singbox` без дефиса нет. Регресс-тесты: `core/config/configtypes/useragent_test.go`, обновлённый `TestFetchSubscription_UserAgentFormat`.
