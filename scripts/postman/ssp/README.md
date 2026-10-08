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
| 04 | InvalidArgument: name is required |
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
| 15 | InvalidArgument: geo |
| 16 | InvalidArgument: min_price cannot be negative |
| 17 | InvalidArgument: name is required |

## ID из seed-данных SSP

- Publisher: `22222222-2222-2222-2222-222222222222`
- Slot home_banner: `11111111-1111-1111-1111-111111111111`

## Ключи

- Publisher → SSP: `ssp_dev_secret_key_777`
- SSP → DSP: `ssp_dev_secret_key_777`

## Полный E2E-цикл через Postman

Чтобы проверить, что SSP и DSP работают вместе:

1. Запусти DSP: `task run:dsp`
2. Запусти SSP: `task run:ssp`
3. В Postman (SSP Collection) отправь `06-run-auction-happy` — получишь `auction_id`.
4. Подставь `auction_id` в `10-impression-happy` — отправишь. Проверь, что ответ `ok=true`.
5. Отправь `13-get-balance` — баланс publisher'а должен вырасти на `price * 0.8`.
   - Например, при ставке 1_500_000 publisher получит 1_200_000.

Если `06` возвращает `NotFound: no bids received`:
- проверь, что DSP запущен на `localhost:50052`;
- проверь, что `api-key` в DSP Collection тоже стоит (SSP шлёт его в DSP автоматически);
- проверь в логах DSP, что запрос дошёл и был распарсен.