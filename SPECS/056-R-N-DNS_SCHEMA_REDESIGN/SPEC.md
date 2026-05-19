# SPEC 056-R-N — DNS_SCHEMA_REDESIGN

**Status:** New (N)
**Type:** Refactor (R) — schema cleanup, no new user-facing features
**Depends on:** SPEC 053 (preset bundles — концепт остаётся), SPEC 055 (preset.outbounds — не трогаем)
**Schema:** последний релизный формат — **v5** (`v0.9.5`). **v6 / `presets_v1`
никогда не релизился** — это дев-схема, существует только в HEAD-of-develop
после SPEC 053. Этот SPEC **переписывает дев-схему на месте**, БЕЗ инкремента
номера. Когда консолидированный preset-стек уйдёт в релиз, тогда и будет
честный bump v5 → v6 уже с правильным дизайном.

---

## Что не так в v6 (дев-схема, SPEC 053)

```json
"dns": {
  "template_servers": { "<tag>": {"enabled": bool} },   // map
  "extra_servers":    [ { full server body } ],          // array
  "extra_rules":      [ { ... } ]                         // array
}
```

**Проблемы (только в DNS — `state.rules[]` route part сделан правильно через kind):**

1. **DNS-серверы в 3 коллекциях.** template-overrides (map) + user-added (array) +
   bundled-runtime (нигде, эмитится из presets). Каждый источник со своим эмит-путём.

2. **Артефактные имена `extra_*`.** Не очевидны — "extra" по отношению к чему? Юзер
   читает «лишние? необязательные?», а по факту «**сверх** template + preset».

3. **Имя секции `dns` неинформативно.** В template'е та же секция называется
   `dns_options`, а в state — `dns`. Зеркальность ломается, в коде вечная путаница.

4. **Back-compat хаки.** `legacyDNSOptionsFromV6` материализует v6 в v5 view
   для UI back-compat → создаёт double-emit риск → разрешён inline guard'ом
   v5/v6 в `dnsConfigForUpdate` → плодит сущности.

5. **Асимметрия с rules.** `state.rules[]` уже использует kind discriminator
   pattern (preset/inline/srs) и работает чисто. DNS — нет. Один продукт,
   разные принципы.

## Как должно быть

`kind` discriminator + flat layout — тот же паттерн что у `state.rules[]`.
Schema version и schema name **не меняются** (v6 / `presets_v1` остаются как
дев-маркер до релиза):

```json
{
  "meta": { "version": 6, "schema": "presets_v1" },

  "rules": [
    // НЕ ТРОГАЕМ — kind discriminator уже работает
    { "kind": "preset", "ref": "russian", "enabled": true, "body": {"vars": {}} },
    { "kind": "inline", "id":  "01J...",  "enabled": true, "body": {...} },
    { "kind": "srs",    "id":  "01J...",  "enabled": true, "body": {...} }
  ],

  "vars": [
    // НЕ ТРОГАЕМ — единый KV-store для всех template переменных,
    // включая dns_strategy / dns_final / dns_independent_cache /
    // dns_default_domain_resolver. UI группирует по табам, state — flat.
    { "name": "tun", "value": "true" },
    { "name": "dns_strategy", "value": "prefer_ipv4" },
    ...
  ],

  "dns_options": {
    "servers": [
      { "kind": "template", "tag": "cloudflare_udp", "enabled": true  },
      { "kind": "template", "tag": "google_doh",     "enabled": true  },
      { "kind": "template", "tag": "cloudflare_doh", "enabled": false },
      { "kind": "user",     "tag": "my-pihole", "type": "udp", "server": "192.168.1.5", "server_port": 53 }
    ],
    "rules": [
      { "kind": "user", "rule_set": "ru-domains", "server": "yandex_doh" }
    ]
  }
}
```

**Что меняется:**

| Сейчас (v6) | Стало |
|---|---|
| `state.dns.template_servers: {tag: {enabled}}` (map) | `state.dns_options.servers[]` с `kind="template"` + `enabled` |
| `state.dns.extra_servers: [...]` (array, full body) | то же `servers[]` с `kind="user"` + full body |
| `state.dns.extra_rules: [...]` | `state.dns_options.rules[]` с `kind="user"` |
| Имя секции `dns` | `dns_options` (зеркалит template-секцию того же имени) |
| Имена `extra_*` в коде | `kind="user"` discriminator |

**Bundled DNS-серверы от preset'ов:** runtime-only, в state НЕ хранятся (это
осталось хорошо в v6, не меняем). Эмитятся при build из `preset.dns_servers`.
UI может показывать их с префиксом `<preset>:` и пометкой «bundled by preset X»,
но в state.json не сохраняет — disable preset → исчезают автоматически.

## Что НЕ трогаем

- `state.rules[]` — уже kind-based, работает чисто
- `state.vars[]` — единый KV-store, включая `dns_*` scalars (UI группирует
  по таб'ам, state.json — flat; перенос в `dns_options.vars[]` был бы
  синтаксическим сахаром без логического выигрыша)
- SPEC 055 preset.outbounds — отдельная фича, своя архитектура (pre-patch parser_config)
- `template.presets[]` концепт целиком — preset bundles остаются
- `template.dns_options.servers[]` как библиотека template DNS-серверов
- `template.config.dns.servers[]` (минимум: local_dns_resolver, direct_dns_resolver)
- Build pipeline для outbounds, route, parser_config — не касается

