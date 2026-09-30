package domain

import "testing"

func TestBidRequest_Validate(t *testing.T) {
	validRequest := BidRequest{
		ID: "req_1",
		Imp: Imp{
			ID:       "imp_1",
			SlotID:   "slot_1",
			Banner:   Banner{Width: 320, Height: 50},
			BidFloor: 1_000_000,
		},
		UserID: "user_1",
	}

	tests := []struct {
		name    string
		modify  func(*BidRequest)
		wantErr bool
	}{
		{"valid", func(*BidRequest) {}, false},
		{"valid without user id", func(r *BidRequest) { r.UserID = "" }, false},
		{"empty id", func(r *BidRequest) { r.ID = "" }, true},
		{"whitespace id", func(r *BidRequest) { r.ID = "   " }, true},
		{"empty imp id", func(r *BidRequest) { r.Imp.ID = "" }, true},
		{"whitespace imp id", func(r *BidRequest) { r.Imp.ID = "   " }, true},
		{"empty imp slot id", func(r *BidRequest) { r.Imp.SlotID = "" }, true},
		{"whitespace imp slot id", func(r *BidRequest) { r.Imp.SlotID = "   " }, true},
		{"invalid imp banner", func(r *BidRequest) { r.Imp.Banner.Width = 0 }, true},
		{"negative imp bid floor", func(r *BidRequest) { r.Imp.BidFloor = -1 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRequest
			tt.modify(&r)
			err := r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
