-- +goose Up
CREATE TABLE ssp.ad_slots (
    id             UUID PRIMARY KEY,
    publisher_id   UUID        NOT NULL REFERENCES ssp.publishers(id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    geo            VARCHAR(2)  NOT NULL,
    min_price      BIGINT      NOT NULL DEFAULT 0 CHECK (min_price >= 0),
    type           TEXT        NOT NULL,

    -- Параметры баннера (для type = 'banner').
    banner_width   INT,
    banner_height  INT,

    -- Параметры видео (для type = 'video').
    video_width    INT,
    video_height   INT,
    video_duration INT,
    video_mimes    TEXT[],

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (publisher_id, name),

    CHECK (
        type <> 'banner' OR
        (banner_width  IS NOT NULL AND banner_width  > 0 AND
         banner_height IS NOT NULL AND banner_height > 0)
    ),

    CHECK (
        type <> 'video' OR
        (video_width    IS NOT NULL AND video_width    > 0 AND
         video_height   IS NOT NULL AND video_height   > 0 AND
         video_duration IS NOT NULL AND video_duration > 0 AND
         video_mimes    IS NOT NULL AND array_length(video_mimes, 1) > 0)
    )
);

CREATE INDEX idx_ad_slots_publisher ON ssp.ad_slots (publisher_id);

-- +goose Down
DROP TABLE ssp.ad_slots;