## Acceptance

1. `state.json` после Save имеет секцию `dns_options.servers[]`/`rules[]`
   через `kind` discriminator. Секции `dns.template_servers`,
   `dns.extra_servers`, `dns.extra_rules` отсутствуют.
2. `state.vars[]` неизменно — `dns_*` scalars остаются как раньше.
3. **Удалены** функции:
   - `legacyDNSOptionsFromV6` (не нужна — нет двух views)
   - inline v5/v6 guard в `dnsConfigForUpdate` (всегда читаем из единого места)
   - `collectRuleSetTagsFromPresets` + `cleanDanglingDNSRule` — пересматриваем:
     остаются если нужны для `dns_options.rules[kind=user]` с `rule_set` ссылками
     на preset rule_set'ы (дроп dangling при disable preset)
4. **Единый emit-путь** для DNS в build: `MergeDNSSection` + `MergePresetsIntoDNS`
   сливаются — один walk по `dns_options.servers[]` со `switch entry.Kind` +
   merge с template-эмитнутыми + preset-bundled.
5. UI рендерит DNS tab из flat `dns_options.servers[]` со kind-aware tile'ами:
   - `kind="template"` → только toggle enable; edit/delete заблокированы
   - `kind="user"` → полный edit + delete
   - bundled (preset-emitted, с префиксом `<preset>:`) → read-only с пометкой
6. **Dev state переписывается на месте.** Schema version и name НЕ меняются
   (v6 / `presets_v1` остаются маркером дев-стека до релиза). На первой
   загрузке после деплоя старого-дев-v6 формата: read со старого shape
   (`dns.template_servers`/`extra_servers`/`extra_rules`), write в новом
   shape, backup `state.json.dev-pre-redesign.bak`.
   Шиппнутые юзеры на v5 — не затрагиваются (parseV5 продолжает работать
   как раньше; в v6-shape переходят только при первом Save с preset-ref'ом
   как обычно, но уже в **правильном** layout'е).
7. `docs/WIZARD_STATE.md` переписан под актуальную схему (попутно —
   раз уж doc всё равно отстал).

## Не в скоупе

- Перенос `dns_*` scalars из `state.vars[]` в `dns_options` (обсуждалось,
  отбито — это был бы синтаксический сахар без логического выигрыша; единый
  KV-store `state.vars[]` остаётся)
- Откат SPEC 053 целиком — preset bundles концепт остаётся, route rules `kind`
  pattern остаётся
- Откат SPEC 055 — preset.outbounds работает, не трогаем
- Изменения UI кроме DNS tab
- Изменения в template structure

## План фаз (детально — в `IMPLEMENTATION_PLAN.md` после approval)

1. **Phase 1 — Schema:** новый `v6.DNSOptions` struct: `Servers []DNSServer{Kind,
   Tag, Enabled, Body map[string]interface{}}` + `Rules []DNSRule{Kind, Body
   map[string]interface{}}`. Schema version и name **не меняются**. Старый
   `v6.DNSConfig` (template_servers/extra_servers/extra_rules) удаляется.

2. **Phase 2 — In-place dev rewrite:** на parseV6 — если встречаем старый
   shape (`dns.template_servers` / `dns.extra_servers` / `dns.extra_rules`),
   читаем по-старому, конвертим в in-memory `v6.DNSOptions` нового shape,
   backup `state.json.dev-pre-redesign.bak`, на ближайшем Save файл
   перезаписывается в новом layout'е.

3. **Phase 3 — Build pipeline:** переписать `MergeDNSSection` + `MergePresetsIntoDNS`
   под единый walk с kind switch. Удалить `legacyDNSOptionsFromV6`. Удалить
   inline v5/v6 guard в `dnsConfigForUpdate`. Сохранить `cleanDanglingDNSRule`
   для `kind=user` DNS rules с `rule_set` ссылками на preset rule_set'ы.

4. **Phase 4 — UI sync:** переписать `SyncDNSFullToStateV6` под flat shape
   с kind. UI рендерит DNS tab из единого списка с kind-aware рендером tile'ов.

5. **Phase 5 — UI rendering polish:** DNS tab tile'ы становятся kind-aware:
   template = toggle only, user = edit/delete, bundled = read-only с preset hint.

6. **Phase 6 — Tests + docs:** unit'ы под flat shape, `docs/WIZARD_STATE.md`
   переписан, `IMPLEMENTATION_REPORT.md` финал.

## Объём работы

~70 LOC чистого прироста (~120 нового кода − ~50 удалений legacy хаков).
~1 день focused work. Риск низкий — дев-only schema rewrite, шипп v5 не
затрагивается (parseV5 работает как раньше).

## Риск / rollback

Дев-only schema rewrite. Шиппнутая v5 не затрагивается (parseV5 работает
как раньше). Дев-юзеры (только разработчик) получают одноразовый
`state.json.dev-pre-redesign.bak` + read-старого-shape для совместимости
при первой загрузке. Тесты на round-trip старый-shape → новый-shape →
emit (данные сохраняются 1:1 по семантике). На любом этапе можно
остановиться — старый read-path остаётся до того момента как Phase 2
сконвертирует конкретный state.
