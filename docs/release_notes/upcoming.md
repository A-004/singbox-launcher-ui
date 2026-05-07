# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

**Не добавлять** сюда мелкие правки **только UI** (порядок виджетов, выравнивание, стиль кнопок без смены действия и т.п.). Писать **новое поведение**: данные, форматы, сохранение, заметные для пользователя возможности.

## EN
### Highlights

- **Fix: NaïveProxy outbound now ships authentication credentials.** [#69](https://github.com/Leadaxe/singbox-launcher/issues/69) / [#67](https://github.com/Leadaxe/singbox-launcher/pull/67). The parser-side `buildNaiveOutbound` was correctly populating `username` / `password` / `quic` / `quic_congestion_control` / `extra_headers` from the `naive+https://` / `naive+quic://` URI, but `GenerateNodeJSON` had no naive case-block — sing-box received a credential-less outbound and authentication failed silently. Adds the missing emit branch (mirroring vless / trojan / hysteria patterns); anonymous URIs (no userinfo) emit nothing extra, authenticated ones get all required fields. Thanks to [@hippus](https://github.com/hippus) for the report and the PR.

### Technical / Internal
-

## RU
### Основное

- **Фикс: NaïveProxy outbound теперь содержит аутентификационные данные.** [#69](https://github.com/Leadaxe/singbox-launcher/issues/69) / [#67](https://github.com/Leadaxe/singbox-launcher/pull/67). Парсер-сайд `buildNaiveOutbound` корректно вытаскивал `username` / `password` / `quic` / `quic_congestion_control` / `extra_headers` из URI `naive+https://` / `naive+quic://`, но в `GenerateNodeJSON` не было case-блока для naive — sing-box получал outbound без credentials и authentication молча проваливался. Добавлен emit-блок (по образцу vless / trojan / hysteria); anonymous URIs (без userinfo) ничего лишнего не эмитят, authenticated получают все нужные поля. Спасибо [@hippus](https://github.com/hippus) за отчёт и PR.

### Техническое / Внутреннее
-
