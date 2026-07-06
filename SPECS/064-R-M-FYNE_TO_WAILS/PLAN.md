# PLAN — Миграция UI с Fyne на Wails

> **ЭТА ТЕМА НЕ АКТИВНА** — ждёт команды пользователя.
> Папка `webui/` оставлена как прототип-концепт.
> Fyne-приложение работает как обычно, никаких изменений в main.go нет.

---

## 0. Предварительные условия

### 0.1 Системные требования для старта

- [ ] Установлен Go 1.25+
- [ ] Установлен Node.js 20+ (для сборки фронтенда)
- [ ] Установлен Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- [ ] На Windows: установлен WebView2 (обычно уже есть в Win10/11)

### 0.2 Структура Wails-проекта

После инициализации Wails-проект будет вложен в текущий репозиторий:

```
singbox-launcher-betterui/
├── main.go              ← переписывается (точка входа Wails вместо Fyne)
├── app.go               ← Wails-приложение (новый файл)
├── wails.json           ← конфиг Wails
├── core/                ← без изменений
├── api/                 ← без изменений
├── internal/            ← без изменений (кроме удаления Fyne-пакетов)
├── frontend/            ← новая папка
│   ├── src/
│   ├── index.html
│   ├── package.json
│   ├── wailsjs/         ← авто-генерация TypeScript binding'ов
│   └── ...
└── build/               ← стандартная папка Wails для иконок/ресурсов
```

---

## 1. Этапы миграции

### Этап 0: Инициализация Wails

**Что делаем:**
1. `wails init` внутри репозитория (или ручная настройка)
2. Настроить `wails.json` (имя приложения, окно, иконка)
3. Настроить `main.go` для запуска через Wails вместо Fyne
4. Перенести иконки (app.ico, on.ico, off.ico) в `build/`
5. Проверить что Wails-заглушка собирается и запускается

**Что должно работать:**
- Пустое окно с WebView (просто "Hello World" на HTML)
- Системный трей (пустое меню)
- Закрытие окна → сворачивание в трей

**Файлы для изменения:**
- `main.go` — замена Fyne на Wails
- Создать `app.go` — структура Wails-приложения
- Создать `wails.json`
- Создать `frontend/` (базовый HTML)

---

### Этап 1: Дашборд (VPN status + proxy list)

**Что делаем:**
1. Создать `frontend/src/dashboard/` — HTML + CSS + JS
2. Написать Wails-биндинги:
   - `GetVPNStatus() → {running, coreVersion, proxyCount, selected, proxies[]}`
   - `ToggleVPN() → {ok}`
   - `SelectProxy(name string) → {ok}`
3. Подключить уже готовые HTML+CSS+JS из `webui/`
4. Настроить автообновление через `EventsOn` (вместо HTTP-полинга)

**Что должно работать:**
- Красивый дашборд как в прототипе `webui/`
- VPN вкл/выкл
- Список прокси с задержками
- Выбор прокси
- Автообновление статуса

**Готовый код (из webui/):**
- `webui/dashboard.html` → `frontend/src/index.html`
- `webui/dashboard.css` → `frontend/src/style.css`
- `webui/dashboard.js` → адаптировать под Wails Events

**Примечание:** бэкенд-логика (статус VPN, прокси-лист) уже есть в `core/controller.go` (методы `GetProxiesList`, `GetActiveProxyName`, `RunningState.IsRunning`). Wails-биндинги будут просто обёртками над ними.

---

### Этап 2: Core Dashboard

**Что делаем:**
1. Wails-биндинги:
   - `GetCoreVersion() → string`
   - `GetLatestCoreVersion() → string`
   - `GetLatestLauncherVersion() → string`
   - `DownloadCore(version string) → progress chan DownloadProgress`
   - `DownloadTemplate() → error`
   - `GetArchivedCoreVersions() → []string`
   - `SwitchToArchivedCoreVersion(version string) → error`
   - `GetInstalledCoreVersion() → string`

2. HTML-вкладка:
   - Текущая версия sing-box
   - Кнопка "Check for updates"
   - Прогресс скачивания (progress bar)
   - Кнопка "Download Template"
   - Список архивных версий с возможностью отката

**Соответствующие методы в коде:**
- `core/core_version.go` — `GetInstalledCoreVersion`, `GetLatestCoreVersion`, `GetLatestLauncherVersion`
- `core/core_downloader.go` — `DownloadCore`, `GetArchivedCoreVersions`, `SwitchToArchivedCoreVersion`
- `core/template_migration.go` — `InvalidateTemplateIfStale`

---

### Этап 3: Clash API Tab (Servers)

