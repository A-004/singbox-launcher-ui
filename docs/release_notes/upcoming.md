# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

**Не добавлять** сюда мелкие правки **только UI** (порядок виджетов, выравнивание, стиль кнопок без смены действия и т.п.). Писать **новое поведение**: данные, форматы, сохранение, заметные для пользователя возможности.

## EN
### Highlights
- **Windows 7: stale TUN adapters cleaned automatically.** On launcher startup (when sing-box is not already running), accumulated `singbox-tun*` WinTun ghosts from prior sessions are removed — no manual Device Manager cleanup after upgrade. Also runs after each VPN Stop/Restart (SPEC 065).
- **Proxy-in inbound supports username/password authentication.** New Settings vars `Proxy-in require authentication` + `Proxy-in username` / `Proxy-in password`. When the toggle is off (default), the mixed inbound stays open (anonymous) as before. When on, `users: [{username, password}]` is emitted into the inbound — clients connecting to the local proxy must authenticate.
- **Template engine: `#if` construct for conditional field inclusion (SPEC 067).** Templates can now declaratively include or omit individual fields based on conditions, without Go post-substitute hooks. Supports an expression language with predicates (`#in`, `#not`, `#notEmpty`, `#isEmpty`, `#notIn`, `#matches`) and runtime globals `@platform` / `@arch` (mapping to `runtime.GOOS` / `runtime.GOARCH`). Two placement modes: map-spread (key inside object → fields merged on truthy condition) and array-element (single-key wrapper → element replaced or removed). See `docs/TEMPLATE_REFERENCE.md` §9 + `docs/CREATE_WIZARD_TEMPLATE.md`.
- **Breaking template format: outer `if`/`if_or` arrays require `@`-prefix on every var reference.** Bare `if: ["tun"]` is now a loader error — must be `if: ["@tun"]`. The bundled `bin/wizard_template.json` is migrated automatically; existing users will get the new template re-downloaded on the first launch via SPEC 046 invalidation (no user action needed). **Custom-template authors** must update their `if`/`if_or` arrays to `@`-form before upgrade. Also: `vars[].name` values `platform` and `arch` are now reserved (collision with runtime globals) — rename if used.

### Technical / Internal
- SPEC 065 follow-up: aggressive cleanup mode (prefix + Wintun only, no CM_PROB_PHANTOM gate) for taskkill stop path; startup hook `CleanupStaleTunAtStartUtil` in `main.go`.
- SPEC 067 implementation: `core/template/substitute.go` (+377 LOC) — `#if` walker with map-spread + array-element modes, 8-form predicate language (bare bool, equality, `#notEmpty`/`#isEmpty`, `#in`/`#notIn`, `#matches`, `#not`), `@platform`/`@arch` runtime globals; `template_validate.go` (+343 / -16) — load-time validation including reserved-name check + strict `@`-only outer if/if_or; 39+ new unit tests (`TestIf_*` + `TestOuterIf_*` + `TestVars_Reserved*`).
- `SubstituteVarsInJSON` signature changed: now takes `goos, goarch string` params (previously inferred from `runtime`). Internal callers updated.
- Bundled template migrated: 19 elements in `if`/`if_or` arrays prefixed with `@`; `inbounds[proxy-in]` wraps optional `users` field in `#if` keyed on `@proxy_in_auth_enabled`. Diff is purely additive plus the 19 prefix edits; no semantic regressions.
- Proxy-in auth: new template vars `proxy_in_auth_enabled` (bool), `proxy_in_username` (text), `proxy_in_password` (text); the last two visible only when `proxy_in_auth_enabled` is on (`vars[].if` cascade).

## RU
### Основное
- **Windows 7: старые TUN-адаптеры чистятся сами.** При запуске лаунчера (если sing-box ещё не работает) снимаются накопившиеся ghost `singbox-tun*` с прошлых сессий — после обновления не нужен ручной Device Manager. Плюс очистка после каждого Stop/Restart VPN (SPEC 065).
- **Proxy-in inbound поддерживает аутентификацию по логин/пароль.** В Settings появились `Proxy-in require authentication` + `Proxy-in username` / `Proxy-in password`. Выключено по умолчанию — mixed inbound остаётся открытым (anonymous) как раньше. Включено — в inbound эмитится `users: [{username, password}]`, клиенты должны аутентифицироваться.
- **Template engine: control-construct `#if` для условных полей (SPEC 067).** Шаблон теперь декларативно умеет включать/исключать отдельные поля по условию, без Go-хуков. Expression language с предикатами (`#in`, `#not`, `#notEmpty`, `#isEmpty`, `#notIn`, `#matches`) и runtime globals `@platform` / `@arch` (соответствуют `runtime.GOOS` / `runtime.GOARCH`). Два режима размещения: map-spread (ключ внутри объекта → поля мерджатся в parent при true) и array-element (single-key wrapper → элемент заменяется или удаляется). См. `docs/TEMPLATE_REFERENCE.md` §9 + `docs/CREATE_WIZARD_TEMPLATE_RU.md`.
- **Breaking template format: outer `if`/`if_or` массивы требуют `@`-префикс у каждого var-ref'а.** Голое `if: ["tun"]` теперь loader error — только `if: ["@tun"]`. Bundled `bin/wizard_template.json` мигрирован автоматически; существующим пользователям шаблон скачается заново на первом запуске через механизм инвалидации SPEC 046 (без действий юзера). **Авторам кастомных шаблонов** — обновить `if`/`if_or` на `@`-форму до апгрейда. Также: имена `platform` и `arch` в `vars[]` теперь зарезервированы (collision с runtime globals) — переименовать если использовались.

### Техническое / Внутреннее
- SPEC 065: aggressive cleanup (только префикс + Wintun), startup-хук `CleanupStaleTunAtStartUtil` в `main.go`.
- SPEC 067 реализация: `core/template/substitute.go` (+377 LOC) — `#if` walker с map-spread + array-element режимами, expression language из 8 форм (bare bool, equality, `#notEmpty`/`#isEmpty`, `#in`/`#notIn`, `#matches`, `#not`), runtime globals `@platform`/`@arch`; `template_validate.go` (+343 / -16) — load-time валидация включая reserved-name check + strict `@`-only outer if/if_or; 39+ новых unit-тестов (`TestIf_*` + `TestOuterIf_*` + `TestVars_Reserved*`).
- Signature `SubstituteVarsInJSON` изменился: теперь принимает `goos, goarch string` параметры (раньше брал из `runtime`). Internal callers обновлены.
- Bundled template мигрирован: 19 элементов в `if`/`if_or` массивах получили `@`-prefix; `inbounds[proxy-in]` оборачивает опциональное `users` поле в `#if` по `@proxy_in_auth_enabled`. Diff чисто аддитивный плюс 19 prefix-правок; semantic regression нет.
- Proxy-in auth: новые template vars `proxy_in_auth_enabled` (bool), `proxy_in_username` (text), `proxy_in_password` (text); последние два видны только когда `proxy_in_auth_enabled` включён (`vars[].if` каскад).
