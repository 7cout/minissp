-- +goose Up
CREATE TABLE ad_slots (
    id           UUID PRIMARY KEY,
    publisher_id UUID        NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    geo          VARCHAR(2)  NOT NULL,
    min_price    BIGINT      NOT NULL DEFAULT 0 CHECK (min_price >= 0),
    type         TEXT        NOT NULL,
    params       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Идемпотентность RegisterSlot.
    UNIQUE (publisher_id, name)
);

CREATE INDEX idx_ad_slots_publisher ON ad_slots (publisher_id);

-- +goose Down
DROP TABLE ad_slots;