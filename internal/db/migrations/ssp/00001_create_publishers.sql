-- +goose Up
CREATE TABLE publishers (
    id         UUID PRIMARY KEY,
    name       TEXT        NOT NULL,
    api_key    TEXT        NOT NULL UNIQUE,
    balance    BIGINT      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_publishers_api_key ON publishers (api_key);

-- +goose Down
DROP TABLE publishers;