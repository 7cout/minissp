package reserve_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/reserve"
)

// TTL в contract-тестах — короткий, чтобы проверять истечение
// без долгих sleep.
const (
	contractTTL    = 50 * time.Millisecond
	contractSleep  = 100 * time.Millisecond
	contractWaitLo = 5 * time.Millisecond
)

// ManagerFactory создаёт новый Manager для одного теста.
//
// Каждая фабрика должна возвращать «чистый» менеджер и подчищать
// за собой ресурсы через t.Cleanup.
type ManagerFactory func(t *testing.T) reserve.Manager

// factories — реализации, которые прогоняются через один и тот же
// набор contract-тестов. Добавляешь новую реализацию — просто
// добавь сюда.
var factories = map[string]ManagerFactory{
	"memory": func(_ *testing.T) reserve.Manager {
		return reserve.NewMemory(reserve.MemoryOptions{
			TTL:          contractTTL,
			ProcessedTTL: time.Minute,
			ScanInterval: time.Millisecond,
		})
	},
	"redis": func(t *testing.T) reserve.Manager {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })

		return reserve.NewRedis(client, reserve.RedisOptions{
			TTL:           contractTTL,
			ProcessedTTL:  time.Minute,
			ScanInterval:  time.Millisecond,
			TickBatchSize: 100,
			KeyPrefix:     "reserve",
			ShardTag:      "test",
		})
	},
}

// contractTest — один сценарий, общий для всех реализаций.
type contractTest struct {
	name string
	run  func(t *testing.T, m reserve.Manager)
}

// contractTests — единый набор сценариев.
var contractTests = []contractTest{
	{"Consume_Fresh", testContractConsumeFresh},
	{"Consume_Already", testContractConsumeAlready},
	{"Consume_NotFound", testContractConsumeNotFound},
	{"Restore_ReturnsRecord", testContractRestoreReturnsRecord},
	{"Restore_RefreshesTTL", testContractRestoreRefreshesTTL},
	{"Peek_ReturnsRecord", testContractPeekReturnsRecord},
	{"Peek_NotFound", testContractPeekNotFound},
	{"Tick_RollsBackExpired", testContractTickRollsBackExpired},
	{"Tick_SkipsFreshRecord", testContractTickSkipsFreshRecord},
	{"Tick_RestoresOnHandlerError", testContractTickRestoresOnHandlerError},
	{"Tick_SkipsAlreadyConsumed", testContractTickSkipsAlreadyConsumed},
	{"Concurrent_Consume_OnlyOneFresh", testContractConcurrentConsume},
	{"Concurrent_ConsumeVsTick", testContractConcurrentConsumeVsTick},
}

func TestContract(t *testing.T) {
	for implName, factory := range factories {
		implName, factory := implName, factory
		t.Run(implName, func(t *testing.T) {
			for _, tc := range contractTests {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					m := factory(t)
					tc.run(t, m)
				})
			}
		})
	}
}

// --- Сценарии ---

func testContractConsumeFresh(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	if err := m.Reserve(ctx, rec); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	got, status, err := m.Consume(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if status != reserve.ConsumeFresh {
		t.Errorf("status = %v, want ConsumeFresh", status)
	}
	if got.AuctionID != rec.AuctionID {
		t.Errorf("id = %q, want %q", got.AuctionID, rec.AuctionID)
	}
	if got.CampaignID != rec.CampaignID {
		t.Errorf("campaign = %q, want %q", got.CampaignID, rec.CampaignID)
	}
	if got.Price != rec.Price {
		t.Errorf("price = %d, want %d", got.Price, rec.Price)
	}
}

func testContractConsumeAlready(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	_, _, _ = m.Consume(ctx, rec.AuctionID)

	_, status, err := m.Consume(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("second consume: %v", err)
	}
	if status != reserve.ConsumeAlready {
		t.Errorf("status = %v, want ConsumeAlready", status)
	}
}

func testContractConsumeNotFound(t *testing.T, m reserve.Manager) {
	_, _, err := m.Consume(context.Background(), "missing")
	if !errors.Is(err, reserve.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func testContractRestoreReturnsRecord(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	taken, _, _ := m.Consume(ctx, rec.AuctionID)

	if err := m.Restore(ctx, taken); err != nil {
		t.Fatalf("restore: %v", err)
	}

	again, _, err := m.Consume(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("consume after restore: %v", err)
	}
	if again.AuctionID != rec.AuctionID {
		t.Errorf("id = %q, want %q", again.AuctionID, rec.AuctionID)
	}
}

func testContractRestoreRefreshesTTL(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	taken, _, _ := m.Consume(ctx, rec.AuctionID)
	_ = m.Restore(ctx, taken)

	// Сразу после Restore запись не должна быть просрочена.
	var called int
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})
	if called != 0 {
		t.Errorf("no rollback expected right after Restore, got %d", called)
	}

	// А через TTL — должна.
	time.Sleep(contractSleep)

	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})
	if called != 1 {
		t.Errorf("rollback expected after TTL, got %d", called)
	}
}