**Что делаем:**
1. Wails-биндинги:
   - `GetProxies() → []ProxyInfo`
   - `GetProxiesInGroup(groupName) → {proxies[], now}`
   - `SwitchProxy(group, name) → error`
   - `PingAllProxies() → error`
   - `ResetClashHTTPTransport()`
   - `GetClashAPIConfig() → {baseURL, token, enabled}`

2. HTML-вкладка:
   - Полная таблица прокси (имя, тип, задержка, трафик up/down)
   - Кнопки: Test All, Select Proxy
   - Auto-ping after connect (если включено)
   - Статус подключения к Clash API

**Бэкенд:** `api/clash_proxy.go`, `api/clash_delay.go`, `api/clash_switch.go`, `core/services/api_service.go`

---

### Этап 4: Diagnostics

**Что делаем:**
1. Wails-биндинги:
   - `GetLogFileContent(logName) → string` (tail -100)
   - `StartDebugAPI(port, token) → error`
   - `StopDebugAPI()`
   - `GetDebugAPIAddr() → string`
   - `GetCoreBinaryPath() → string`
   - `CheckWintunDLL() → bool`
   - `GetTrafficProfilerStatus() → {}`

2. HTML-вкладка:
   - Viewer логов (main log, child log, API log)
   - Debug API (вкл/выкл, копировать URL)
   - Информация о бинарниках
   - Wintun DLL (Windows)
   - Кнопка "Open Traffic Profiler"

**Бэкенд:** `core/debugapi_wiring.go`, `core/error_handler.go`, `internal/traffic/monitor.go`

---

### Этап 5: Configurator (Wizard) — САМЫЙ БОЛЬШОЙ

**Что делаем:**
Wizard состоит из 4 вкладок, каждая — отдельный SPA-компонент:

#### 5a. Sources (подписки)
- Wails-биндинги: CRUD для источников подписок (`StateService`)
- HTML: список + модальное окно редактирования
- Те же поля что и в Fyne: URL, tag prefix/suffix, skip/mask, transport

#### 5b. Outbounds (глобальные outbound'ы)
- Wails-биндинги: чтение/запись `Connections.Outbounds`, генерация preset'ов
- HTML: дерево с Drag&Drop (сортировка)

#### 5c. Rules (правила)
- Wails-биндинги: `GetRules()`, `AddRule()`, `EditRule()`, `DeleteRule()`, `ReorderRules()`
- HTML: список правил с tipoм (preset/inline/srs), редактирование в модалке

#### 5d. DNS
- Wails-биндинги: `GetDNSServers()`, `GetDNSRules()`, `AddDNSServer()`, `AddDNSRule()`
- HTML: два списка (серверы + правила)

**Ключевые файлы бэкенда:**
- `core/state/` — модель состояния
- `ui/configurator/business/` — бизнес-логика (WizardModel, реконсиляция)
- `ui/configurator/presentation/` — презентация (переписать под Wails-биндинги)

---

### Этап 6: Traffic Profiler

**Что делаем:**
1. Wails-биндинги:
   - `GetTrafficData() → {sessions[], current}`
   - `StartTrafficRecording(sessionName) → error`
   - `StopTrafficRecording()`

2. HTML:
   - Canvas-графики (Chart.js или vanilla Canvas 2D)
   - Сессии записи трафика

**Бэкенд:** `internal/traffic/monitor.go`, `internal/traffic/config.go`, `internal/traffic/widget.go` (логику сохранить, UI выпилить)

---

### Этап 7: Settings

**Что делаем:**
1. Wails-биндинги:
   - `GetSettings() → Settings`
   - `SaveSettings(settings) → error`
   - `SetLang(lang)`
   - `SetPingTestURL(url)`
   - `ToggleAutoUpdate(enabled)`
   - `ToggleAutoPing(enabled)`

2. HTML:
   - Язык, тема (пока тёмная), Ping Test URL
   - Auto-update, Auto-ping toggles
   - Subscription settings

---

### Этап 8: Трей-иконка (systray)

**Что делаем:**
1. Настроить Wails-трей:
   - Иконки (on/off/grey)
   - Меню: Start VPN, Stop VPN, Select Proxy, Open, Quit
   - Обновление меню при изменении прокси

2. Бэкенд-логика:
   - `core/tray_menu.go` — перенести в Wails-биндинги (метод `CreateTrayMenu` → Wails Menus)
   - Переписать `UpdateTrayMenuFunc` на Wails Events

---

### Этап 9: Удаление Fyne-кода

**После полной проверки feature parity:**

1. Удалить папки:
   - `ui/` (весь)
   - `internal/apptheme/`
   - `internal/fynewidget/`
   - `internal/dialogs/`
   - `core/uiservice/`

2. Удалить из `go.mod`:
   - `fyne.io/fyne/v2`
   - `github.com/dweymouth/fyne-tooltip`
   - `fyne.io/systray`
   - `github.com/go-gl/gl*`
   - `github.com/fyne-io/*`
   - `github.com/nicksnyder/go-i18n/v2` (если не используется вне UI)

