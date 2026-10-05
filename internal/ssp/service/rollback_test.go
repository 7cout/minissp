package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func TestService_processExpired_HappyPath(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, _ := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	// Искусственно делаем запись «старой».
	svc.mu.Lock()
	rec := svc.auctions[result.AuctionID]
	rec.CreatedAt = time.Now().Add(-time.Minute)
	svc.auctions[result.AuctionID] = rec
	svc.mu.Unlock()

	svc.processExpired(context.Background())

	// Rollback должен быть отправлен.
	bidder.mu.Lock()
	rbID := bidder.capturedRollbackID
	rbPrice := bidder.capturedRollbackPr
	bidder.mu.Unlock()

	if rbID != "camp_nike" {
		t.Errorf("rollback campaign = %q, want camp_nike", rbID)
	}
	if rbPrice != 5_000_000 {
		t.Errorf("rollback price = %d", rbPrice)
	}

	// Запись должна быть удалена.
	if _, ok := svc.takeAuction(result.AuctionID); ok {
		t.Error("expired auction should be removed")
	}
}

func TestService_processExpired_NoExpired(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, _ := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

	_, _ = svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	svc.processExpired(context.Background())

	// Rollback не должен быть отправлен.
	bidder.mu.Lock()
	rbID := bidder.capturedRollbackID
	bidder.mu.Unlock()

	if rbID != "" {
		t.Errorf("rollback should not be called, got %q", rbID)
	}
}

func TestService_processExpired_RollbackFails_RecordRestored(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	bidder.rollbackErr = errors.New("dsp down")

	svc, slots, _ := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	svc.mu.Lock()
	rec := svc.auctions[result.AuctionID]
	rec.CreatedAt = time.Now().Add(-time.Minute)
	svc.auctions[result.AuctionID] = rec
	svc.mu.Unlock()

	svc.processExpired(context.Background())

	// При ошибке rollback запись должна быть возвращена.
	if _, ok := svc.takeAuction(result.AuctionID); !ok {
		t.Error("auction should be restored on rollback failure")
	}
}

func TestService_StartRollbackWorker_StopsOnContext(t *testing.T) {
	svc, _, _ := newTestService()

	ctx, cancel := context.WithCancel(context.Background())
	svc.StartRollbackWorker(ctx, 10*time.Millisecond)

	time.Sleep(30 * time.Millisecond)
	cancel()

	// Ждём, пока воркер завершится. Если не завершится — тест
	// упадёт по таймауту, что тоже сигнал о проблеме.
	done := make(chan struct{})
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Всё хорошо — воркер не завис.
	case <-time.After(1 * time.Second):
		t.Fatal("rollback worker did not stop after context cancellation")
	}
}

func TestService_Close(t *testing.T) {
	b1 := testBidder("nike", "camp_nike", 5_000_000)
	b2 := testBidder("adidas", "camp_adidas", 5_000_000)
	svc, _, _ := newTestService(b1, b2)

	if err := svc.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !b1.closed {
		t.Error("b1 not closed")
	}
	if !b2.closed {
		t.Error("b2 not closed")
	}
}
