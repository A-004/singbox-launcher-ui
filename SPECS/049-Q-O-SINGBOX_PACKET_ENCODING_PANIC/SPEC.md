# SPEC 049 — SINGBOX_PACKET_ENCODING_PANIC (upstream report)

**Тип:** Q (Question / upstream report) · **Статус:** O (Open — issue ещё не залит апстрим, но материал собран).

Подготовка bug-report'а для **sing-box**: panic в `vless.NewOutbound`, когда `packet_encoding` имеет значение, не входящее в свитч (`""`, `"packetaddr"`, `"xudp"`). Корень — `format.ToString` в `sing/common/format` не покрывает `*string`, и при попытке отформатировать сообщение `unknown packet encoding: <ptr>` падает в `panic("unknown value")` вместо честного `E.New`-error.

Этот SPEC — материал для отдельного апстрим-issue. Не блокер для нас (свою сторону мы фиксим в следующей фич-ветке: валидируем `packetEncoding` на этапе парсинга подписки), но баг апстрима **всё равно надо репортнуть** — чтобы любой sing-box-клиент, у которого пользователь подсунет неизвестный `packet_encoding`, не крашился, а получал нормальную ошибку конфигурации.

---

## 1. Что воспроизводится

**Версия sing-box:** v1.13.11 (CI-сборки, выложенные на GitHub Releases). Скорее всего тот же код в более ранних 1.13.x.

**Триггер:** `outbounds[*].type == "vless"` имеет поле `"packet_encoding"` со значением, не равным `""`, `"packetaddr"` или `"xudp"`. Например — `"none"`.

**Минимальный конфиг для воспроизведения:**

```jsonc
{
  "outbounds": [
    {
      "tag": "test-vless",
      "type": "vless",
      "server": "example.com",
      "server_port": 443,
      "uuid": "00000000-0000-0000-0000-000000000000",
      "packet_encoding": "none",
      "tls": {"enabled": true, "server_name": "example.com"}
    }
  ]
}
```

Команда:

```sh
sing-box check -c minimal.json
```

**Ожидание:** сообщение об ошибке вида `unknown packet encoding: none` с exit-code != 0.

**Реальность:** `panic: unknown value` + полный goroutine stack trace, exit-code 2.

## 2. Stack trace (с macos arm64, sing-box 1.13.11)

```
panic: unknown value

goroutine 1 [running]:
github.com/sagernet/sing/common/format.ToString({0x140003d27c8?, 0x140004e6d50?, 0x151467f88?})
	github.com/sagernet/sing@v0.8.9/common/format/fmt.go:60 +0x540
github.com/sagernet/sing/common/exceptions.New(...)
	github.com/sagernet/sing@v0.8.9/common/exceptions/error.go:26
github.com/sagernet/sing-box/protocol/vless.NewOutbound(...)
	github.com/sagernet/sing-box/protocol/vless/outbound.go:86 +0x704
github.com/sagernet/sing-box/adapter/outbound.Register[...].func2(...)
	github.com/sagernet/sing-box/adapter/outbound/registry.go:23 +0xfc
github.com/sagernet/sing-box/adapter/outbound.(*Registry).CreateOutbound(...)
	github.com/sagernet/sing-box/adapter/outbound/registry.go:64 +0x204
github.com/sagernet/sing-box/adapter/outbound.(*Manager).Create(...)
	github.com/sagernet/sing-box/adapter/outbound/manager.go:265 +0x70
github.com/sagernet/sing-box.New(...)
	github.com/sagernet/sing-box/box.go:288 +0x1cd8
main.check()
	github.com/sagernet/sing-box/cmd/sing-box/cmd_check.go:34 +0x110
main.init.func1(...)
	github.com/sagernet/sing-box/cmd/sing-box/cmd_check.go:16 +0x1c
github.com/spf13/cobra.(*Command).execute(...)
	github.com/spf13/cobra@v1.10.2/command.go:1019 +0x7bc
...
main.main()
	github.com/sagernet/sing-box/cmd/sing-box/main.go:8 +0x24
```

(Тот же stack-trace воспроизводится на windows-amd64 — см. `singbox-launcher.log.old:1369–1395` в attached логе пользователя.)

## 3. Корень бага

### 3.1. Триггер в `vless/outbound.go:86`