3. Обновить `go.mod`: `go mod tidy`

4. Удалить `app.manifest` (Fyne-Windows specific)

---

### Этап 10: Финальное тестирование

**Проверить на всех платформах:**
- Windows 10/11
- macOS (Intel + Apple Silicon)
- Linux (Ubuntu/Debian/Fedora)

**Чеклист:**
- [ ] Сборка без ошибок (no OpenGL dependency)
- [ ] Запуск без ошибок
- [ ] VPN start/stop работает
- [ ] Прокси-лист загружается
- [ ] Выбор прокси работает
- [ ] Пинг всех/одного прокси
- [ ] Скачивание ядра
- [ ] Скачивание шаблона
- [ ] Debug API вкл/выкл
- [ ] Трей-иконка работает (все 3 состояния)
- [ ] Трей-меню обновляется
- [ ] Сворачивание в трей при закрытии окна
- [ ] Configurator: Sources CRUD
- [ ] Configurator: Rules CRUD + реордер
- [ ] Configurator: DNS серверы + правила
- [ ] Configurator: Outbounds + presets
- [ ] Сохранение состояния (state.json)
- [ ] Автообновление подписок
- [ ] Traffic Profiler: запись + графики
- [ ] Настройки (язык, тема)
- [ ] Sleep/Resume (Windows)
- [ ] macOS Dock icon click

---

## 2. Технические решения

### 2.1 Коммуникация Go ↔ JS

```go
// Go-side binding
func (a *App) GetVPNStatus() webui.StatusResponse {
    return webui.StatusResponse{
        VPNRunning:  a.ctrl.RunningState.IsRunning(),
        CoreVersion: getVersion(a.ctrl),
        ProxyCount:  len(a.ctrl.APIService.GetProxiesList()),
        Selected:    a.ctrl.GetActiveProxyName(),
        Proxies:     buildProxyItems(a.ctrl),
    }
}
```

```javascript
// JS-side
window.runtime.Call('GetVPNStatus').then(data => {
    renderDashboard(data);
});

// Events (push from Go)
window.runtime.EventsOn('vpnStatusChanged', data => {
    updateVPNStatus(data.running);
});
```

### 2.2 Тема (CSS)

Оставляем тёмную тему из прототипа (`webui/dashboard.css`). CSS Variables позволяют легко добавить светлую тему позже:

```css
:root {
  --bg-primary: #0a0a0b;
  --text-primary: #f5f5f7;
  --accent-green: #30d158;
  /* ... */
}
```

### 2.3 Reactivity

Для простых экранов (дашборд, status, core dashboard) — vanilla JS.
Для configurator (сложные формы, списки, редактирование) — Petite-Vue (6KB gzip).

### 2.4 Сборка фронтенда

Wails использует esbuild по умолчанию. `frontend/package.json`:
```json
{
  "name": "singbox-launcher",
  "scripts": {
    "dev": "wails dev",
    "build": "wails build"
  },
  "dependencies": {
    "petite-vue": "^0.4.1"
  }
}
```

---

## 3. Критические точки (checkpoints)

Каждый этап должен заканчиваться работающим билдом. Никаких "сломано, починим потом".

| Checkpoint | Что должно работать |
|------------|-------------------|
| После 0 | Пустое Wails-окно + трей |
| После 1 | Дашборд (как webui-прототип) |
| После 2 | Core Dashboard (скачивание, версии) |
| После 3 | Clash API tab (прокси, пинг) |
| После 4 | Diagnostics (логи, debug API) |
| После 5 | Полный Wizard (все 4 таба) |
| После 6 | Traffic Profiler (графики) |
| После 7 | Settings (настройки) |
| После 8 | Трей-иконка |
| После 9 | Всё работает, Fyne выпилен |
| После 10 | Протестировано на всех платформах |

---

## 4. Что НЕ делается

- **Не рефакторим бэкенд.** Меняем только UI слой.
- **Не оптимизируем парсеры/генераторы конфигов.** Они и так отлично работают.
- **Не добавляем новые фичи.** Только feature parity.
- **Не переписываем на React/Vue/Svelte** (пока). Vanilla JS + Petite-Vue достаточно.
- **Не трогаем `internal/locale`** — оставляем как есть на Go-стороне.

---

## 5. После миграции (roadmap)

После удаления Fyne и стабилизации Wails:

- [ ] Светлая тема (CSS Variables + переключатель)
- [ ] Анимации переходов между вкладками
- [ ] Native Window frame или frameless с кастомным заголовком
- [ ] Горячие клавиши (Ctrl+P пинг, Ctrl+W закрыть)
- [ ] Сворачивание в трей при закрытии (как в Telegram)