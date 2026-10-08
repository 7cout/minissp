-- +goose Up
CREATE TABLE dsp.advertisers (
    id         UUID PRIMARY KEY,
    name       TEXT        NOT NULL,
    balance    BIGINT      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE dsp.advertisers;