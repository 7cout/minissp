# Postman-запросы для SSP Service

## Настройка

- URL: `localhost:50051`
- Service definition: Automatic (reflection) или импорт `proto/ssp/v1/ssp.proto`
- Auth: на уровне Collection → API Key
  - Key: `api-key`
  - Value: `ssp_dev_secret_key_777`
  - Add to: Header

## Порядок тестирования

1. `01-get-slot-by-name` — убедиться, что seed-слот есть.
2. `06-run-auction-happy` — получить `auction_id`.
3. `10-impression-happy` — вставить `auction_id`, отправить.
4. `13-get-balance` — убедиться, что баланс вырос.

## Ожидаемые ответы

| Файл | Ответ |
|---|---|
| 01 | OK, Slot с id=11111111-... |
| 02 | NotFound: slot not found |
| 03 | OK, Slot с новым id |
| 04 | InvalidArgument: invalid request |
| 05 | InvalidArgument: banner params are required |
| 06 | OK, BidResponse с creativeUrl, clickUrl |
| 07 | InvalidArgument: request_id is required |
| 08 | InvalidArgument: slot_id is required |
| 09 | NotFound: slot not found |
| 10 | OK, ok=true |
| 11 | InvalidArgument: auction_id is required |
| 12 | NotFound: auction not found |
| 13 | OK, balance (растёт после 10) |
| 14 | Unauthenticated: missing api-key |

## ID из seed-данных SSP

- Publisher: `22222222-2222-2222-2222-222222222222`
- Slot home_banner: `11111111-1111-1111-1111-111111111111`

## Ключи

- Publisher → SSP: `ssp_dev_secret_key_777`
- SSP → DSP: `ssp_dev_secret_key_777`