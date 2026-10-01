# 006. Трёхсервисная архитектура (SSP + DSP + Publisher)

- **Статус:** Принято
- **Дата:** 2026-10-01

## Контекст

Изначально был один сервис с симулятором биддера внутри.
Это не соответствует реальному RTB: SSP и DSP — разные
платформы с разными данными и ответственностями.
Запутался в процессе реализации и тестирования.
Поулчался слишком перегруженный сервис.

## Решение

Три сервиса, общение по gRPC:

**Publisher Service (эмулятор площадки)**
- Генерирует запросы на показ.
- Получает креатив, «показывает» его.
- Шлёт событие impression.
- Свой "баланс" - хранит на стороне SSP.

**SSP Service (наш аукцион)**
- Владеет слотами (ad_slots) и издателями (publishers).
- Проводит аукцион second-price.
- Хранит publishers.balance и ssp_revenue.
- Инициирует Commit в DSP по impression.

**DSP Service (биддер)**
- Владеет кампаниями, креативами, бюджетами.
- Решает, участвовать ли в аукционе.
- Резервирует бюджет при ставке, списывает при Commit.
- Хранит advertiser.balance и campaign.budget_*.

## Поток одного показа

1. Publisher → SSP: RunAuction(slot_id)
2. SSP → DSP: GetBid(slot info)
3. DSP: выбирает кампанию, Reserve, отвечает
4. SSP: выбирает победителя
5. SSP → Publisher: креатив
6. Publisher → SSP: Impression(auction_id)
7. SSP → DSP: Commit(auction_id)
8. DSP: списывает бюджет
9. SSP: publisher.balance += price*0.8, ssp_revenue += price*0.2