[`protocol/vless/outbound.go`](https://github.com/SagerNet/sing-box/blob/v1.13.11/protocol/vless/outbound.go#L76-L90):

```go
if options.PacketEncoding == nil {
    outbound.xudp = true
} else {
    switch *options.PacketEncoding {
    case "":
    case "packetaddr":
        outbound.packetAddr = true
    case "xudp":
        outbound.xudp = true
    default:
        return nil, E.New("unknown packet encoding: ", options.PacketEncoding)
                                                     // ↑ здесь *string, не string
    }
}
```

`options.PacketEncoding` объявлен как **`*string`** (указатель). В default-ветке он передаётся в `E.New(...)` как есть — указателем, не разыменованной строкой.

### 3.2. Падение в `format/fmt.go:60`

[`common/format/fmt.go`](https://github.com/SagerNet/sing/blob/v0.8.9/common/format/fmt.go#L18-L65):

```go
func ToString(items ...any) string {
	output := ""
	for _, item := range items {
		switch message := item.(type) {
		case nil: ...
		case string: ...
		case []byte: ...
		case bool: ...
		case int, int8, int16, int32, int64,
		     uint, uint8, uint16, uint32, uint64: ...
		case float32, float64: ...
		case uintptr: ...
		case error: ...
		case Stringer: ...
		default:
			panic("unknown value")
		}
	}
	return output
}
```

Свитч **не покрывает указательные типы** (`*string`, `*int`, и т.д.). Когда `E.New` (это просто обёртка над `errors.New(format.ToString(args...))`) получает `*string`, тип не совпадает с `string`-кейсом, попадает в `default`, и → `panic("unknown value")`.

В итоге **там, где должно быть «return invalid configuration error», sing-box просто крашится**. Любой `sing-box check`/`sing-box run` с такой конфигурацией убивает процесс полностью.

## 4. Что предлагается фиксить апстрим

### Вариант A — поправить `format.ToString` (правильный)

В `sing/common/format/fmt.go` обработать указатели через рефлексию или добавить case'ы для распространённых указательных типов (`*string`, `*int`, etc.). Использовать `fmt.Sprintf("%v", item)` как fallback вместо паники. Аргумент: `format.ToString` называется «общим to-string», его задача — никогда не паниковать на легитимном Go-значении. Сейчас даже банальная программистская опечатка (передал указатель вместо разыменованного значения) полностью валит процесс.

### Вариант B — поправить `vless.NewOutbound:86` (косметический)

Изменить `E.New("unknown packet encoding: ", options.PacketEncoding)` на `E.New("unknown packet encoding: ", *options.PacketEncoding)`. Точечный фикс, но защищает только от этого callsite — остальные места в sing-box, где `E.New` получает указатель, продолжат паниковать.

### Рекомендация

**Сделать оба**: `format.ToString` обязан не паниковать (вариант A), но и в `vless/outbound.go` правильно разыменовать (вариант B) — корректность аргументов на стороне caller'а тоже хорошо.

## 5. Воспроизводимый репро для апстрим-issue

Минимальный JSON (см. §1) + одна команда `sing-box check -c minimal.json`. Никаких подписок / сети / TUN. Чистая логика парсинга outbounds.

Файл-фикстура для скриншота / приложения к issue: можно положить в свой gist или прямо в issue как код-блок.

## 6. Контекст со стороны клиента (singbox-launcher)

Параллельно **мы фиксим свою сторону** — `core/config/subscription/node_parser.go:607–609` слепо копирует значение query-параметра `?packetEncoding=` из URI подписки в `outbound.packet_encoding`, без allow-list. Когда подписка отдаёт `vless://...?packetEncoding=none`, у нас уезжает `"packet_encoding": "none"` в config.json → sing-box крашится.

Наш фикс (отдельный SPEC / релиз 0.8.8.4):
- Sanitize: принимаем только `xudp`, `packetaddr`. Любое другое значение (включая `none`) — **поле опускаем** в outbound, sing-box получит nil pointer и применит дефолт.
- Случай `none` логически эквивалентен «без специального encoding» = sing-box default → лучше всего просто опустить поле.

Этот фикс **не зависит** от апстрим-фикса; апстрим нужен сам по себе, чтобы любой клиент с любой опечаткой в `packet_encoding` получал нормальную ошибку, а не panic.

## 7. Артефакты

- **Лог-файл пользователя:** строки 1369–1395 в `singbox-launcher.log.old` (присутствуют в issue-вложениях; не публиковать в апстрим — там UUID-ы её нод, лишний шум).
- **Триггерный конфиг:** одна VLESS-нода с `packet_encoding=none`, всё остальное (TLS, transport) — стандартное.
- **Версия sing-box, на которой воспроизведено:** v1.13.11 (текущая stable на момент 2026-04-27). Нужно проверить также 1.12.x и 1.11.x перед issue — могло быть и раньше.

## 8. План работы пользователя (после этого SPEC'а)

1. Открыть GitHub issue в [SagerNet/sing-box](https://github.com/SagerNet/sing-box/issues/new) — категория `Bug`.
2. Заголовок: `panic("unknown value") in vless.NewOutbound when packet_encoding has unknown value`.
3. В теле — содержимое §1 (репро), §2 (stack), §3 (корневая причина), §4 (предложение).
4. Параллельно — issue в [SagerNet/sing](https://github.com/SagerNet/sing/issues/new) на `format.ToString`-half.
5. Если сделают PR — отметить SPEC как **C** (Complete) + запомнить версию апстрима, в которой починилось.

## 9. Связи

- **Соседний SPEC:** наш собственный фикс в node_parser sanitization — будет отдельный SPEC (NNN-F-O-VLESS_PACKET_ENCODING_SANITIZE) когда сядем за фикс.
- **Источник проблемы у пользователя:** `singbox-launcher.log.old:1369–1395` (Telegram-чат, 27.04.2026). Не вкладывать в апстрим issue — там персональные данные.
- **Связано с:** общая тема валидации значений из подписок ([SPEC 044 NaïveProxy](../044-F-C-NAIVE_PROXY_PARSER/SPEC.md), SPEC 048 PARSER_CONFIG_VAR_SUBSTITUTION). Тренд — субcкрипционные URI могут содержать что угодно, парсер должен быть параноидальным.
