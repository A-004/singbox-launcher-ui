# SPEC 059-F-N — TRAFFIC_PROFILER

**Status:** New (N)
**Type:** Feature (F)
**Inspired by:** LxBox §044 «Per-app traffic profiler» (mobile counterpart, v1.7.0 / v1.8.0).
**Не меняет:** sing-box config.json format; state.json schema.

---

## Цель

Дать на desktop инструмент realtime-диагностики: «куда какой process содиняется, через какие домены/IP/порты, сколько байт передал и как давно открыто соединение». Чтобы ответить на вопросы вида:

- «Почему Slack/Telegram/браузер ходит мимо VPN?» — увидеть какой outbound выбрал router для конкретного процесса
- «Куда стучит этот закрытый приложение?» — privacy-аудит
- «Почему X не открывается через VPN» — увидеть DNS chain, CNAME-таргет, через какой outbound идёт
- «Что вообще сейчас происходит на сетевом уровне?» — discovery / overview mode

На LxBox такой инструмент уже есть и оказался основным diagnostic-tool, ускоряющим debug VPN routing'а в 30–50 раз. Переносим концепт на desktop.

## Скоп (что делаем)

**Отдельное окно «Traffic Profiler»**, запускаемое кнопкой с вкладки **Diagnostics**. Singleton — повторный клик focus'ит уже открытое окно. Внутри окна два view'а:

1. **Live** (system-wide, discovery mode) — стрим всех DNS/TCP/UDP событий sing-box'а в realtime. Фильтрация: kind, search, process name. Без явного recording — окно открыто = видишь поток.

2. **Per-process recording** — pick process → ▶ START → видишь только его трафик с агрегатами по доменам/IP/connections. ⏹ STOP финализирует session в ring-buffer последних 5.

