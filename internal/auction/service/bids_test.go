package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/auction/domain"
)

// fakeBidder - клиент биддера для тестов
type fakeBidder struct {
	bid   *domain.Bid
	err   error
	delay time.Duration
}

func (f *fakeBidder) GetBid(ctx context.Context, _ domain.BidRequest) (*domain.Bid, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.bid, nil
}

func newTestService(bidders ...BidderClient) *Service {
	return New(nil, nil, nil, bidders)
}

func validBid(id, campaignID string, price int64) *domain.Bid {
	return &domain.Bid{
		ID:         id,
		ImpID:      "imp_1",
		CampaignID: campaignID,
		CreativeID: "creative_1",
		Price:      price,
	}
}

func TestService_collectBids(t *testing.T) {
	req := domain.BidRequest{
		ID: "req_1",
		Imp: domain.Imp{
			ID:       "imp_1",
			SlotID:   "slot_1",
			Banner:   domain.Banner{Width: 320, Height: 50},
			BidFloor: 1_000_000,
		},
	}

	t.Run("no bidders", func(t *testing.T) {
		svc := newTestService()
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 0 {
			t.Errorf("want 0 bids, got %d", len(bids))
		}
	})

	t.Run("one valid bidder", func(t *testing.T) {
		svc := newTestService(&fakeBidder{bid: validBid("b1", "c1", 5_000_000)})
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 1 {
			t.Fatalf("want 1 bid, got %d", len(bids))
		}
		if bids[0].ID != "b1" {
			t.Errorf("want bid b1, got %s", bids[0].ID)
		}
	})

	t.Run("multiple valid bidders", func(t *testing.T) {
		svc := newTestService(
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
			&fakeBidder{bid: validBid("b2", "c2", 8_000_000)},
			&fakeBidder{bid: validBid("b3", "c3", 12_000_000)},
		)
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 3 {
			t.Errorf("want 3 bids, got %d", len(bids))
		}
	})

	t.Run("one bidder fails, others continue", func(t *testing.T) {
		svc := newTestService(
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
			&fakeBidder{err: errors.New("connection refused")},
			&fakeBidder{bid: validBid("b3", "c3", 12_000_000)},
		)
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 2 {
			t.Errorf("want 2 bids (one failed), got %d", len(bids))
		}
	})

	t.Run("all bidders fail — empty result, no error", func(t *testing.T) {
		svc := newTestService(
			&fakeBidder{err: errors.New("fail 1")},
			&fakeBidder{err: errors.New("fail 2")},
		)
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 0 {
			t.Errorf("want 0 bids, got %d", len(bids))
		}
	})

	t.Run("invalid bid is filtered out", func(t *testing.T) {
		invalidBid := validBid("b1", "c1", 5_000_000)
		invalidBid.Price = 0 // невалидная цена

		svc := newTestService(
			&fakeBidder{bid: invalidBid},
			&fakeBidder{bid: validBid("b2", "c2", 8_000_000)},
		)
		bids, err := svc.collectBids(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 1 {
			t.Fatalf("want 1 valid bid, got %d", len(bids))
		}
		if bids[0].ID != "b2" {
			t.Errorf("want b2, got %s", bids[0].ID)
		}
	})

	t.Run("slow bidder is cut off by timeout", func(t *testing.T) {
		svc := newTestService(
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
			&fakeBidder{
				bid:   validBid("b2", "c2", 8_000_000),
				delay: 500 * time.Millisecond, // больше bidTimeout (100ms)
			},
		)

		start := time.Now()
		bids, err := svc.collectBids(context.Background(), req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bids) != 1 {
			t.Fatalf("want 1 bid (slow cut off), got %d", len(bids))
		}
		if bids[0].ID != "b1" {
			t.Errorf("want b1, got %s", bids[0].ID)
		}
		// Проверяем, что не ждали все 500мс — таймаут сработал
		if elapsed > 300*time.Millisecond {
			t.Errorf("collectBids took %v, expected ~100ms (timeout)", elapsed)
		}
	})

	t.Run("parent context cancelled during work", func(t *testing.T) {
		svc := newTestService(
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000), delay: 50 * time.Millisecond},
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := svc.collectBids(ctx, req)
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		svc := newTestService(&fakeBidder{bid: validBid("b1", "c1", 5_000_000)})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // отменяем сразу

		_, err := svc.collectBids(ctx, req)
		if err == nil {
			t.Fatal("want error on cancelled context, got nil")
		}
	})
}
