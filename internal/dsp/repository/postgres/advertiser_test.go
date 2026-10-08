//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// newTestPool подключается к Postgres из docker-compose и очищает
// таблицы перед тестом. Если DATABASE_URL не задан — тест скипается.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), `TRUNCATE dsp.advertisers CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func testAdvertiser(balance int64) *domain.Advertiser {
	return &domain.Advertiser{
		ID:      uuid.NewString(),
		Name:    "Nike",
		Balance: balance,
	}
}

func TestPostgresAdvertiserRepo_AddAndGet(t *testing.T) {
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)
	ctx := context.Background()

	a := testAdvertiser(1_000_000_000)
	if err := repo.Add(ctx, a); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := repo.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != a.Name {
		t.Errorf("name = %q, want %q", got.Name, a.Name)
	}
	if got.Balance != a.Balance {
		t.Errorf("balance = %d, want %d", got.Balance, a.Balance)
	}
}

func TestPostgresAdvertiserRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)

	_, err := repo.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrAdvertiserNotFound) {
		t.Errorf("want ErrAdvertiserNotFound, got %v", err)
	}
}

func TestPostgresAdvertiserRepo_Spend(t *testing.T) {
	t.Run("success — balance decreased", func(t *testing.T) {
		pool := newTestPool(t)
		repo := NewAdvertiserRepo(pool)
		ctx := context.Background()

		a := testAdvertiser(1_000_000)
		_ = repo.Add(ctx, a)

		if err := repo.Spend(ctx, a.ID, 300_000); err != nil {
			t.Fatalf("spend: %v", err)
		}

		got, _ := repo.Get(ctx, a.ID)
		if got.Balance != 700_000 {
			t.Errorf("balance = %d, want 700000", got.Balance)
		}
	})

	t.Run("not found", func(t *testing.T) {
		pool := newTestPool(t)
		repo := NewAdvertiserRepo(pool)

		err := repo.Spend(context.Background(), uuid.NewString(), 100)
		if !errors.Is(err, domain.ErrAdvertiserNotFound) {
			t.Errorf("want ErrAdvertiserNotFound, got %v", err)
		}
	})

	t.Run("insufficient balance", func(t *testing.T) {
		pool := newTestPool(t)
		repo := NewAdvertiserRepo(pool)
		ctx := context.Background()

		a := testAdvertiser(1_000)
		_ = repo.Add(ctx, a)

		err := repo.Spend(ctx, a.ID, 2_000)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("want ErrInsufficientBalance, got %v", err)
		}

		// Баланс не изменился.
		got, _ := repo.Get(ctx, a.ID)
		if got.Balance != 1_000 {
			t.Errorf("balance = %d, want 1000 (unchanged)", got.Balance)
		}
	})
}

func TestPostgresAdvertiserRepo_Spend_Concurrent(t *testing.T) {
	// То же, что в memory-тесте: 200 горутин тратят по 10,
	// баланса хватает ровно на 100 успешных.
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)
	ctx := context.Background()

	a := testAdvertiser(1_000)
	_ = repo.Add(ctx, a)

	const goroutines = 200
	const amount = 10

	var wg sync.WaitGroup
	var succeeded atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.Spend(ctx, a.ID, amount); err == nil {
				succeeded.Add(1)
			}
		}()
	}
	wg.Wait()

	if succeeded.Load() != 100 {
		t.Errorf("succeeded = %d, want exactly 100", succeeded.Load())
	}

	got, _ := repo.Get(ctx, a.ID)
	if got.Balance != 0 {
		t.Errorf("balance = %d, want 0", got.Balance)
	}
}
