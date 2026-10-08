package domain

import "testing"

func validBannerCreative() Creative {
	return Creative{
		ID:         "cr_1",
		CampaignID: "camp_1",
		Type:       CreativeTypeBanner,
		URL:        "https://cdn.example.com/banner.jpg",
		ClickURL:   "https://nike.com/summer",
		Banner:     &Banner{Width: 320, Height: 50},
	}
}

func validVideoCreative() Creative {
	return Creative{
		ID:         "cr_2",
		CampaignID: "camp_1",
		Type:       CreativeTypeVideo,
		URL:        "https://cdn.example.com/video.mp4",
		ClickURL:   "https://nike.com/summer",
		Video: &Video{
			Width:    640,
			Height:   480,
			Duration: 30,
			MIMEs:    []string{"video/mp4"},
		},
	}
}

func TestCreative_Validate(t *testing.T) {
	t.Run("banner creative", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Creative)
			wantErr bool
		}{
			{"valid", func(*Creative) {}, false},
			{"empty id", func(c *Creative) { c.ID = "" }, true},
			{"empty campaign id", func(c *Creative) { c.CampaignID = "" }, true},
			{"invalid url", func(c *Creative) { c.URL = "" }, true},
			{"invalid click url", func(c *Creative) { c.ClickURL = "" }, true},
			{"nil banner", func(c *Creative) { c.Banner = nil }, true},
			{"zero banner width", func(c *Creative) { c.Banner.Width = 0 }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := validBannerCreative()
				tt.modify(&c)
				err := c.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("video creative", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Creative)
			wantErr bool
		}{
			{"valid", func(*Creative) {}, false},
			{"nil video", func(c *Creative) { c.Video = nil }, true},
			{"zero duration", func(c *Creative) { c.Video.Duration = 0 }, true},
			{"empty mimes", func(c *Creative) { c.Video.MIMEs = nil }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := validVideoCreative()
				tt.modify(&c)
				err := c.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		c := validBannerCreative()
		c.Type = "unsupported"
		if err := c.Validate(); err == nil {
			t.Error("want error for unsupported type, got nil")
		}
	})

	t.Run("banner type without banner params", func(t *testing.T) {
		c := validBannerCreative()
		c.Banner = nil
		if err := c.Validate(); err == nil {
			t.Error("want error for banner without params, got nil")
		}
	})

	t.Run("video type without video params", func(t *testing.T) {
		c := validVideoCreative()
		c.Video = nil
		if err := c.Validate(); err == nil {
			t.Error("want error for video without params, got nil")
		}
	})

	t.Run("native type not implemented", func(t *testing.T) {
		c := Creative{
			ID:         "cr_3",
			CampaignID: "camp_1",
			Type:       CreativeTypeNative,
			URL:        "https://cdn.example.com/icon.png",
			ClickURL:   "https://nike.com",
			Native:     &Native{},
		}
		if err := c.Validate(); err == nil {
			t.Error("want error for native (not implemented), got nil")
		}
	})

	t.Run("audio type not implemented", func(t *testing.T) {
		c := Creative{
			ID:         "cr_4",
			CampaignID: "camp_1",
			Type:       CreativeTypeAudio,
			URL:        "https://cdn.example.com/audio.mp3",
			ClickURL:   "https://nike.com",
			Audio:      &Audio{},
		}
		if err := c.Validate(); err == nil {
			t.Error("want error for audio (not implemented), got nil")
		}
	})
}
