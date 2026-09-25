# MVP-сценарий MiniSSP

 


## Роли

- **Publisher** — владелец приложения, продаёт место под рекламу. Зарабатывает.
- **Advertiser** — компания, покупает показы. Тратит бюджет.
- **SSP (платформа)** — проводит аукцион, берёт комиссию 20%.
- **User** — видит рекламу в приложении Publisher'а.

## Полный цикл

1. **Publisher** регистрирует слот через gRPC API:
   `name=home_banner, width=320, height=50, geo=RU, min_price=10`.

2. **Advertiser** регистрирует кампанию:
   - `name=Nike Summer, budget=10000, geo_target=RU`
   - привязывает рекламные креативы `type=image, width=320, height=50, url=..., click_url=....`

3. **Приложение Publisher'а** шлёт запрос: «дай рекламу для слота X, 
   geo=RU, user_id=u123».

4. **Auction Service**:
   - достаёт слот из PostgreSQL (или Redis, если закэширован);
   - отбирает кампании с подходящим `geo_target` и `budget_remaining > 0`;
   - для каждой кампании берёт её креативы и оставляет только те, у которых width и height совпадают со слотом;
   - параллельно опрашивает биддеров через gRPC

5. **Second-price аукцион**:
   - ставки обрезаются бюджетом: `real_bid = min(want_bid, budget_remaining)`;
   - побеждает максимальная ставка;
   - цена = ставка второго по величине.

6. **Auction Service резервирует** цену в Redis (`SET reservation:{auction_id} 
   EX 30`), уменьшает `budget_reserved` у кампании.

7. Возвращает креатив победителя приложению.

8. **Приложение показывает баннер** и шлёт событие `impression` 
   с `auction_id`.

9. **Auction Service коммитит** (после impression):
   - `advertiser.budget_remaining -= price`
   - `advertiser.budget_reserved -= price`
   - `publisher.revenue_total += price * 0.8`
   - `platform.revenue_total += price * 0.2`
   - удаляет резерв из Redis.

10. **Если impression не пришёл за 30 секунд** — Redis удаляет ключ, 
    фоновая задача откатывает `budget_reserved -= price`. Деньги не двигаются.

11. **Событие** `impression` публикуется в Kafka.

12. **Event Collector** читает из Kafka, пишет в ClickHouse.

13. **Prometheus** собирает метрики со всех сервисов, **Grafana** 
    показывает дашборды.

## Модель данных

**publishers**: `id, name, revenue_total`

**advertisers**: `id, name, budget_total, budget_remaining, budget_reserved`

**ad_slots**: `id, publisher_id, name, width, height, geo, min_price`

**campaigns**: `id, advertiser_id, name, budget_total, budget_remaining, 
budget_reserved, geo_target, creative_url, click_url`

**creatives**: id, campaign_id, type, width, height, url, click_url

**platform_stats**: `id, revenue_total, commission_percent=20`

**События** (ClickHouse): `type, auction_id, slot_id, campaign_id, price, 
publisher_revenue, platform_fee, timestamp`

## События в Kafka

| Топик | Когда | Кто публикует | Кто читает |
|---|---|---|---|
| `auction_won` | Победа в аукционе | Auction Service | Event Collector |
| `impression` | Показ состоялся | Auction Service (после запроса от приложения) | Event Collector |
| `click` | Клик по баннеру | Auction Service | Event Collector |
| `auction_lost` | Проигрыш (опционально) | Auction Service | Event Collector |

Клик логируется в ClickHouse, но денег за него не списываем — только показы.

## Диаграмма

```mermaid
sequenceDiagram
    autonumber
    participant App as Приложение Publisher'а
    participant GW as API Gateway
    participant Auc as Auction Service
    participant R as Redis
    participant DB as PostgreSQL
    participant Bid as Bidder
    participant K as Kafka
    participant C as Event Collector
    participant CH as ClickHouse

    App->>GW: POST /auction {slot_id, geo, user_id}
    GW->>Auc: RunAuction(BidRequest)
    Auc->>R: GET slot:{id}
    alt cache miss
        Auc->>DB: SELECT slot
        Auc->>R: SET slot:{id} TTL
    end
    Auc->>DB: SELECT campaigns WHERE geo=RU AND budget>0
    par Опрос биддеров
        Auc->>Bid: GetBid
        Bid-->>Auc: BidResponse
    and
        Auc->>Bid: GetBid
        Bid-->>Auc: BidResponse
    end
    Note over Auc: Second-price: выбираем победителя
    Auc->>R: SET reservation:{id} EX 30
    Auc->>DB: UPDATE campaign SET budget_reserved += price
    Auc-->>GW: BidResponse{creative, price}
    GW-->>App: 200 OK

    Note over App: Показ состоялся
    App->>GW: POST /impression {auction_id}
    GW->>Auc: CommitImpression
    Auc->>R: DEL reservation:{id}
    Auc->>DB: UPDATE budget_remaining -= price
    Auc->>DB: UPDATE publisher revenue_total += 80%
    Auc->>DB: UPDATE platform revenue_total += 20%
    Auc->>K: publish impression
    K->>C: consume impression
    C->>CH: INSERT event