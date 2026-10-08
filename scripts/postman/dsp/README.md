# Postman-запросы для DSP Service

## Настройка Postman

1. **New → gRPC Request**.
2. **URL:** `localhost:50052`.
3. **Service definition:** Automatic (через reflection) или импорт `proto/dsp/v1/dsp.proto`.
4. **Method:** `dsp.v1.DspService/GetBid`, `/Commit` или `/Rollback`.
5. **Message:** скопируйте JSON из файла.

## Аутентификация

Все запросы требуют metadata `api-key: ssp_dev_secret_key_777`.
Без него — `Unauthenticated`. Если получил `Unauthenticated` с ключом —
проверь, что значение совпадает с `DSP_API_KEYS` в `deployments/.env`.

В Postman:
- Вкладка **Metadata** → добавь `api-key` со значением из .env.
- Или **Authorization → API Key → Add to Header**.

Если ключ не добавлен — сервер вернёт **Unauthenticated**.

## Порядок тестирования

## Тесты аутентификации

- Без metadata `api-key` → `16 UNAUTHENTICATED`.
- С неверным ключом → `16 UNAUTHENTICATED`.
- С валидным ключом → обычный ответ метода.

### Базовые (независимые)

Файлы **01–08** — `GetBid`. Каждый самодостаточен, можно запускать в любом порядке.

### Зависимые (stateful)

Файлы **09–17** — `Commit` и `Rollback`. Они **меняют состояние** репозитория.
Порядок важен:

**Для проверки Commit happy path:**
1. Запустите `01-getbid-happy.json` — создаётся резерв на Nike-кампанию.
2. Запустите `09-commit-happy.json` — резерв подтверждается, деньги списываются.

**Для проверки Rollback happy path:**
1. Запустите `01-getbid-happy.json` — создаётся резерв.
2. Запустите `14-rollback-happy.json` — резерв снимается.

**Если запустить Commit/Rollback без предварительного GetBid:**
- `09-commit-happy.json` вернёт **FailedPrecondition: insufficient budget**.
- `14-rollback-happy.json` — то же самое.

Это нормальное поведение. Чтобы вернуть чистое состояние — перезапустите DSP.

## Какие ответы ожидать

| Файл | Ожидаемый ответ |
|---|---|
| 01 | OK, BidResponse с campaign_id=Nike |
| 02 | InvalidArgument: request_id is required |
| 03 | InvalidArgument: imp id is required |
| 04 | InvalidArgument: banner width must be positive |
| 05 | InvalidArgument: geo |
| 06 | NotFound: no eligible campaign |
| 07 | NotFound: no eligible campaign |
| 08 | NotFound: no eligible campaign |
| 08a | InvalidArgument: banner type requires banner params |
| 08b | NotFound: no eligible campaign (видео-тип валиден, но кампаний нет) |
| 08c | InvalidArgument: unsupported creative type |
| 09 | OK, ok=true (требует GetBid 01) |
| 10 | InvalidArgument: campaign_id is required |
| 11 | InvalidArgument: price must be positive |
| 12 | NotFound: campaign not found |
| 13 | FailedPrecondition: insufficient budget (нет резерва) |
| 14 | OK, ok=true (требует GetBid 01) |
| 15 | InvalidArgument: campaign_id is required |
| 16 | NotFound: campaign not found |
| 17 | FailedPrecondition: insufficient budget (нет резерва) |

## ID из seed-данных

- **Nike campaign:** `33333333-3333-3333-3333-333333333333`
- **Adidas campaign:** `55555555-5555-5555-5555-555555555555`
- **Puma campaign:** `77777777-7777-7777-7777-777777777777`
- **Несуществующая кампания:** `99999999-9999-9999-9999-999999999999`