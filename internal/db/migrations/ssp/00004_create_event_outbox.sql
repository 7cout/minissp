-- +goose Up
CREATE TABLE ssp.event_outbox (
    id           UUID PRIMARY KEY,
    topic        TEXT        NOT NULL,
    event_key    TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts     INT         NOT NULL DEFAULT 0,
    last_error   TEXT
);

-- Индекс для воркера: он ищет только неопубликованные события
-- в порядке появления. Partial-индекс — на порядок меньше.
CREATE INDEX idx_event_outbox_unpublished
    ON ssp.event_outbox (created_at)
    WHERE published_at IS NULL;

-- +goose Down
DROP TABLE ssp.event_outbox;