-- +goose Up
CREATE TABLE ad_slots (
    id           UUID PRIMARY KEY,
    publisher_id UUID        NOT NULL,
    name         TEXT        NOT NULL,
    width        INT         NOT NULL CHECK (width > 0),
    height       INT         NOT NULL CHECK (height > 0),
    geo          VARCHAR(2)  NOT NULL,
    min_price    BIGINT      NOT NULL DEFAULT 0 CHECK (min_price >= 0)
);

-- +goose Down
DROP TABLE ad_slots;