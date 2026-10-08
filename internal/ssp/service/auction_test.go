package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func TestService_RunAuction_HappyPath_SecondPrice(t *testing.T) {
	// Три биддера: 12, 8, 5. Победитель — 12, цена — 8 (вторая).
	svc, slots, _ := newTestService(
		testBidder("nike", "camp_nike", 12_000_000),
		testBidder("adidas", "camp_adidas", 8_000_000),
		testBidder("puma", "camp_puma", 5_000_000),
	)
	slot := testBannerSlot("slot_1", "pub_1", "home_banner")
	addSlot(t, slots, slot)

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AuctionID == "" {
		t.Error("auction id is empty")
	}
	if result.CreativeURL != "https://cdn.example.com/nike.jpg" {
		t.Errorf("creative url = %q, want nike", result.CreativeURL)
	}

	// Проверяем сохранённую запись.
	rec := peekRecord(t, svc, result.AuctionID)
	if rec.CampaignID != "camp_nike" {
		t.Errorf("campaign = %q, want camp_nike", rec.CampaignID)
	}
	if rec.BidderID != "nike" {
		t.Errorf("bidder = %q, want nike", rec.BidderID)
	}
	if rec.Price != 8_000_000 {
		t.Errorf("price = %d, want 8000000 (second)", rec.Price)
	}

	if rec.ImpID == "" {
		t.Error("imp id should be set in auction record")
	}
}

func TestService_RunAuction_SingleBid_UsesFloor(t *testing.T) {
	svc, slots, _ := newTestService(
		testBidder("nike", "camp_nike", 500_000), // ставка ниже floor
	)
	slot := testBannerSlot("slot_1", "pub_1", "home_banner")
	addSlot(t, slots, slot)

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := peekRecord(t, svc, result.AuctionID)
	if rec.Price != slot.MinPrice {
		t.Errorf("price = %d, want floor=%d", rec.Price, slot.MinPrice)
	}
}

func TestService_RunAuction_TwoBids_SecondBelowFloor(t *testing.T) {
	// Победитель 5млн, второй — 500к. Floor — 1млн. Цена должна = 1млн.
	svc, slots, _ := newTestService(
		testBidder("nike", "camp_nike", 5_000_000),
		testBidder("adidas", "camp_adidas", 500_000),
	)
	slot := testBannerSlot("slot_1", "pub_1", "home_banner")
	addSlot(t, slots, slot)

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := peekRecord(t, svc, result.AuctionID)
	if rec.Price != slot.MinPrice {
		t.Errorf("price = %d, want floor=%d", rec.Price, slot.MinPrice)
	}
}

func TestService_RunAuction_EqualBids_FirstWins(t *testing.T) {
	svc, slots, _ := newTestService(
		testBidder("nike", "camp_nike", 5_000_000),
		testBidder("adidas", "camp_adidas", 5_000_000),
	)
	slot := testBannerSlot("slot_1", "pub_1", "home_banner")
	addSlot(t, slots, slot)

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := peekRecord(t, svc, result.AuctionID)
	if rec.Price != 5_000_000 {
		t.Errorf("price = %d, want 5000000", rec.Price)
	}
}

func TestService_RunAuction_NoBids(t *testing.T) {
	svc, slots, _ := newTestService(
		&fakeBidder{name: "nike", bidErr: domain.ErrNoBids},
		&fakeBidder{name: "adidas", bidErr: domain.ErrNoBids},
	)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	_, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if !errors.Is(err, domain.ErrNoBids) {
		t.Errorf("want ErrNoBids, got %v", err)
	}
}

func TestService_RunAuction_AllBiddersFail(t *testing.T) {
	svc, slots, _ := newTestService(
		&fakeBidder{name: "nike", bidErr: errors.New("connection refused")},
		&fakeBidder{name: "adidas", bidErr: errors.New("timeout")},
	)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	_, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if !errors.Is(err, domain.ErrNoBids) {
		t.Errorf("want ErrNoBids, got %v", err)
	}
}

func TestService_RunAuction_OneFailsOthersSucceed(t *testing.T) {
	svc, slots, _ := newTestService(
		&fakeBidder{name: "nike", bidErr: errors.New("boom")},
		testBidder("adidas", "camp_adidas", 5_000_000),
	)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := peekRecord(t, svc, result.AuctionID)
	if rec.BidderID != "adidas" {
		t.Errorf("bidder = %q, want adidas", rec.BidderID)
	}
}

func TestService_RunAuction_SlotNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "missing",
	})
	if !errors.Is(err, domain.ErrSlotNotFound) {
		t.Errorf("want ErrSlotNotFound, got %v", err)
	}
}

func TestService_RunAuction_InvalidRequest(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "",
		SlotID:    "slot_1",
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestService_RunAuction_ContextCancelled(t *testing.T) {
	svc, slots, _ := newTestService(
		&fakeBidder{
			name:     "nike",
			bid:      testBidder("nike", "camp_nike", 5_000_000).bid,
			bidDelay: 100 * time.Millisecond,
		},
	)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.RunAuction(ctx, domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

func TestService_RunAuction_BidderReceivesSlotParams(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, _ := newTestService(bidder)
	slot := testBannerSlot("slot_1", "pub_1", "home_banner")
	addSlot(t, slots, slot)

	_, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
		UserID:    "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bidder.mu.Lock()
	defer bidder.mu.Unlock()
	if bidder.capturedGetBidReq.RequestID != "req_1" {
		t.Errorf("request id = %q", bidder.capturedGetBidReq.RequestID)
	}
	if bidder.capturedGetBidReq.UserID != "user_1" {
		t.Errorf("user id = %q", bidder.capturedGetBidReq.UserID)
	}
	if bidder.capturedGetBidSlot == nil {
		t.Fatal("slot not passed")
	}
	if bidder.capturedGetBidSlot.ID != "slot_1" {
		t.Errorf("slot id = %q", bidder.capturedGetBidSlot.ID)
	}
	if bidder.capturedGetBidSlot.Geo != "RU" {
		t.Errorf("slot geo = %q", bidder.capturedGetBidSlot.Geo)
	}
}
