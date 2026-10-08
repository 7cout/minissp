package reserve

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func testRecord(id string) domain.AuctionRecord {
	return domain.AuctionRecord{
		AuctionID:   id,
		ImpID:       "imp_" + id,
		CampaignID:  "camp_" + id,
		CreativeID:  "cr_" + id,
		PublisherID: "pub_1",
		SlotID:      "slot_1",
		BidderID:    "nike",
		Price:       1_500_000,
		CreatedAt:   time.Now(),
	}
}

func TestMemoryManager_Consume_Fresh(t *testing.T) {
	m := NewMemory(DefaultMemoryOptions())
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))

	rec, status, err := m.Consume(ctx, "a_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ConsumeFresh {
		t.Errorf("status = %v, want ConsumeFresh", status)
	}
	if rec.AuctionID != "a_1" {
		t.Errorf("id = %q, want a_1", rec.AuctionID)
	}
	if m.Contains("a_1") {
		t.Error("record should be removed from reserve after Consume")
	}
	if !m.IsProcessed("a_1") {
		t.Error("record should be marked as processed")
	}
}

func TestMemoryManager_Consume_AlreadyProcessed(t *testing.T) {
	m := NewMemory(DefaultMemoryOptions())
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))
	_, _, _ = m.Consume(ctx, "a_1")

	_, status, err := m.Consume(ctx, "a_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ConsumeAlready {
		t.Errorf("status = %v, want ConsumeAlready", status)
	}
}

func TestMemoryManager_Consume_NotFound(t *testing.T) {
	m := NewMemory(DefaultMemoryOptions())

	_, _, err := m.Consume(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestMemoryManager_Restore(t *testing.T) {
	m := NewMemory(DefaultMemoryOptions())
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))
	rec, _, _ := m.Consume(ctx, "a_1")

	if err := m.Restore(ctx, rec); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if !m.Contains("a_1") {
		t.Error("record should be back in reserve")
	}
	if m.IsProcessed("a_1") {
		t.Error("processed mark should be removed")
	}
}

func TestMemoryManager_Peek(t *testing.T) {
	t.Run("found — returns record without removing it", func(t *testing.T) {
		m := NewMemory(DefaultMemoryOptions())
		ctx := context.Background()

		_ = m.Reserve(ctx, testRecord("a_1"))

		rec, err := m.Peek(ctx, "a_1")
		if err != nil {
			t.Fatalf("peek: %v", err)
		}
		if rec.AuctionID != "a_1" {
			t.Errorf("id = %q, want a_1", rec.AuctionID)
		}
		// Peek не должен изымать запись.
		if !m.Contains("a_1") {
			t.Error("record should still be in reserve after Peek")
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := NewMemory(DefaultMemoryOptions())

		_, err := m.Peek(context.Background(), "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("processed record is not visible", func(t *testing.T) {
		m := NewMemory(DefaultMemoryOptions())
		ctx := context.Background()

		_ = m.Reserve(ctx, testRecord("a_1"))
		_, _, _ = m.Consume(ctx, "a_1")

		_, err := m.Peek(ctx, "a_1")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("want ErrNotFound after Consume, got %v", err)
		}
	})
}

func TestMemoryManager_Tick_ExpiresOldRecord(t *testing.T) {
	m := NewMemory(MemoryOptions{
		TTL:          50 * time.Millisecond,
		ProcessedTTL: time.Minute,
		ScanInterval: time.Millisecond,
	})
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))

	// Не просрочено — callback не вызывается.
	var called []domain.AuctionRecord
	m.Tick(ctx, func(rec domain.AuctionRecord) error {
		called = append(called, rec)
		return nil
	})
	if len(called) != 0 {
		t.Errorf("no record should be expired yet, got %d", len(called))
	}

	m.ExpireNow("a_1")
	m.Tick(ctx, func(rec domain.AuctionRecord) error {
		called = append(called, rec)
		return nil
	})

	if len(called) != 1 {
		t.Fatalf("want 1 expired record, got %d", len(called))
	}
	if called[0].AuctionID != "a_1" {
		t.Errorf("id = %q, want a_1", called[0].AuctionID)
	}
	if m.Contains("a_1") {
		t.Error("expired record should be removed")
	}
}

