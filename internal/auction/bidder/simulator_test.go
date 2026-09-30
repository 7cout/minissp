package bidder

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func testRequest(bidFloor int64) domain.BidRequest {
	return domain.BidRequest{
		ID: "req_1",
		Imp: domain.Imp{
			ID:       "imp_1",
			SlotID:   "slot_1",
			Banner:   domain.Banner{Width: 320, Height: 50},
			BidFloor: bidFloor,
		},
	}
}

func TestSimulator_GetBid(t *testing.T) {
	t.Run("returns bid within range", func(t *testing.T) {
		s := NewSimulator("camp_1", 1_000_000, 5_000_000)

		bid, err := s.GetBid(context.Background(), testRequest(500_000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if bid.Price < 1_000_000 || bid.Price > 5_000_000 {
			t.Errorf("price %d out of range [1000000, 5000000]", bid.Price)
		}
		if bid.CampaignID != "camp_1" {
			t.Errorf("campaign = %q, want camp_1", bid.CampaignID)
		}
		if bid.ImpID != "imp_1" {
			t.Errorf("imp id = %q, want imp_1", bid.ImpID)
		}
		if bid.ID == "" {
			t.Error("bid id is empty")
		}
		if bid.CreativeID == "" {
			t.Error("creative id is empty")
		}
	})

	t.Run("returns error when price below bid floor", func(t *testing.T) {
		s := NewSimulator("camp_1", 100, 200) // диапазон ниже floor

		_, err := s.GetBid(context.Background(), testRequest(1_000_000))
		if !errors.Is(err, domain.ErrNoBids) {
			t.Errorf("want ErrNoBids, got %v", err)
		}
	})

	t.Run("returns error when context is cancelled", func(t *testing.T) {
		s := NewSimulator("camp_1", 1_000_000, 5_000_000)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := s.GetBid(ctx, testRequest(500_000))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("want context.Canceled, got %v", err)
		}
	})

	t.Run("bid is valid domain object", func(t *testing.T) {
		s := NewSimulator("camp_1", 1_000_000, 5_000_000)

		bid, err := s.GetBid(context.Background(), testRequest(500_000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := bid.Validate(); err != nil {
			t.Errorf("bid is not valid: %v", err)
		}
	})

	t.Run("price at exactly bid floor is accepted", func(t *testing.T) {
		// minPrice == maxPrice == bidFloor — детерминированно.
		const floor int64 = 1_000_000
		s := NewSimulator("camp_1", floor, floor)

		bid, err := s.GetBid(context.Background(), testRequest(floor))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bid.Price != floor {
			t.Errorf("price = %d, want %d", bid.Price, floor)
		}
	})
}
