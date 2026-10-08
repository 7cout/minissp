# Схема БД

Заметка по PostgreSQL в MiniSSP. Не ADR, просто шпаргалка:
что за таблицы, как храним, какие SQL-паттерны используем.

## Таблицы

**SSP:**

    publishers       id, name, api_key, balance
    ad_slots         id, publisher_id, name, geo, min_price, type,
                     banner_width, banner_height,
                     video_width, video_height, video_duration, video_mimes
    auctions         id, imp_id, campaign_id, creative_id, publisher_id,
                     slot_id, bidder_id, price, status, created_at
    event_log        id, type, auction_id, campaign_id, publisher_id,
                     price, publisher_share, ssp_revenue, created_at

**DSP:**

    advertisers      id, name, balance
    campaigns        id, advertiser_id, name,
                     budget_total, budget_remaining, budget_reserved, geo_target
    creatives        id, campaign_id, type, url, click_url, params

## Типы

- **Деньги** — `BIGINT` в микроединицах. Никаких float.
- **ID** — `UUID`.
- **Geo** — `VARCHAR(2)`, ISO 3166-1 alpha-2.
- **Type** — `TEXT`: `banner`, `video`, `native`, `audio`.
- **Параметры креатива** — явные колонки под каждый тип
  (`banner_width`, `banner_height`, `video_*`), не JSONB.
  Причины: горячий путь (GetSlotByName на каждый RunAuction),
  ограниченное число типов (4), CHECK-и в БД на корректность.
- **Время** — `TIMESTAMPTZ`.

### Почему не JSONB для параметров

В AdTech на горячем пути важна скорость и предсказуемость.
Явные колонки быстрее парсятся, индексируются стандартно и
типизированы CHECK-ами. JSONB оставляем для `event_log` — там
схема нестабильна и объём большой.

## Индексы

- `publishers (api_key)` — UNIQUE, для аутентификации.
- `ad_slots (publisher_id, name)` — UNIQUE, для идемпотентного RegisterSlot.
- `campaigns (geo_target) WHERE budget_remaining > 0` — partial, для ListByGeo.
- `creatives (campaign_id)` — для ListByCampaign.
- `auctions (status, created_at)` — для TTL-воркера.
- `event_log (created_at)` — для чтения событий.

## Checks

- `balance >= 0` — на всех балансах.
- `budget_remaining >= 0`, `budget_reserved >= 0`, `budget_total >= 0`.
- `budget_remaining + budget_reserved <= budget_total` — на `campaigns`.
- `min_price >= 0`, `price > 0`.

## Ключевые SQL-паттерны

**Резерв бюджета — атомарно, без SELECT-then-UPDATE:**

```sql
UPDATE campaigns
SET budget_remaining = budget_remaining - $1,
    budget_reserved  = budget_reserved  + $1
WHERE id = $2 AND budget_remaining >= $1
RETURNING budget_remaining, budget_reserved;
```

Пусто в `RETURNING` — либо кампании нет, либо бюджета не хватило.
Различаем `SELECT EXISTS`.

**Commit в DSP — одна транзакция:**

```sql
BEGIN;

UPDATE campaigns
SET budget_reserved = budget_reserved - $1
WHERE id = $2 AND budget_reserved >= $1;
-- RowsAffected != 1 → ROLLBACK, ErrInsufficientBudget

UPDATE advertisers
SET balance = balance - $1
WHERE id = $3 AND balance >= $1;
-- RowsAffected != 1 → ROLLBACK, ErrInsufficientBalance

COMMIT;
```

Так закрывается баг #5 — ручная компенсация через `Uncommit` не нужна.

**RegisterSlot — без race:**

```sql
INSERT INTO ad_slots (id, publisher_id, name, geo, min_price, type, params)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (publisher_id, name) DO NOTHING
RETURNING id, publisher_id, name, geo, min_price, type, params;
```

Пусто в `RETURNING` — слот есть, делаем `SELECT` и возвращаем его.

**Impression — state machine:**

```sql
UPDATE auctions
SET status = 'processing'
WHERE id = $1 AND status = 'pending'
RETURNING imp_id, campaign_id, creative_id, publisher_id, slot_id,
          bidder_id, price, created_at;
```

Пусто — смотрим текущий статус:
- `processing` / `committed` — идемпотентный повтор, возвращаем nil.
- `rolled_back` — ErrAuctionNotFound.
- `commit_failed` — можно ретраить.

**TTL-воркер:**

```sql
UPDATE auctions
SET status = 'rolled_back'
WHERE status = 'pending'
  AND created_at < now() - interval '30 seconds'
RETURNING id, campaign_id, bidder_id, price;
```

Позже заменится на Redis TTL.

## Ошибки

Маппим `pgx` в доменные sentinel прямо в репозиториях:

| pgx / pgerrcode | Домен |
|---|---|
| `pgx.ErrNoRows` | `ErrXxxNotFound` |
| 23505 (unique) | `ErrSlotAlreadyExists` |
| 23514 (check) | `ErrInvalidID`, `ErrInsufficientBudget` |
| 23503 (fk) | `ErrCampaignNotFound`, `ErrPublisherNotFound` |

`pgx` в сервисный слой не утекает.

## Драйвер и переключение

- **`pgx/v5`** + raw SQL. ORM не нужен — атомарные UPDATE с условиями
  в `WHERE` это SQL-нативная логика, ORM её только испортит.
- Переключение через env:

      STORAGE=memory    # по умолчанию
      STORAGE=postgres

- Репозитории в `internal/*/repository/memory/` и `internal/*/repository/postgres/`
  реализуют одни и те же интерфейсы. Сервисный слой не меняется.

## Миграции

- Инструмент — **goose**, уже настроен в `Taskfile.yml`.
- Формат: `NNNNNNNNNNNNNN_name.sql` с `-- +goose Up` / `-- +goose Down`.
- Порядок: publishers → ad_slots → advertisers → campaigns → creatives → auctions → event_log → seed.

## Связанные документы

- [MVP-сценарий](mvp-scenario.md) — поток данных и деньги.
- [ADR 006. Трёхсервисная архитектура](../decisions/006-three-service-architecture.md)