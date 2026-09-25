package domain

import "testing"

func TestBid_Validate(t *testing.T) {
	validBid := Bid{
		ID:         "bid_1",
		ImpID:      "imp_1",
		CampaignID: "camp_1",
		CreativeID: "creative_1",
		Price:      5_000_000,
	}

	tests := []struct {
		name    string
		modify  func(*Bid)
		wantErr bool
	}{
		{"valid", func(*Bid) {}, false},
		{"empty id", func(b *Bid) { b.ID = "" }, true},
		{"whitespace id", func(b *Bid) { b.ID = "   " }, true},
		{"empty imp id", func(b *Bid) { b.ImpID = "" }, true},
		{"whitespace imp id", func(b *Bid) { b.ImpID = "   " }, true},
		{"empty campaign id", func(b *Bid) { b.CampaignID = "" }, true},
		{"whitespace campaign id", func(b *Bid) { b.CampaignID = "   " }, true},
		{"empty creative id", func(b *Bid) { b.CreativeID = "" }, true},
		{"whitespace creative id", func(b *Bid) { b.CreativeID = "   " }, true},
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
