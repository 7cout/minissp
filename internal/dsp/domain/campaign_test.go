package domain

import "testing"

func TestCampaign_Validate(t *testing.T) {
	validCampaign := Campaign{
		ID:              "camp_1",
		AdvertiserID:    "adv_1",
		Name:            "Nike Summer",
		BudgetTotal:     10_000_000_000,
		BudgetRemaining: 10_000_000_000,
		BudgetReserved:  0,
		GeoTarget:       "RU",
	}

	tests := []struct {
		name    string
		modify  func(*Campaign)
		wantErr bool
	}{
		{"valid", func(*Campaign) {}, false},
		{"empty id", func(c *Campaign) { c.ID = "" }, true},
		{"whitespace id", func(c *Campaign) { c.ID = "   " }, true},
		{"empty advertiser id", func(c *Campaign) { c.AdvertiserID = "" }, true},
		{"whitespace advertiser id", func(c *Campaign) { c.AdvertiserID = "   " }, true},
		{"empty name", func(c *Campaign) { c.Name = "" }, true},
		{"whitespace name", func(c *Campaign) { c.Name = "   " }, true},
		{"negative total", func(c *Campaign) { c.BudgetTotal = -1 }, true},
		{"negative remaining", func(c *Campaign) { c.BudgetRemaining = -1 }, true},
		{"negative reserved", func(c *Campaign) { c.BudgetReserved = -1 }, true},
		{
			name: "budget exceeded",
			modify: func(c *Campaign) {
				c.BudgetRemaining = 6_000_000_000
				c.BudgetReserved = 5_000_000_000
			},
			wantErr: true,
		},
		{
			name: "budget exactly full",
			modify: func(c *Campaign) {
				c.BudgetRemaining = 6_000_000_000
				c.BudgetReserved = 4_000_000_000
			},
			wantErr: false,
		},
		{"invalid geo", func(c *Campaign) { c.GeoTarget = "RUSSIA" }, true},
		{"lowercase geo", func(c *Campaign) { c.GeoTarget = "ru" }, true},
		{"empty geo", func(c *Campaign) { c.GeoTarget = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCampaign
			tt.modify(&c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
