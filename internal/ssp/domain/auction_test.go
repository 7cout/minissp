package domain

import (
	"testing"
	"time"
)

func TestBidRequest_Validate(t *testing.T) {
	validReq := BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
		UserID:    "user_1",
	}

	tests := []struct {
		name    string
		modify  func(*BidRequest)
		wantErr bool
	}{
		{"valid", func(*BidRequest) {}, false},
		{"valid without user id", func(r *BidRequest) { r.UserID = "" }, false},
		{"empty request id", func(r *BidRequest) { r.RequestID = "" }, true},
		{"whitespace request id", func(r *BidRequest) { r.RequestID = "   " }, true},
		{"empty slot id", func(r *BidRequest) { r.SlotID = "" }, true},
		{"whitespace slot id", func(r *BidRequest) { r.SlotID = "   " }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validReq
			tt.modify(&r)
			err := r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuctionRecord_Validate(t *testing.T) {
	validRecord := AuctionRecord{
		AuctionID:   "a_1",
		ImpID:       "imp_1",
		CampaignID:  "camp_1",
		CreativeID:  "cr_1",
		PublisherID: "pub_1",
		SlotID:      "slot_1",
		BidderID:    "nike",
		Price:       1_500_000,
		CreatedAt:   time.Now(),
	}

	tests := []struct {
		name    string
		modify  func(*AuctionRecord)
		wantErr bool
	}{
		{"valid", func(*AuctionRecord) {}, false},
		{"empty auction id", func(r *AuctionRecord) { r.AuctionID = "" }, true},
		{"whitespace auction id", func(r *AuctionRecord) { r.AuctionID = "   " }, true},
		{"empty campaign id", func(r *AuctionRecord) { r.CampaignID = "" }, true},
		{"empty publisher id", func(r *AuctionRecord) { r.PublisherID = "" }, true},
		{"zero price", func(r *AuctionRecord) { r.Price = 0 }, true},
		{"negative price", func(r *AuctionRecord) { r.Price = -1 }, true},
		{"empty bidder id", func(r *AuctionRecord) { r.BidderID = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord
			tt.modify(&r)
			err := r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
