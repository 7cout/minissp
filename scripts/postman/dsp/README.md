# Postman-запросы для DSP Service

## Настройка Postman

1. **New → gRPC Request**.
2. **URL:** `localhost:50052`.
3. **Service definition:** Automatic (через reflection) или импорт `proto/dsp/v1/dsp.proto`.
4. **Method:** `dsp.v1.DspService/GetBid`, `/Commit` или `/Rollback`.
5. **Message:** скопируйте JSON из файла.

## Порядок тестирования

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
| 04 | InvalidArgument: width must be positive |
| 05 | InvalidArgument: geo |
| 06 | NotFound: no eligible campaign |
| 07 | NotFound: no eligible campaign |
| 08 | NotFound: no eligible campaign |
| 09 | OK, ok=true |
| 10 | InvalidArgument: campaign_id is required |
| 11 | InvalidArgument: price must be positive |
| 12 | NotFound: campaign not found |
| 13 | FailedPrecondition: insufficient budget |
| 14 | OK, ok=true |
| 15 | InvalidArgument: campaign_id is required |
| 16 | NotFound: campaign not found |
| 17 | FailedPrecondition: insufficient budget |

## ID из seed-данных

- **Nike campaign:** `33333333-3333-3333-3333-333333333333`
- **Adidas campaign:** `55555555-5555-5555-5555-555555555555`
- **Puma campaign:** `77777777-7777-7777-7777-777777777777`
- **Несуществующая кампания:** `99999999-9999-9999-9999-999999999999`