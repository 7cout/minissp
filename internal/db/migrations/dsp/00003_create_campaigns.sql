-- +goose Up
CREATE TABLE dsp.campaigns (
    id               UUID PRIMARY KEY,
    advertiser_id    UUID        NOT NULL REFERENCES dsp.advertisers(id) ON DELETE CASCADE,
    name             TEXT        NOT NULL,
    budget_total     BIGINT      NOT NULL CHECK (budget_total >= 0),
    budget_remaining BIGINT      NOT NULL CHECK (budget_remaining >= 0),
    budget_reserved  BIGINT      NOT NULL DEFAULT 0 CHECK (budget_reserved >= 0),
    geo_target       VARCHAR(2)  NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (budget_remaining + budget_reserved <= budget_total)
);

CREATE INDEX idx_campaigns_geo_with_budget
    ON dsp.campaigns (geo_target)
    WHERE budget_remaining > 0;

CREATE INDEX idx_campaigns_advertiser ON dsp.campaigns (advertiser_id);

-- +goose Down
DROP TABLE dsp.campaigns;