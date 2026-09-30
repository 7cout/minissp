package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func testCampaign(id, geo string, total int64) *domain.Campaign {
	return &domain.Campaign{
		ID:              id,
		AdvertiserID:    "adv_1",
		Name:            "Test Campaign",
		BudgetTotal:     total,
		BudgetRemaining: total,
		BudgetReserved:  0,
		GeoTarget:       geo,
	}
}

func TestCampaignRepo_ListByGeo(t *testing.T) {
	t.Run("filters by geo", func(t *testing.T) {
		repo := NewCampaignRepo()
		repo.Add(testCampaign("c1", "RU", 1_000_000))
		repo.Add(testCampaign("c2", "US", 1_000_000))
		repo.Add(testCampaign("c3", "RU", 1_000_000))

		campaigns, err := repo.ListByGeo(context.Background(), "RU")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(campaigns) != 2 {
			t.Errorf("want 2 campaigns, got %d", len(campaigns))
		}
	})

	t.Run("filters out zero budget", func(t *testing.T) {
		repo := NewCampaignRepo()

		// Кампания без бюджета.
		empty := testCampaign("c1", "RU", 1_000_000)
		empty.BudgetRemaining = 0
		empty.BudgetReserved = 0
		repo.Add(empty)

		// Кампания с зарезервированным бюджетом.
		reserved := testCampaign("c2", "RU", 1_000_000)
		reserved.BudgetRemaining = 1_000_000
		reserved.BudgetReserved = 1_000_000
		repo.Add(reserved)

		// Кампания с остатком.
		repo.Add(testCampaign("c3", "RU", 1_000_000))

		campaigns, err := repo.ListByGeo(context.Background(), "RU")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(campaigns) != 1 {
			t.Errorf("want 1 campaign with budget, got %d", len(campaigns))
		}
		if campaigns[0].ID != "c3" {
			t.Errorf("want c3, got %s", campaigns[0].ID)
		}
	})
}

func TestCampaignRepo_Reserve(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewCampaignRepo()
		repo.Add(testCampaign("c1", "RU", 1_000_000))

		err := repo.Reserve(context.Background(), "c1", 300_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewCampaignRepo()

		err := repo.Reserve(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("insufficient budget", func(t *testing.T) {
		repo := NewCampaignRepo()
		repo.Add(testCampaign("c1", "RU", 1_000_000))

		err := repo.Reserve(context.Background(), "c1", 2_000_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})
}

func TestCampaignRepo_Reserve_Concurrent(t *testing.T) {
	t.Run("all reservations succeed when budget allows", func(t *testing.T) {
		repo := NewCampaignRepo()
		repo.Add(testCampaign("c1", "RU", 1_000_000))

		const goroutines = 100
		const amount = 1_000 // всего 100_000 < 1_000_000

		var wg sync.WaitGroup
		var succeeded atomic.Int64

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := repo.Reserve(context.Background(), "c1", amount); err == nil {
					succeeded.Add(1)
				}
			}()
		}
		wg.Wait()

		if succeeded.Load() != goroutines {
			t.Errorf("succeeded = %d, want %d", succeeded.Load(), goroutines)
		}
	})

	t.Run("exactly as many succeed as budget allows", func(t *testing.T) {
		repo := NewCampaignRepo()
		repo.Add(testCampaign("c1", "RU", 1_000))

		const goroutines = 200
		const amount = 10 // хватит ровно на 100

		var wg sync.WaitGroup
		var succeeded atomic.Int64

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := repo.Reserve(context.Background(), "c1", amount); err == nil {
					succeeded.Add(1)
				}
			}()
		}
		wg.Wait()

		if succeeded.Load() != 100 {
			t.Errorf("succeeded = %d, want exactly 100", succeeded.Load())
		}
	})
}
