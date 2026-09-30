package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

// --- Fake-репозитории ---

type fakeSlotRepo struct {
	slots map[string]*domain.Slot
	err   error
}

func (f *fakeSlotRepo) Get(_ context.Context, id string) (*domain.Slot, error) {
	if f.err != nil {
		return nil, f.err
	}
	slot, ok := f.slots[id]
	if !ok {
		return nil, domain.ErrSlotNotFound
	}
	return slot, nil
}

type fakeCampaignRepo struct {
	reserveErr error
	reserved   []reserveCall
}

type reserveCall struct {
	campaignID string
	amount     int64
}

func (f *fakeCampaignRepo) ListByGeo(_ context.Context, _ string) ([]domain.Campaign, error) {
	return nil, nil // не используется в RunAuction (пока)
}

func (f *fakeCampaignRepo) Reserve(_ context.Context, campaignID string, amount int64) error {
	if f.reserveErr != nil {
		return f.reserveErr
	}
	f.reserved = append(f.reserved, reserveCall{campaignID, amount})
	return nil
}

// --- Хелперы ---

func testSlot() *domain.Slot {
	return &domain.Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Banner:      domain.Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	}
}

func testRequest() domain.BidRequest {
	return domain.BidRequest{
		ID: "req_1",
		Imp: domain.Imp{
			ID:       "imp_1",
			SlotID:   "slot_1",
			Banner:   domain.Banner{Width: 320, Height: 50},
			BidFloor: 1_000_000,
		},
		UserID: "user_1",
	}
}

// --- Тесты ---

func TestService_RunAuction(t *testing.T) {
	t.Run("happy path — winner selected and budget reserved", func(t *testing.T) {
		slots := &fakeSlotRepo{slots: map[string]*domain.Slot{"slot_1": testSlot()}}
		campaigns := &fakeCampaignRepo{}
		svc := New(
			slots,
			campaigns,
			nil,
			[]BidderClient{
				&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
				&fakeBidder{bid: validBid("b2", "c2", 8_000_000)},
				&fakeBidder{bid: validBid("b3", "c3", 12_000_000)},
			},
		)

		result, err := svc.RunAuction(context.Background(), testRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.CampaignID != "c3" {
			t.Errorf("winner = %s, want c3", result.CampaignID)
		}
		if result.BidID != "b3" {
			t.Errorf("bid = %s, want b3", result.BidID)
		}
		// second-price: победил 12, платит 8
		if result.Price != 8_000_000 {
			t.Errorf("price = %d, want 8000000", result.Price)
		}
		if result.AuctionID == "" {
			t.Error("auction id is empty")
		}

		if len(campaigns.reserved) != 1 {
			t.Fatalf("want 1 reserve call, got %d", len(campaigns.reserved))
		}
		if campaigns.reserved[0].campaignID != "c3" {
			t.Errorf("reserved campaign = %s, want c3", campaigns.reserved[0].campaignID)
		}
		if campaigns.reserved[0].amount != 8_000_000 {
			t.Errorf("reserved amount = %d, want 8000000", campaigns.reserved[0].amount)
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		svc := New(&fakeSlotRepo{}, &fakeCampaignRepo{}, nil, nil)
		req := testRequest()
		req.ID = "" // невалидно

		_, err := svc.RunAuction(context.Background(), req)
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("slot not found", func(t *testing.T) {
		svc := New(&fakeSlotRepo{slots: map[string]*domain.Slot{}}, &fakeCampaignRepo{}, nil, nil)

		_, err := svc.RunAuction(context.Background(), testRequest())
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})

	t.Run("no bids — ErrNoBids", func(t *testing.T) {
		slots := &fakeSlotRepo{slots: map[string]*domain.Slot{"slot_1": testSlot()}}
		svc := New(slots, &fakeCampaignRepo{}, nil, []BidderClient{
			&fakeBidder{err: errors.New("fail 1")},
			&fakeBidder{err: errors.New("fail 2")},
		})

		_, err := svc.RunAuction(context.Background(), testRequest())
		if !errors.Is(err, domain.ErrNoBids) {
			t.Errorf("want ErrNoBids, got %v", err)
		}
	})

	t.Run("reserve fails — insufficient budget", func(t *testing.T) {
		slots := &fakeSlotRepo{slots: map[string]*domain.Slot{"slot_1": testSlot()}}
		campaigns := &fakeCampaignRepo{reserveErr: domain.ErrInsufficientBudget}
		svc := New(slots, campaigns, nil, []BidderClient{
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
		})

		_, err := svc.RunAuction(context.Background(), testRequest())
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})

	t.Run("single bidder — pays bid floor", func(t *testing.T) {
		slots := &fakeSlotRepo{slots: map[string]*domain.Slot{"slot_1": testSlot()}}
		campaigns := &fakeCampaignRepo{}
		svc := New(slots, campaigns, nil, []BidderClient{
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
		})

		result, err := svc.RunAuction(context.Background(), testRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Один биддер — платит bidfloor (1_000_000)
		if result.Price != 1_000_000 {
			t.Errorf("price = %d, want 1000000 (bid floor)", result.Price)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		slots := &fakeSlotRepo{slots: map[string]*domain.Slot{"slot_1": testSlot()}}
		svc := New(slots, &fakeCampaignRepo{}, nil, []BidderClient{
			&fakeBidder{bid: validBid("b1", "c1", 5_000_000)},
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := svc.RunAuction(ctx, testRequest())
		if err == nil {
			t.Fatal("want error on cancelled context")
		}
	})
}
