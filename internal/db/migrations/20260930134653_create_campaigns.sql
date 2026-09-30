-- +goose Up
CREATE TABLE campaigns (
    id               UUID PRIMARY KEY,
    advertiser_id    UUID       NOT NULL,
    name             TEXT       NOT NULL,
    budget_total     BIGINT     NOT NULL CHECK (budget_total >= 0),
    budget_remaining BIGINT     NOT NULL CHECK (budget_remaining >= 0),
    budget_reserved  BIGINT     NOT NULL DEFAULT 0 CHECK (budget_reserved >= 0),
    geo_target       VARCHAR(2) NOT NULL,

    -- Ключевой инвариант: сумма не должна превышать общий бюджет.
    CHECK (budget_remaining + budget_reserved <= budget_total)
);

-- Индекс для быстрого поиска кампаний по гео.
CREATE INDEX idx_campaigns_geo ON campaigns (geo_target);

-- +goose Down
DROP TABLE campaigns;