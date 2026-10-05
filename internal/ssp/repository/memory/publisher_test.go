package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func testPublisher(id, name, apiKey string, balance int64) *domain.Publisher {
	return &domain.Publisher{
		ID:      id,
		Name:    name,
		APIKey:  apiKey,
		Balance: balance,
	}
}

func TestPublisherRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		got, err := repo.Get(context.Background(), "pub_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "T2" {
			t.Errorf("name = %q, want T2", got.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewPublisherRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrPublisherNotFound) {
			t.Errorf("want ErrPublisherNotFound, got %v", err)
		}
	})

	t.Run("returns copy", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 100))

		got, _ := repo.Get(context.Background(), "pub_1")
		got.Balance = 999

		again, _ := repo.Get(context.Background(), "pub_1")
		if again.Balance != 100 {
			t.Errorf("balance changed via pointer: %d", again.Balance)
		}
	})
}

func TestPublisherRepo_GetByAPIKey(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		got, err := repo.GetByAPIKey(context.Background(), "key_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "pub_1" {
			t.Errorf("id = %q, want pub_1", got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		_, err := repo.GetByAPIKey(context.Background(), "wrong_key")
		if !errors.Is(err, domain.ErrPublisherNotFound) {
			t.Errorf("want ErrPublisherNotFound, got %v", err)
		}
	})
}

func TestPublisherRepo_AddBalance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		err := repo.AddBalance(context.Background(), "pub_1", 500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		p, _ := repo.Get(context.Background(), "pub_1")
		if p.Balance != 500 {
			t.Errorf("balance = %d, want 500", p.Balance)
		}
	})

	t.Run("multiple additions", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 100))

		_ = repo.AddBalance(context.Background(), "pub_1", 50)
		_ = repo.AddBalance(context.Background(), "pub_1", 25)

		p, _ := repo.Get(context.Background(), "pub_1")
		if p.Balance != 175 {
			t.Errorf("balance = %d, want 175", p.Balance)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewPublisherRepo()

		err := repo.AddBalance(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrPublisherNotFound) {
			t.Errorf("want ErrPublisherNotFound, got %v", err)
		}
	})

	t.Run("concurrent — all additions applied", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		const goroutines = 100
		const amount = 10

		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = repo.AddBalance(context.Background(), "pub_1", amount)
			}()
		}
		wg.Wait()

		p, _ := repo.Get(context.Background(), "pub_1")
		want := int64(goroutines * amount)
		if p.Balance != want {
			t.Errorf("balance = %d, want %d", p.Balance, want)
		}
	})

	t.Run("concurrent — with missing publisher", func(t *testing.T) {
		repo := NewPublisherRepo()
		repo.Add(testPublisher("pub_1", "T2", "key_1", 0))

		var wg sync.WaitGroup
		var failed atomic.Int64

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := repo.AddBalance(context.Background(), "missing", 10); err != nil {
					failed.Add(1)
				}
			}()
		}
		wg.Wait()

		if failed.Load() != 50 {
			t.Errorf("failed = %d, want 50", failed.Load())
		}
	})
}
