package service

import (
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func TestSelectWinner(t *testing.T) {
	const bidFloor = 1_000_000

	tests := []struct {
		name         string
		bids         []domain.Bid
		wantWinnerID string
		wantPrice    int64
		wantErr      error
	}{
		{
			name:         "empty bids",
			bids:         []domain.Bid{},
			wantWinnerID: "",
			wantPrice:    0,
			wantErr:      domain.ErrNoBids,
		},
		{
			name: "single bid",
			bids: []domain.Bid{
				{ID: "b1", Price: 5_000_000},
			},
			wantWinnerID: "b1",
			wantPrice:    bidFloor,
		},
		{
			name: "two bids, different prices",
			bids: []domain.Bid{
				{ID: "b1", Price: 5_000_000},
				{ID: "b2", Price: 8_000_000},
			},
			wantWinnerID: "b2",
			wantPrice:    5_000_000,
		},
		{
			name: "three bids, second-price",
			bids: []domain.Bid{
				{ID: "b1", Price: 5_000_000},
				{ID: "b2", Price: 8_000_000},
				{ID: "b3", Price: 12_000_000},
			},
			wantWinnerID: "b3",
			wantPrice:    8_000_000,
		},
		{
			name: "two equal bids — first wins",
			bids: []domain.Bid{
				{ID: "b1", Price: 10_000_000},
				{ID: "b2", Price: 10_000_000},
			},
			wantWinnerID: "b1",
			wantPrice:    10_000_000,
		},
		{
			name: "three bids, two equal at top",
			bids: []domain.Bid{
				{ID: "b1", Price: 8_000_000},
				{ID: "b2", Price: 10_000_000},
				{ID: "b3", Price: 10_000_000},
			},
			wantWinnerID: "b2",
			wantPrice:    10_000_000,
		},
		{
			name: "bid below bid floor",
			bids: []domain.Bid{
				{ID: "b1", Price: 500_000},
			},
			wantWinnerID: "b1",
			wantPrice:    bidFloor,
		},
		{
			name: "bid equal to bid floor",
			bids: []domain.Bid{
				{ID: "b1", Price: bidFloor},
			},
			wantWinnerID: "b1",
			wantPrice:    bidFloor,
		},

		{
			name: "second bid below floor",
			bids: []domain.Bid{
				{ID: "b1", Price: 5_000_000},
				{ID: "b2", Price: 500_000},
			},
			wantWinnerID: "b1",
			wantPrice:    bidFloor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner, price, err := selectWinner(tt.bids, bidFloor)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("want error %v, got nil", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if winner.ID != tt.wantWinnerID {
				t.Errorf("winner.ID = %q, want %q", winner.ID, tt.wantWinnerID)
			}
			if price != tt.wantPrice {
				t.Errorf("price = %d, want %d", price, tt.wantPrice)
			}
		})
	}
}
