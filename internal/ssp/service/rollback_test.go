package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func TestService_ExpiredAuction_RollsBack(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, _ := newTestService(bidder)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	result, err := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})
	if err != nil {
		t.Fatalf("run auction: %v", err)
	}

	// Помечаем запись как просроченную и триггерим один проход воркера
	// синхронно — без ожидания тикера.
	mgr := mustMemoryReserve(t, svc)
	mgr.ExpireNow(result.AuctionID)
	svc.TickReserve(context.Background())

	bidder.mu.Lock()
	rbID := bidder.capturedRollbackID
	rbPrice := bidder.capturedRollbackPr
	bidder.mu.Unlock()

	if rbID != "camp_nike" {
		t.Errorf("rollback campaign = %q, want camp_nike", rbID)
	}
	if rbPrice != 5_000_000 {
		t.Errorf("rollback price = %d, want 5000000", rbPrice)
	}
	if mgr.Contains(result.AuctionID) {
		t.Error("expired auction should be removed from reserve")
	}
}

func TestService_TickReserve_NoExpired(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	svc, slots, _ := newTestService(bidder)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	_, _ = svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	svc.TickReserve(context.Background())

	bidder.mu.Lock()
	rbID := bidder.capturedRollbackID
	bidder.mu.Unlock()

	if rbID != "" {
		t.Errorf("rollback should not be called, got %q", rbID)
	}
}

func TestService_TickReserve_RollbackFails_RecordRestored(t *testing.T) {
	bidder := testBidder("nike", "camp_nike", 5_000_000)
	bidder.rollbackErr = errors.New("dsp down")

	svc, slots, _ := newTestService(bidder)
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
		RequestID: "req_1",
		SlotID:    "slot_1",
	})

	mgr := mustMemoryReserve(t, svc)
	mgr.ExpireNow(result.AuctionID)
	svc.TickReserve(context.Background())

	// При ошибке rollback запись должна быть возвращена.
	if !mgr.Contains(result.AuctionID) {
		t.Error("auction should be restored on rollback failure")
	}
}

// TestService_StartReserveWorker_StopsOnContext — smoke-тест: воркер
// стартует, затем корректно останавливается по отмене контекста.
//
// Точная проверка остановки воркера — в reserve/memory_test.go
// (TestMemoryManager_Subscribe_StopsOnContext). Здесь важно убедиться,
// что StartReserveWorker не блокирует вызов и не паникует.
func TestService_StartReserveWorker_StopsOnContext(t *testing.T) {
	svc, _, _ := newTestService()

	ctx, cancel := context.WithCancel(context.Background())
	svc.StartReserveWorker(ctx)

	// Даём воркеру стартовать.
	time.Sleep(20 * time.Millisecond)
	cancel()

	// Даём воркеру время увидеть отмену и завершиться.
	time.Sleep(20 * time.Millisecond)

	// Если мы досюда дошли без deadlock и без паники — воркер
	// не блокирует вызывающего и корректно реагирует на cancel.
	t.Log("reserve worker started and stopped on context cancellation")
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

func TestService_ImpressionVsExpiry_Race(t *testing.T) {
	for i := 0; i < 100; i++ {
		bidder := testBidder("nike", "camp_nike", 5_000_000)
		svc, slots, pubs := newTestService(bidder)
		addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))
		addPublisher(t, pubs, testPublisher("pub_1"))

		result, _ := svc.RunAuction(context.Background(), domain.BidRequest{
			RequestID: "req_1",
			SlotID:    "slot_1",
		})

		// Помечаем запись просроченной, чтобы TickReserve её увидел.
		mgr := mustMemoryReserve(t, svc)
		mgr.ExpireNow(result.AuctionID)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); svc.TickReserve(context.Background()) }()
		go func() { defer wg.Done(); _ = svc.Impression(context.Background(), result.AuctionID) }()
		wg.Wait()

		// Инвариант: Commit и Rollback не должны оба произойти.
		bidder.mu.Lock()
		committed := bidder.capturedCommitID != ""
		rolledBack := bidder.capturedRollbackID != ""
		bidder.mu.Unlock()

		if committed && rolledBack {
			t.Fatalf("iter %d: both Commit and Rollback were called", i)
		}
	}
}
