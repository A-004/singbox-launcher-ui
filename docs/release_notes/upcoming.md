# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

**Не добавлять** сюда мелкие правки **только UI** (порядок виджетов, выравнивание, стиль кнопок без смены действия и т.п.). Писать **новое поведение**: данные, форматы, сохранение, заметные для пользователя возможности.

## EN
### Highlights
- **Saved states switcher is safer.** The Core dashboard "Switch state…" dropdown now lists **● Current (active)** as the first item and shows it as the selected value, so a stray tap on the top of the list is a no-op instead of switching away from the live state. (Switching to a named state already asks first — Save current / Discard / Cancel — that confirm dialog is unchanged.)

### Technical / Internal
- `core_dashboard_tab.go`: `refreshStateSelector` prepends the localized `core.state_current_option` anchor and `selectCurrentStateSilently()` keeps it shown as selected (without firing OnChanged) on refresh and on confirm-dialog cancel; OnChanged treats the current/empty selection as a no-op.

## RU
### Основное
- **Переключатель сохранённых состояний стал безопаснее.** В дашборде Core выпадающий список «Сменить state…» теперь первым пунктом показывает **● Текущее (активно)** и отображает его как выбранное — случайный тап по верху списка ничего не делает, а не уводит с живого состояния. (Переключение на именованный state по-прежнему спрашивает — Сохранить текущее / Не сохранять / Отмена — этот модал не менялся.)

### Техническое / Внутреннее
- `core_dashboard_tab.go`: `refreshStateSelector` добавляет первым локализованный якорь `core.state_current_option`, а `selectCurrentStateSilently()` держит его выбранным (без триггера OnChanged) на refresh и на Cancel модала; OnChanged трактует выбор текущего/пустого как no-op.