Каждый event обогащён: process_path (если sing-box детектит owner'а через `find_process`), domain (если DNS / SNI sniffed), CNAME chain, ip, port, outbound chain, bytes ↑↓, duration.

**Почему отдельное окно, а не tab:**

- Диагностика — secondary workflow. Юзер открывает на 5-10 минут когда что-то ломается, а не держит постоянно.
- Окно можно расположить рядом с проблемным приложением (например, Slack слева, Traffic справа) — tab внутри Configurator'а такого не даст.
- Configurator wizard ещё может быть открыт параллельно — Traffic не конкурирует за tab-bar real estate.
- Закрытие окна = stop live capture (см. §"Lifecycle"), не зависит от закрытия app'а.

## Что НЕ делаем (out of scope MVP)

- HTTP-уровневую инспекцию (URL/headers/method) — sing-box работает на L4, не L7
- Packet capture / pcap — у нас есть routing engine, этого достаточно
- Differential mode (compare session A vs B) — на post-MVP
- Block / Add-to-rule inline actions из profiler view — на post-MVP
- TLS fingerprinting (JA3/JA4) — sing-box capability ещё не expose'ит
- Persistence sessions через restart — in-memory only (как в LxBox)

---

## Адаптация LxBox §044 под desktop

| Аспект | LxBox (mobile/Android) | Desktop singbox-launcher | Решение |
|---|---|---|---|
| Process keying | Package name `ru.tinkoff.investing` | Executable path `/Applications/Slack.app/.../Slack` | Используем canonical executable path; UI display name = basename или Info.plist `CFBundleName` (mac) / VERSIONINFO `FileDescription` (win) / `.desktop` (linux) |
| Process detection | sing-box `find_process: true` + UID resolver | `find_process: true` работает идентично — отдаёт `metadata.processPath` | Шаблон по умолчанию должен иметь `route.find_process: true` (см. §Pre-requisites) |
| Secondary apps (Android WebView) | Multi-pick secondary packages | Не нужно — каждый desktop process отдельный | Single-pick per session |
| App picker icons | Android PackageManager + AppInfoCache | macOS NSWorkspace `iconForFile`; win `SHGetFileInfo`; linux `.desktop`/freedesktop icon lookup | Best-effort через platform code; fallback на generic file icon |
| Log stream source | In-app `ClashLogPump` (Flutter notifier) | sing-box.log file (Go process) | Tail `bin/logs/sing-box.log` через `fsnotify` (rename-rotation safe) или периодический `os.Open + Seek + ReadFrom` |
| Live event push | Server-Sent Events (Flutter http) | Go channel + Fyne `fyne.Do()` UI thread schedule | `chan TrafficEvent` буфер N=256, goroutine drainer → UI |
| Verbose toggle | Setting `vars.log_level=debug` через Debug API + reload | Edit `vars.log_level` через configurator + reload sing-box | Кнопка «🔬 Debug DNS» в Traffic tab toolbar — на ON делает `vars[log_level] = debug` + рестарт; на OFF revert |
| Recording indicator | ⚡ chip на HomeScreen + Stats tab title | ⚡ badge на кнопке «Traffic Profiler» в Diagnostics tab когда recording active; ⚡ в title окна когда оно открыто | Update button label + window title через TrafficProfiler listener |
| Connection close trigger | Clash API DELETE `/connections/<id>` (есть в LxBox) | Не в MVP скоупе (можно добавить как secondary feature) | Post-MVP |
| Polling cadence | Clash `/connections` poll 1s + log-stream | Идентично — `/connections` poll 1s + log tail | Same |

## Pre-requisites

1. **`route.find_process: true`** в дефолтном wizard_template.json. Если выключено — Traffic tab показывает баннер «Process detection disabled in template — install / update template to enable». На macOS sing-box запускается с правами, на Windows — admin: в обоих случаях `find_process` доступен.

2. **`experimental.clash_api.external_controller`** должен быть настроен (уже есть в template — `127.0.0.1:9090` + secret). Используется как сегодня для `/proxies`, добавляется new endpoint usage `/connections`.

3. **`log.level=info`** минимум; для DNS chain детектирования нужен `debug`. Без debug DNS-уровневые события могут быть неполные — Traffic tab показывает баннер «Switch to debug logs for full DNS visibility» с кнопкой включения (см. Verbose toggle).

---

## UX flow

### Где живёт

**Кнопка «Traffic Profiler»** в Diagnostics tab (рядом с «Open log», «Kill sing-box» и прочими). Клик открывает singleton окно «Traffic Profiler» (separate `fyne.Window`). Повторный клик focus'ит уже открытое окно.

Окно живёт независимо от Configurator wizard, может располагаться рядом с проблемным приложением (Slack слева, Traffic справа). Внутри окна — два view'а в tab-bar'е: Live + Per-process.

Recording indicator: пока session active — кнопка «Traffic Profiler» в Diagnostics tab показывается как **«Traffic Profiler ⚡»**, и title окна — **«Traffic Profiler ⚡ Recording · 02:34»**.

### Diagnostics tab — точка входа

```
┌─ Diagnostics ─────────────────────────────────────────────────────────┐
│  Logs                                                                  │
│  [Open launcher log]  [Open sing-box log]  [Open config]              │
│                                                                         │
│  Process                                                               │
│  [Kill sing-box (privileged)]                                          │
│                                                                         │
│  Diagnostics                                                           │
│  [Traffic Profiler ⚡]  ← открывает отдельное окно                     │
│  [Run network test]   [STUN check]                                     │
└────────────────────────────────────────────────────────────────────────┘
```

⚡ badge добавляется к label кнопки когда `profiler.ActiveSession() != nil` — юзер видит recording state даже если Traffic окно скрыто за другими.

### Окно — idle state (no recording)

```
┌─ Traffic Profiler ────────────────────────────────────────────────[—□×]┐
│  [Live]  [Per-process]                                                 │
│                                                                         │
│  ─── Live (system-wide, newest first) ──────────────────────── 🔬 dbg ─│
│  [Search…]  [DNS] [DNS×] [TCP] [TCP·] [UDP]  [Filter by process ▾]    │
│                                                                         │
│  12:34:15  DNS  cdn.t-bank-app.ru → 193.17.93.194                     │
│              Slack.app                                                  │
│  12:34:14  DNS  certs.t-bank-app.ru → 81.222.127.186  ⚠               │
│              Telegram.app          → via vpn-1 → 🇫🇮 Финляндия         │
│  12:34:12  TCP  api.example.com:443                                    │
│              Spotify.app           ↑ 458 B  ↓ 2.1 KB  open 3.2s        │
│  ...                                                                    │
└────────────────────────────────────────────────────────────────────────┘
```

- **Live tab** — discovery mode, видишь поток сразу как открыл окно. Не нужно ничего записывать
- **Per-process tab** — pick process → ▶ START
- Чекбоксы фильтрации kind (multi-select); search-поле — substring match по domain/IP/process; «Filter by process» — bottom panel с checkbox per замеченный process

### Окно — per-process recording

```
┌─ Traffic Profiler ⚡ Recording · 02:34 ────────────────────────[—□×]┐
│  [Live]  [Per-process]                                                 │
│                                                                         │
│  Target: [Slack.app ▾]                       [⏹ STOP]      🔬 dbg     │
│                                                                         │
│  ⏺ Recording · 02:34 · 47 doms · 53 ips · 287 ev                      │
│                                                                         │
│  [Live] [Domains] [IPs] [Connections]                                  │
│                                                                         │
│  ─── Live ─────────────────────────────────────────                   │
│  10:42:15  DNS  cdn.t-bank-app.ru → 193.17.93.194                     │
│              ↳ CNAME cl-ead2c819.edgecdn.ru                            │
│  10:42:15  TCP  cdn.t-bank-app.ru:443                                  │
│              ↳ via direct-out                                          │
│  10:42:14  DNS  certs.t-bank-app.ru → 81.222.127.186 ↗  ⚠            │
│              ↳ CNAME eq09pc7nbi.a.trbcdn.net                          │
│  10:42:14  TCP  certs.t-bank-app.ru:443                            ⚠ │
│              ↳ via vpn-1 → 🇫🇮 Финляндия                              │
│  ...                                                                    │
│                                                                         │
│  ─── Saved sessions ──── (when recording = idle)                       │
│  • Slack.app — 02:34 · 47 doms                  [Open] [Share] [Del]  │
│  • Telegram.app — 00:48 · 12 doms               [Open] [Share] [Del]  │
└────────────────────────────────────────────────────────────────────────┘
```

### Sub-tabs (Per-process mode)

| Sub-tab | Содержимое |
|---|---|
| **Live** | Stream newest-first. Event = ts + kind label + summary + bytes + duration + ⚠ icon if issue. Color-coded kind labels: DNS (tertiary), DNS× (error red), TCP (primary), TCP· (closed, dimmed), UDP (secondary) |
| **Domains** | Aggregated unique domains, sorted by total bytes. Search matches domain / IP / CNAME target. Tap row → expand: CNAME chain, all resolved IPs, outbound chain, first/last seen, ⚠ issues |
| **IPs** | Aggregated unique destination IPs, sorted by bytes. Useful для hostless connections (raw TCP без SNI sniff). Tap IP `↗` → переход на Domains tab с auto-fill search этим IP (cross-IP audit) |
| **Connections** | Timeline per-connection (TCP/UDP open/close). Tap row → expand: CNAME chain, all IPs, sing-box rule that matched, outbound, ⚠ issues |

### Recording toolbar

| Кнопка | Действие |
|---|---|
| `[⏹ STOP]` / `[▶ START]` | Start/stop recording session. Process picker заблокирован пока active |
| `🔬 dbg` | Toggle verbose: set `vars[log_level]=debug` + reload sing-box. На OFF revert. Banner «Verbose logs active — battery/CPU impact» внутри tab'а пока ON |
| `[⋮ Overflow]` | Copy session JSON, Export to file, Clear all sessions, Help |

### Pre-session backfill

Profiler service always holds rolling buffer 60s × ~3000 events (все process'ы, не только target). На `▶ START` — события за last 60s, которые match target process, копируются в session с marker `〽 backfilled from pre-recording`. Решает «юзер видит проблему и только потом нажимает Record — теряет первые секунды».

### Saved sessions

Когда session active нет — внизу Per-process tab показываются последние 5 завершённых. Ring-buffer FIFO. Tap → открыть session в read-only режиме (те же 4 sub-tab'а). Force-stop приложения = все sessions стираются (in-memory only).

---

## Confidence levels (perевод из LxBox §048)

Каждое event имеет confidence — насколько точно мы уверены что это traffic нашего target process'а:

| Level | UI marker | Когда |
|---|---|---|
| `verified` | (no marker) | sing-box лог `router: found process name: <target_path>` явный match |
| `inferred` | 〽 | TCP без `find_process`-match, но к IP, который был resolved через DNS query, attribute'нутый к этому target'у (10s window) |
| `unattributed` | ? (dimmed) | никакая strategy не сработала, событие показано как nearby (только в Live system-wide tab'е) |

Tooltip над badge'ом показывает `matched_via` (как сработала attribution) — debug-info.

---

## Connection issues (⚠ маркеры)

Не статистические аномалии, а конкретные diagnostic-сигналы. Два locale-агностичных типа (как в LxBox после §048 cleanup):

| Issue | Условие | Use case |
|---|---|---|
| **DNS timeout** | sing-box лог `dns: exchange failed for <host>: <reason>` (context deadline exceeded и пр.) | Network-level problem, DNS server недоступен |
| **TCP RST early** | TCP conn closed в течение 1s, ↑0 ↓0 байт | Firewall RST / TLS handshake fail / block / unreachable |

Отвергнуто (LxBox прошёл этот путь и выпилил):
- ~~`geoMismatch`~~ — RU-bias via emoji parsing в outbound name; правильная реализация требует user-config home-locale; post-MVP
- ~~`unusualPort`~~ — arbitrary whitelist, шум для torrent / steam / corp
- ~~`badLatency`~~ — dead code на mobile, не нужен

В JSON session events: `events[i].issues: [{kind, description}]`.

---

## Архитектура

### Сервис `TrafficProfiler` (новый, Go)

`internal/traffic/profiler.go` — singleton, lifetime app'а.

```go
type TrafficProfiler struct {
    mu          sync.Mutex
    rollingBuf  *ringBuffer  // last 60s × ~3000 events (system-wide)
    active      *Session     // current recording or nil
    completed   []*Session   // ring max 5
    listeners   []chan<- TrafficEvent  // UI subscribers
    
    clashAPI    *clashClient
    logTail     *logTailReader
    stopCh      chan struct{}
}

func (p *TrafficProfiler) StartSession(processPath string, verbose bool) (*Session, error)
func (p *TrafficProfiler) StopSession() (*Session, error)
func (p *TrafficProfiler) ActiveSession() *Session
func (p *TrafficProfiler) CompletedSessions() []*Session
func (p *TrafficProfiler) DeleteSession(id string)
func (p *TrafficProfiler) ClearAll()

func (p *TrafficProfiler) Subscribe() (<-chan TrafficEvent, func())  // returns unsub func
func (p *TrafficProfiler) Snapshot(lastN time.Duration) []TrafficEvent  // for late-join UI
```

Singleton живёт пока приложение запущено. На startup app'а: подключается к Clash API, начинает log tail, начинает rolling buffer. Идёт ровно один tailer лог-файла и одна 1-сек poll-loop `/connections`.

### Data model

```go
type Session struct {
    ID            string
    TargetProcess string         // canonical executable path
    StartedAt     time.Time
    FinishedAt    *time.Time
    WasVerbose    bool           // log.level was bumped to debug
    Events        []TrafficEvent // ring 50000 / 3h sliding window
    // aggregated views computed on-demand:
    domainCache   map[string]*DomainStats
    ipCache       map[string]*IpStats
    connCache     []*ConnRecord
}

type TrafficEvent struct {
    TS             time.Time
    Kind           EventKind        // DNSResolve / DNSFail / TCPOpen / TCPClose / UDPOpen
    ProcessPath    string           // empty if unattributed
    Confidence     Confidence       // verified / inferred / unattributed
    MatchedVia     string           // "router_log" / "prior_dns_10s" / ""
    Domain         string           // sniffed/resolved hostname (empty if hostless)
    CnameChain     []string         // [t-bank-app.ru, edgecdn.ru, ...]
    IP             string
    Port           int
    OutboundChain  []string         // [vpn-1, 🇫🇮 Финляндия, vless-server]
    UpBytes        int64
    DownBytes      int64
    Duration       time.Duration    // for TCPClose
    Issues         []ConnectionIssue
    RawLogLine     string           // debug only
}

type ConnectionIssue struct {
    Kind        IssueKind  // DnsTimeout / TcpRstEarly
    Description string
}

type DomainStats struct {
    Domain      string
    Connections int
    UpBytes     int64
    DownBytes   int64
    FirstSeen   time.Time
    LastSeen    time.Time
    IPs         map[string]struct{}
    Outbounds   map[string]struct{}
    Issues      []ConnectionIssue
    CnameChain  []string  // first observed chain
}
```

### Data sources

**1. Clash API `/connections` (poll 1s)**

```
GET http://127.0.0.1:9090/connections
Authorization: Bearer <secret>

Response:
{
  "downloadTotal": 12345, "uploadTotal": 6789,
  "connections": [
    {
      "id": "<uuid>",
      "metadata": {
        "network": "tcp",
        "type": "TLS",
        "host": "api.example.com",
        "destinationIP": "1.2.3.4",
        "destinationPort": "443",
        "processPath": "/Applications/Slack.app/Contents/MacOS/Slack",
        "process": "Slack"
      },
      "upload": 458, "download": 2058,
      "start": "2026-05-24T12:34:00Z",
      "chains": ["vless-server", "🇫🇮 Финляндия", "vpn-1"],
      "rule": "domain_suffix",
      "rulePayload": "example.com"
    }
  ]
}
```

Diff с предыдущим snapshot:
- Новый id → emit `TCPOpen` / `UDPOpen`
- Существующий id → update bytes (without emit if no kind-change)
- Исчез id → emit `TCPClose` / `UDPClose` с duration = now - start

**2. sing-box.log tail (fsnotify)**

Tail `bin/logs/sing-box.log`. Парсим regex'ами:

```
[id] dns: exchanged|cached <type> <domain>. -> <ip>|<cname>.
[id] dns: exchange failed for <host>: <reason>
[id] router: found process name: <process_path>
[id] router: match[<rule>] => route(<outbound>)
[id] inbound/tun[<id>]: outbound connection to <host>:<port>
```

Patterns могут разойтись между minor sing-box releases — log format не stable. Регекс'ы вынесены в `internal/traffic/parser.go` с unit-тестами на zoo of log samples (как в LxBox).

`fsnotify` подписка следит за rotate / truncate. На rotate — reopen.

**3. Cross-source join**

Conn-ID — это ключ. Sing-box логирует `[<conn_id>]` префиксом в большинстве строк. Clash API возвращает `id` тот же. Profiler:
- На DNS строку — кладёт в `_dnsAccumulator[conn_id]`
- На `router: found process name` — кладёт в `_connProcessMap[conn_id]`
- На Clash poll: matches active conn ↔ rolling buf events

CNAME chain reconstruction: первое `dns: exchanged` для conn_id — это domain юзера; последующие CNAME-targets копятся в chain, A-record IP terminates. Без этого finder в Domains tab показывал бы CDN-домен, а не оригинальный (LxBox §044 impl bug #4).

### Verbose toggle

Toggle ON:
1. Read current `vars[log_level]`, save → `_savedLogLevel`
2. Set `vars[log_level]=debug` через configurator
3. `core.ReloadSingBox()` — light reload (без TUN teardown)
4. Banner shows
5. Profiler начинает capture'ить debug-уровень events

Toggle OFF: revert log_level → reload → banner hides.

⚠ Reload разрывает active TCP-соединения. UI диалог-warning при первом toggle: «Reloading sing-box — active connections will reset. Continue?».

---

## UI компоненты

Новые widgets (в `ui/traffic/`):

- `TrafficWindow` — singleton `fyne.Window` с двумя view'ами (Live / Per-process) внутри `container.AppTabs`
- `liveView` — system-wide stream + фильтры
- `perProcessView` — target picker + ▶/⏹ + 4 sub-tab'а (Live/Domains/IPs/Connections)
- `processPickerDialog` — список бегущих process'ов с executable path + icon (через platform code)
- `_IssueChip` — ⚠ visual marker
- `_VerboseLogsBanner` — banner внутри окна сверху

**Точка входа** в `ui/diagnostics_tab.go`: button «Traffic Profiler» в существующей секции tab'а (рядом с Kill sing-box etc). Tooltip объясняет назначение.

Window registration через `UIService` (или подобный singleton manager):

```go
type UIService struct {
    ...
    TrafficWindow fyne.Window  // nil if not open
}

func (s *UIService) ShowTrafficWindow() {
    if s.TrafficWindow != nil {
        s.TrafficWindow.RequestFocus()
        return
    }
    win := s.app.NewWindow("Traffic Profiler")
    // ... setup content
    win.SetOnClosed(func() {
        // unsubscribe from profiler, stop live capture if no session
        s.TrafficWindow = nil
    })
    s.TrafficWindow = win
    win.Show()
}
```

Process picker:
- macOS: enumerate via `ps -axco command,pid` + `osascript` для bundle path / `NSWorkspace.runningApplications`
- Windows: `EnumProcesses` + `QueryFullProcessImageName`
- Linux: `/proc/<pid>/exe` + `.desktop` lookup для display name

Encapsulated в `internal/platform/proclist.go` (новый file, build-tag separated).

### Recording indicator

Два места:

1. **Кнопка в Diagnostics tab** — label `"Traffic Profiler ⚡"` когда `profiler.ActiveSession() != nil`, иначе `"Traffic Profiler"`. Update через TrafficProfiler subscriber на session start/stop.

2. **Title окна** — `"Traffic Profiler ⚡ Recording · 02:34"` при active session (с таймером, обновляется раз в секунду через `fyne.Do`). При idle — `"Traffic Profiler"`.

Update wiring: TrafficProfiler exposes channel событий lifecycle (start/stop session, tick), UI subscriber'ы (Diagnostics button + window title) обновляются через `fyne.Do`. Recording продолжается даже когда окно закрыто — на следующем re-open юзер видит активную session.

### Lifecycle

- **App start** → TrafficProfiler singleton init'ится (rolling buffer + log tail + Clash poll always-on). Окно не показывается.
- **Юзер жмёт «Traffic Profiler» в Diagnostics** → ShowTrafficWindow() → окно создаётся → subscribe на profiler events.
- **Юзер закрывает окно (X)** → unsubscribe → window = nil. Profiler singleton продолжает работать (если active session — recording продолжается; rolling buffer тоже не останавливается, т.к. он дешёвый и нужен для следующего open).
- **App quit** → all sessions wiped (in-memory only). Profiler goroutines stop.

Отличие vs «Live tab всегда on while window open» (LxBox): мы оставляем background capture (rolling buffer + clash poll) всегда on на app lifetime — это дёшево и решает «открыл окно, увидел уже накопленное» UX. CPU/battery impact мизерный без verbose toggle (regex-парсинг ~50-200 строк/сек).

---

## Edge cases & limits

| Случай | Поведение |
|---|---|
| `find_process: false` в config'е | Live + Per-process tab показывают banner с инструкцией «Enable process detection in wizard template». Profiler работает но process_path везде пустой → все events confidence=unattributed |
| Process detection misses (system-level, kernel TCP) | Fallback: inferred attribution через recent DNS IP (10s window), отметка 〽 |
| Verbose toggle включается/выключается mid-session | Sing-box reload, active connections рвутся. Warning dialog при toggle. Session continues с новым conn-id space, в meta фиксируется `verbose_toggled_at`. |
| Session events overflow (>50000 ev или >3h) | Drop oldest, counter `events_dropped` в session meta, виден в UI footer |
| Memory pressure | Max 6 sessions concurrent (1 active + 5 completed). Old auto-evict'ятся (FIFO) |
| Sing-box restart mid-session | Auto-finalize partial DNS chains. Session continues с новым conn-id space. UI notification «Sing-box reloaded — recording continues» |
| App quit / kill | Все in-memory sessions стираются. Persist принципиально не делается — Share/Copy через overflow menu если нужно сохранить |
| Log file rotation (singbox-launcher's own logrotate) | `fsnotify` detect, reopen без потерь — буфер log-tail'а удерживает ~500ms |
| Clash API недоступен (sing-box не запущен) | Live tab показывает «Sing-box not running»; Per-process pick disabled |
| Hostless connection (raw TCP без SNI / без DNS resolve) | Event пишется с `Domain=""`, `IP+Port` only. В Live event row отображается как `[<ip>]:<port>`. Видим в IPs sub-tab, в Domains tab отсутствует |
| Conn-id collision после sing-box reload | sing-box генерит conn-id заново, реюз практически невозможен; на reload `_dnsAccumulator` и `_connProcessMap` очищаются |

---

## Acceptance

1. Кнопка «Traffic Profiler» в Diagnostics tab открывает отдельное singleton окно. Повторный клик focus'ит существующее.
2. Окно содержит Live + Per-process views (как tabs внутри окна).
3. Live view показывает stream system-wide events realtime без recording.
4. Per-process: pick process → ▶ START → recording active → 4 sub-tab'а (Live/Domains/IPs/Connections) работают.
5. Process picker показывает запущенные процессы с display name + executable path + icon (best-effort per OS).
6. DNS chain reconstruction: CNAME-targets копятся в chain, оригинальный domain виден в Domains sub-tab (не CDN).
7. Connection issues (DnsTimeout, TcpRstEarly) детектируются и помечаются ⚠ + tooltip.
8. Verbose toggle: меняет `vars[log_level]=debug`, reload sing-box, revert на OFF, banner внутри окна.
9. Pre-session backfill: 60s rolling buffer событий target process'а копируется в session на START.
10. Ring-buffer 5 completed sessions. Force-stop = все стираются (in-memory only).
11. Export session JSON через overflow menu (copy to clipboard + save to file).
12. Recording indicator: ⚡ badge на кнопке «Traffic Profiler» в Diagnostics tab + в title окна (с таймером) когда session active.
13. Окно можно закрыть — active session продолжается в background, на повторном open юзер видит её active.
14. Process detection отключен → banner с инструкцией внутри окна.
15. Process attribution через `find_process` работает на macOS и Windows. На Linux при `find_process: true` тоже работает (verify).
16. Fyne UI thread safety: все event push'ы в UI через `fyne.Do()`. Goroutine профайлера не блокирует main thread. Tooltip layer (`fynetooltip.AddWindowToolTipLayer`) подключён к окну для ttwidget виджетов.
17. Memory cap: 50000 events / 3h sliding window. Counter `events_dropped` виден в footer.

---

## План фаз

1. **Phase 1 — Data sources**
   - `internal/traffic/clash_connections.go` — Clash API `/connections` poll + diff
   - `internal/traffic/logtail.go` — `bin/logs/sing-box.log` tail через fsnotify
   - `internal/traffic/parser.go` — regex parsing DNS/router/connection lines + unit-тесты на log samples

2. **Phase 2 — Profiler service**
   - `internal/traffic/profiler.go` — singleton `TrafficProfiler` + Session lifecycle
   - Cross-source join: conn-id ↔ process map, DNS accumulator
   - Subscribe/snapshot API
   - Connection issue classifier (DnsTimeout, TcpRstEarly)
   - Tests: lifecycle, log parsing, attribution, issues

3. **Phase 3 — Process listing platform code**
   - `internal/platform/proclist_darwin.go` / `_windows.go` / `_linux.go`
   - Enumerate running processes with path + display name + icon (path к icon файлу)
   - Tests: smoke-test на macOS host

4. **Phase 4 — UI: window shell + Live system-wide view**
   - `ui/traffic/window.go` — singleton `TrafficWindow`, открывается через `UIService.ShowTrafficWindow()`. Fynetooltip layer подключён.
   - Button «Traffic Profiler» в `ui/diagnostics_tab.go` (Diagnostics section).
   - Окно: `container.AppTabs` с двумя tab'ами (Live / Per-process).
   - Live: stream rendering через Subscribe(), filter chips, search, newest-first scroll.

5. **Phase 5 — UI: Per-process recording view**
   - Process picker dialog
   - ▶/⏹ button + recording status
   - 4 sub-tab'а (Live/Domains/IPs/Connections) с aggregated views
   - Saved sessions list + open/share/delete

6. **Phase 6 — Verbose toggle + overflow menu**
   - Toggle wired в `vars[log_level]` через configurator
   - Reload sing-box flow
   - Banner widget
   - Overflow menu: copy/export/clear/help

7. **Phase 7 — Recording indicator + edge cases**
   - ⚡ badge на button label в Diagnostics tab + в title окна
   - Singleton focus-on-reopen behavior
   - find_process=false banner
   - Sing-box-not-running banner
   - Memory cap counter

8. **Phase 8 — Build + tests + reinstall + docs**
   - `docs/TRAFFIC_PROFILER.md` user guide
   - Smoke-test на live machine: запустить Slack/Chrome/Telegram, проверить attribution
   - Update RELEASE_NOTES/upcoming.md

## Риск

**Средний.** Самое рискованное — log parsing: sing-box log format не stable между minor releases. Mitigation: parser изолирован в одном файле + zoo of log samples в `testdata/`; на regression теста ловим до релиза.

Второе по риску — process listing на разных OS: каждая платформа требует своего impl. Mitigation: best-effort + fallback на «pick by path» (вручную ввести путь к executable).

## Final decisions (open vs LxBox)

| # | Вопрос | Decision | Comment |
|---|---|---|---|
| 1 | Где живёт UI | **Отдельное окно**, запуск кнопкой из Diagnostics tab | Diagnostics — secondary workflow, окно можно расположить рядом с проблемным app'ом; не конкурирует за tab-bar с Configurator wizard |
| 2 | Min sing-box version | Текущий pinned (1.13.11) | Лог-формат текущий, если sing-box обновится — обновим parser regex'ы |
| 3 | macOS process icon source | NSWorkspace iconForFile (через cgo) | На MVP — fallback to generic icon если cgo не trivial |
| 4 | Persist sessions | **No** | In-memory only как в LxBox |
| 5 | Session JSON schema version | **No version** | In-memory only, no import path |
| 6 | Recording across `core.ReloadSingBox()` | **Continue with new conn-id space** | Auto-finalize partial; session не теряется |
| 7 | Background capture (rolling buffer + clash poll) lifetime | **Always on while app runs** | Дёшево (~50-200 lines/sec parsing); решает «открыл окно → видишь уже накопленное» UX. Recording продолжается даже при закрытом окне |
| 8 | Pre-session backfill window | **60s × ~3000 ev** | Как в LxBox §048 |
| 9 | Completed sessions cap | **5** | Как в LxBox |
| 10 | Window singleton policy | **Один экземпляр**, повторный клик focus'ит | Не плодим окна; recording state виден после reopen |

## Не в скоупе (post-MVP)

- Inline actions: Add to direct / Block domain / Make preset from selected
- Differential mode (compare session A vs B)
- HTTP-level inspection (URL/headers) — sing-box capability missing
- TLS fingerprinting — sing-box capability missing
- Persist sessions через app restart
- Community export library (share .json sessions)
- Latency / RTT per-domain — только bytes на MVP
