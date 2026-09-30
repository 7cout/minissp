package domain

import "testing"

func TestCreative_Validate(t *testing.T) {
	validCreative := Creative{
		ID:         "creative_1",
		CampaignID: "camp_1",
		Banner:     Banner{Width: 320, Height: 50},
		URL:        "https://cdn.example.com/banner.jpg",
		ClickURL:   "https://nike.com/summer",
	}

	tests := []struct {
		name    string
		modify  func(*Creative)
		wantErr bool
	}{
		{"valid", func(*Creative) {}, false},
		{"empty id", func(c *Creative) { c.ID = "" }, true},
		{"whitespace id", func(c *Creative) { c.ID = "   " }, true},
		{"empty campaign id", func(c *Creative) { c.CampaignID = "" }, true},
		{"whitespace campaign id", func(c *Creative) { c.CampaignID = "   " }, true},
		{"invalid banner", func(c *Creative) { c.Banner.Width = 0 }, true},
		{"empty url", func(c *Creative) { c.URL = "" }, true},
		{"empty click url", func(c *Creative) { c.ClickURL = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCreative
			tt.modify(&c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
