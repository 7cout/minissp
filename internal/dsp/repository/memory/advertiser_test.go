package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func testAdvertiser(id string, balance int64) *domain.Advertiser {
	return &domain.Advertiser{
		ID:      id,
		Name:    "Test Advertiser",
		Balance: balance,
	}
}

func TestAdvertiserRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		repo.Add(testAdvertiser("adv_1", 1_000_000))

		got, err := repo.Get(context.Background(), "adv_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "adv_1" {
			t.Errorf("id = %q, want adv_1", got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewAdvertiserRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrAdvertiserNotFound) {
			t.Errorf("want ErrAdvertiserNotFound, got %v", err)
		}
	})

	t.Run("returns copy", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		repo.Add(testAdvertiser("adv_1", 1_000_000))

		got, _ := repo.Get(context.Background(), "adv_1")
		got.Balance = 0

		again, _ := repo.Get(context.Background(), "adv_1")
		if again.Balance != 1_000_000 {
			t.Errorf("balance changed via returned pointer: %d", again.Balance)
		}
	})
}

func TestAdvertiserRepo_Spend(t *testing.T) {
	t.Run("success — balance decreased", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		repo.Add(testAdvertiser("adv_1", 1_000_000))

		err := repo.Spend(context.Background(), "adv_1", 300_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a, _ := repo.Get(context.Background(), "adv_1")
		if a.Balance != 700_000 {
			t.Errorf("balance = %d, want 700000", a.Balance)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewAdvertiserRepo()

		err := repo.Spend(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrAdvertiserNotFound) {
			t.Errorf("want ErrAdvertiserNotFound, got %v", err)
		}
	})

	t.Run("insufficient balance", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		repo.Add(testAdvertiser("adv_1", 1_000))

		err := repo.Spend(context.Background(), "adv_1", 2_000)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("want ErrInsufficientBalance, got %v", err)
		}
	})
}

func TestAdvertiserRepo_Spend_Concurrent(t *testing.T) {
	t.Run("exactly as many succeed as balance allows", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		repo.Add(testAdvertiser("adv_1", 1_000))

		const goroutines = 200
		const amount = 10 // хватит ровно на 100

		var wg sync.WaitGroup
		var succeeded atomic.Int64

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := repo.Spend(context.Background(), "adv_1", amount); err == nil {
					succeeded.Add(1)
				}
			}()
		}
		wg.Wait()

		if succeeded.Load() != 100 {
			t.Errorf("succeeded = %d, want exactly 100", succeeded.Load())
		}

		a, _ := repo.Get(context.Background(), "adv_1")
		if a.Balance != 0 {
			t.Errorf("balance = %d, want 0", a.Balance)
		}
	})
}
