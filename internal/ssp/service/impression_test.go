package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func TestService_Impression_HappyPath(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, pubs := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))
	pubs.Add(testPublisher("pub_1"))

	// Проводим аукцион.
	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("run auction: %v", err)
	}

	// Отправляем impression.
	if err := svc.Impression(context.Background(), result.AuctionID); err != nil {
		t.Fatalf("impression: %v", err)
	}

	// Проверяем, что коммит ушёл в биддер.
	bidder.mu.Lock()
	if bidder.capturedCommitID != "camp_nike" {
		t.Errorf("committed campaign = %q", bidder.capturedCommitID)
	}
	if bidder.capturedCommitPrice != 5_000_000 {
		t.Errorf("committed price = %d", bidder.capturedCommitPrice)
	}
	bidder.mu.Unlock()

	// Проверяем, что баланс publisher'а пополнен.
	p, _ := pubs.Get(context.Background(), "pub_1")
	// 5_000_000 * (100-20)/100 = 4_000_000
	if p.Balance != 4_000_000 {
		t.Errorf("publisher balance = %d, want 4000000", p.Balance)
	}
}

func TestService_Impression_Idempotent(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, pubs := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))
	pubs.Add(testPublisher("pub_1"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	_ = svc.Impression(context.Background(), result.AuctionID)
	// Повторный вызов — должен быть no-op.
	err := svc.Impression(context.Background(), result.AuctionID)
	if err != nil {
		t.Errorf("second impression should be no-op, got %v", err)
	}

	// Баланс должен быть пополнен ровно один раз.
	p, _ := pubs.Get(context.Background(), "pub_1")
	if p.Balance != 4_000_000 {
		t.Errorf("balance = %d, want 4000000 (charged once)", p.Balance)
	}
}

func TestService_Impression_Concurrent(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, pubs := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))
	pubs.Add(testPublisher("pub_1"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = svc.Impression(context.Background(), result.AuctionID)
		}(i)
	}
	close(start)
	wg.Wait()

	// Все вызовы должны вернуть nil — идемпотентность.
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	// Commit в DSP должен уйти ровно один раз.
	bidder.mu.Lock()
	commits := 0
	if bidder.capturedCommitID != "" {
		commits = 1
	}
	bidder.mu.Unlock()
	if commits != 1 {
		t.Errorf("commit count = %d, want 1", commits)
	}

	// Баланс publisher'а начислен ровно один раз.
	p, _ := pubs.Get(context.Background(), "pub_1")
	if p.Balance != 4_000_000 {
		t.Errorf("balance = %d, want 4000000 (charged once)", p.Balance)
	}
}

func TestService_Impression_UnknownAuction(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.Impression(context.Background(), "unknown-auction")
	if !errors.Is(err, domain.ErrAuctionNotFound) {
		t.Errorf("want ErrAuctionNotFound, got %v", err)
	}
}

func TestService_Impression_CommitFails(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	bidder.commitErr = errors.New("dsp down")

	svc, slots, pubs := newTestService(bidder)
	slots.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))
	pubs.Add(testPublisher("pub_1"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	err := svc.Impression(context.Background(), result.AuctionID)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	// Запись должна вернуться — можно повторить.
	if _, ok := svc.takeAuction(result.AuctionID); !ok {
		t.Error("auction record should be restored for retry")
	}
}

func TestService_GetPublisherBalance(t *testing.T) {
	svc, _, pubs := newTestService()
	p := testPublisher("pub_1")
	p.Balance = 1_234_567
	pubs.Add(p)

	balance, err := svc.GetPublisherBalance(context.Background(), "pub_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 1_234_567 {
		t.Errorf("balance = %d, want 1234567", balance)
	}
}

func TestService_GetPublisherBalance_NotFound(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.GetPublisherBalance(context.Background(), "missing")
	if !errors.Is(err, domain.ErrPublisherNotFound) {
		t.Errorf("want ErrPublisherNotFound, got %v", err)
	}
}
