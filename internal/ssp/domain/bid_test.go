package domain

import "testing"

func TestBid_Validate(t *testing.T) {
	validBid := Bid{
		ID:          "bid_1",
		CampaignID:  "camp_1",
		CreativeID:  "cr_1",
		CreativeURL: "https://cdn.example.com/a.jpg",
		ClickURL:    "https://example.com/click",
		Price:       1_500_000,
	}

	tests := []struct {
		name    string
		modify  func(*Bid)
		wantErr bool
	}{
		{"valid", func(*Bid) {}, false},
		{"empty id", func(b *Bid) { b.ID = "" }, true},
		{"whitespace id", func(b *Bid) { b.ID = "   " }, true},
		{"empty campaign id", func(b *Bid) { b.CampaignID = "" }, true},
		{"empty creative id", func(b *Bid) { b.CreativeID = "" }, true},
		{"empty creative url", func(b *Bid) { b.CreativeURL = "" }, true},
		{"invalid creative url", func(b *Bid) { b.CreativeURL = "not-a-url" }, true},
		{"empty click url", func(b *Bid) { b.ClickURL = "" }, true},
		{"zero price", func(b *Bid) { b.Price = 0 }, true},
		{"negative price", func(b *Bid) { b.Price = -1 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := validBid
			tt.modify(&b)
			err := b.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
