-- +goose Up
CREATE TABLE creatives (
    id          UUID PRIMARY KEY,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    width       INT  NOT NULL CHECK (width > 0),
    height      INT  NOT NULL CHECK (height > 0),
    url         TEXT NOT NULL,
    click_url   TEXT NOT NULL
);

-- Индекс для поиска креативов по кампании.
CREATE INDEX idx_creatives_campaign ON creatives (campaign_id);

-- +goose Down
DROP TABLE creatives;