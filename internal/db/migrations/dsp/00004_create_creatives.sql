-- +goose Up
CREATE TABLE dsp.creatives (
    id           UUID PRIMARY KEY,
    campaign_id  UUID  NOT NULL REFERENCES dsp.campaigns(id) ON DELETE CASCADE,
    type         TEXT  NOT NULL,
    url          TEXT  NOT NULL,
    click_url    TEXT  NOT NULL,

    banner_width  INT,
    banner_height INT,

    video_width    INT,
    video_height   INT,
    video_duration INT,
    video_mimes    TEXT[],

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

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

CREATE INDEX idx_creatives_campaign ON dsp.creatives (campaign_id);

-- +goose Down
DROP TABLE dsp.creatives;