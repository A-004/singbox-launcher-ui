# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

**Не добавлять** сюда мелкие правки **только UI** (порядок виджетов, выравнивание, стиль кнопок без смены действия и т.п.). Писать **новое поведение**: данные, форматы, сохранение, заметные для пользователя возможности.

## EN
### Highlights
-

### Technical / Internal
- **SPEC 053 preset bundles — shipped end-to-end.** Self-contained parametrized rule presets work top-to-bottom: `template.presets[]` with typed `vars` (outbound / dns_server / enum / text / number / bool), local-tag-scoped `rule_set` / `dns_servers`, conditional fragments via `if` / `if_or`, expansion engine, build-pipeline integration (`MergePresetsIntoRoute` / `MergePresetsIntoDNS` as additive pass over legacy `MergeRouteSection` / `MergeDNSSection` — old code untouched), state v6 schema (`presets_v1`) with auto-migration from v5 (idempotent backup as `state.json.v5.bak`), Library dialog with Presets section, Rules tab tiles, dedicated edit dialog with universal var-form rendering, **Convert to user rule** one-way conversion button, DNS tab template-server override handler syncing into `state.DNSV6.TemplateServers`. Six ready-to-use presets shipped in `bin/wizard_template.json` (`private-ips-direct`, `local-lan-domains`, `bittorrent-direct`, `block-ads`, `ru-direct-preset`, `ru-inside`) covering all preset forms. State.json format auto-switches between v5 and v6 based on presence of preset-refs — pure inline/srs users stay on v5 untouched. 91 new unit tests, all 23 packages green.

## RU
### Основное
-

### Техническое / Внутреннее
- **SPEC 053 preset bundles — выкачен полностью.** Self-contained параметризованные пресеты правил работают end-to-end: `template.presets[]` с типизированными `vars` (outbound / dns_server / enum / text / number / bool), локально-scoped `rule_set` / `dns_servers`, условные фрагменты через `if` / `if_or`, expansion engine, **интеграция в основной build pipeline** через `MergePresetsIntoRoute` / `MergePresetsIntoDNS` (дополнительный pass поверх legacy `MergeRouteSection` / `MergeDNSSection` — старый код не трогается), state v6 схема (`presets_v1`) с авто-миграцией с v5 (идемпотентный backup `state.json.v5.bak`), UI Library dialog с секцией Presets, tile'ы preset-ref'ов в Rules tab, dedicated edit dialog с универсальным rendering формы, **кнопка Convert to user rule** для one-way конверсии, DNS tab template-server override handler синхронизируется в `state.DNSV6.TemplateServers`. В `bin/wizard_template.json` добавлено 6 ready-to-use preset'ов (`private-ips-direct`, `local-lan-domains`, `bittorrent-direct`, `block-ads`, `ru-direct-preset`, `ru-inside`) покрывающих все формы. Формат state.json авто-переключается между v5 и v6 по наличию preset-ref'ов — юзеры с чистыми inline/srs правилами остаются на v5 без изменений. 91 новый unit-тест, все 23 пакета зелёные.