func testContractPeekReturnsRecord(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)

	got, err := m.Peek(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if got.AuctionID != rec.AuctionID {
		t.Errorf("id = %q, want %q", got.AuctionID, rec.AuctionID)
	}

	// Peek не должен изымать запись.
	again, err := m.Peek(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("second peek: %v", err)
	}
	if again.AuctionID != rec.AuctionID {
		t.Errorf("record disappeared after first Peek")
	}
}

func testContractPeekNotFound(t *testing.T, m reserve.Manager) {
	_, err := m.Peek(context.Background(), "missing")
	if !errors.Is(err, reserve.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func testContractTickRollsBackExpired(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	time.Sleep(contractSleep)

	var got []domain.AuctionRecord
	m.Tick(ctx, func(r domain.AuctionRecord) error {
		got = append(got, r)
		return nil
	})

	if len(got) != 1 {
		t.Fatalf("want 1 expired, got %d", len(got))
	}
	if got[0].AuctionID != rec.AuctionID {
		t.Errorf("id = %q, want %q", got[0].AuctionID, rec.AuctionID)
	}

	// После Tick запись должна быть удалена из резерва.
	if _, err := m.Peek(ctx, rec.AuctionID); !errors.Is(err, reserve.ErrNotFound) {
		t.Errorf("record should be gone after Tick, got %v", err)
	}
}

func testContractTickSkipsFreshRecord(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)

	var called int
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})
	if called != 0 {
		t.Errorf("fresh record should not be rolled back, got %d", called)
	}
}

func testContractTickRestoresOnHandlerError(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	time.Sleep(contractSleep)

	handlerErr := errors.New("bidder down")
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		return handlerErr
	})

	// Запись должна быть возвращена в резерв.
	got, err := m.Peek(ctx, rec.AuctionID)
	if err != nil {
		t.Fatalf("record should be restored, got %v", err)
	}
	if got.AuctionID != rec.AuctionID {
		t.Errorf("id = %q, want %q", got.AuctionID, rec.AuctionID)
	}
}

func testContractTickSkipsAlreadyConsumed(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)
	_, _, _ = m.Consume(ctx, rec.AuctionID)

	// Ждём TTL и пытаемся тикать: запись не должна прийти в onExpired.
	time.Sleep(contractSleep)

	var called int
	m.Tick(ctx, func(_ domain.AuctionRecord) error {
		called++
		return nil
	})
	if called != 0 {
		t.Errorf("already consumed record should not be rolled back, got %d", called)
	}
}

func testContractConcurrentConsume(t *testing.T, m reserve.Manager) {
	ctx := context.Background()
	rec := testRecord("a_1")

	_ = m.Reserve(ctx, rec)

	const goroutines = 50
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		fresh int
	)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, status, err := m.Consume(ctx, rec.AuctionID)
			if err != nil {
				return
			}
			if status == reserve.ConsumeFresh {
				mu.Lock()
				fresh++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if fresh != 1 {
		t.Errorf("fresh = %d, want exactly 1", fresh)
	}
}

func testContractConcurrentConsumeVsTick(t *testing.T, m reserve.Manager) {
	// Гонка: Impression (Consume) и TTL-воркер (Tick) на одной записи.
	// Победитель должен быть ровно один.
	for i := 0; i < 30; i++ {
		ctx := context.Background()
		rec := testRecord("a_1")

		_ = m.Reserve(ctx, rec)
		time.Sleep(contractSleep) // делаем запись просроченной

		var (
			wg           sync.WaitGroup
			mu           sync.Mutex
			rollbackSeen bool
			freshSeen    bool
		)

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, status, err := m.Consume(ctx, rec.AuctionID)
			if err == nil && status == reserve.ConsumeFresh {
				mu.Lock()
				freshSeen = true
				mu.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			m.Tick(ctx, func(_ domain.AuctionRecord) error {
				mu.Lock()
				rollbackSeen = true
				mu.Unlock()
				return nil
			})
		}()
		wg.Wait()

		if freshSeen && rollbackSeen {
			t.Fatalf("iter %d: both Consume(fresh) and Tick(rollback) observed same record", i)
		}
	}
}

// --- helper ---

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