func TestMemoryManager_Tick_RestoresOnHandlerError(t *testing.T) {
	m := NewMemory(DefaultMemoryOptions())
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))
	m.ExpireNow("a_1")

	handlerErr := errors.New("bidder down")
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		return handlerErr
	})

	if !m.Contains("a_1") {
		t.Error("record should be restored after handler error")
	}
}

func TestMemoryManager_Subscribe_StopsOnContext(t *testing.T) {
	m := NewMemory(MemoryOptions{
		TTL:          time.Millisecond,
		ProcessedTTL: time.Minute,
		ScanInterval: time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())

	var count int
	var mu sync.Mutex
	m.Subscribe(ctx, func(_ domain.AuctionRecord) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	_ = m.Reserve(context.Background(), testRecord("a_1"))
	m.ExpireNow("a_1")

	// Ждём, пока воркер отработает.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := count > 0
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()

	mu.Lock()
	got := count
	mu.Unlock()
	if got == 0 {
		t.Error("worker did not process expired record")
	}
}

func TestMemoryManager_Tick_SkipsAlreadyTaken(t *testing.T) {
	m := NewMemory(MemoryOptions{
		TTL:          50 * time.Millisecond,
		ProcessedTTL: time.Minute,
		ScanInterval: time.Millisecond,
	})
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))
	// Забираем запись через Consume — как будто Impression успел раньше.
	_, _, _ = m.Consume(ctx, "a_1")

	// Пропускаем TTL.
	time.Sleep(80 * time.Millisecond)

	var called int
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})

	if called != 0 {
		t.Errorf("handler should not be called for already-consumed record, got %d", called)
	}
}

func TestMemoryManager_Concurrent_Consume(t *testing.T) {
	// 100 горутин Consume на одну запись. Ровно одна должна получить
	// ConsumeFresh, остальные — ConsumeAlready.
	m := NewMemory(DefaultMemoryOptions())
	ctx := context.Background()
	_ = m.Reserve(ctx, testRecord("a_1"))

	const goroutines = 100
	var wg sync.WaitGroup
	fresh := make([]int, goroutines)
	already := make([]int, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, status, _ := m.Consume(ctx, "a_1")
			if status == ConsumeFresh {
				fresh[idx] = 1
			} else {
				already[idx] = 1
			}
		}(i)
	}
	wg.Wait()

	freshCount := 0
	for _, v := range fresh {
		freshCount += v
	}
	if freshCount != 1 {
		t.Errorf("fresh = %d, want exactly 1", freshCount)
	}
}

func TestMemoryManager_Restore_RefreshesTTL(t *testing.T) {
	// Регрессионный тест на баг: Restore должен обновлять CreatedAt.
	// Без этого запись после Restore остаётся «просроченной» по старому
	// времени, и следующий Tick сразу уводит её в Rollback — Impression
	// не успевает повториться.
	m := NewMemory(MemoryOptions{
		TTL:          50 * time.Millisecond,
		ProcessedTTL: time.Minute,
		ScanInterval: time.Millisecond,
	})
	ctx := context.Background()

	_ = m.Reserve(ctx, testRecord("a_1"))
	m.ExpireNow("a_1") // теперь запись просрочена

	// Impression не удался → Restore.
	rec, _, _ := m.Consume(ctx, "a_1")
	if err := m.Restore(ctx, rec); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Сразу после Restore Tick не должен видеть запись просроченной.
	var called int
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})
	if called != 0 {
		t.Errorf("record should not be expired right after Restore, got %d rollback calls", called)
	}
}
