package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func testCampaign(id, advertiserID, geo string, total int64) *domain.Campaign {
	return &domain.Campaign{
		ID:              id,
		AdvertiserID:    advertiserID,
		Name:            "Test Campaign",
		BudgetTotal:     total,
		BudgetRemaining: total,
		BudgetReserved:  0,
		GeoTarget:       geo,
	}
}

func addCampaign(t *testing.T, repo *CampaignRepo, c *domain.Campaign) {
	t.Helper()
	if err := repo.Add(context.Background(), c); err != nil {
		t.Fatalf("add campaign: %v", err)
	}
}

func TestCampaignRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))

		got, err := repo.Get(context.Background(), "c1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "c1" {
			t.Errorf("id = %q, want c1", got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewCampaignRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})
}

func TestCampaignRepo_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewCampaignRepo()
		err := repo.Add(context.Background(), testCampaign("c1", "adv_1", "RU", 1_000_000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		repo := NewCampaignRepo()
		err := repo.Add(context.Background(), testCampaign("", "adv_1", "RU", 1_000_000))
		if err == nil {
			t.Error("want error for empty id, got nil")
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))

		err := repo.Add(context.Background(), testCampaign("c1", "adv_2", "RU", 1_000_000))
		if err == nil {
			t.Error("want error for duplicate id, got nil")
		}
	})
}

func TestCampaignRepo_ListByGeo(t *testing.T) {
	t.Run("filters by geo", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))
		addCampaign(t, repo, testCampaign("c2", "adv_1", "US", 1_000_000))
		addCampaign(t, repo, testCampaign("c3", "adv_1", "RU", 1_000_000))

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

		empty := testCampaign("c1", "adv_1", "RU", 1_000_000)
		empty.BudgetRemaining = 0
		addCampaign(t, repo, empty)

		addCampaign(t, repo, testCampaign("c2", "adv_1", "RU", 1_000_000))

		campaigns, _ := repo.ListByGeo(context.Background(), "RU")
		if len(campaigns) != 1 {
			t.Errorf("want 1 campaign with budget, got %d", len(campaigns))
		}
		if campaigns[0].ID != "c2" {
			t.Errorf("want c2, got %s", campaigns[0].ID)
		}
	})
}

func TestCampaignRepo_Reserve(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))

		err := repo.Reserve(context.Background(), "c1", 300_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		c, _ := repo.Get(context.Background(), "c1")
		if c.BudgetRemaining != 700_000 {
			t.Errorf("remaining = %d, want 700000", c.BudgetRemaining)
		}
		if c.BudgetReserved != 300_000 {
			t.Errorf("reserved = %d, want 300000", c.BudgetReserved)
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
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))

		err := repo.Reserve(context.Background(), "c1", 2_000_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})
}

func TestCampaignRepo_Rollback(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))
		_ = repo.Reserve(context.Background(), "c1", 300_000)

		err := repo.Rollback(context.Background(), "c1", 300_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		c, _ := repo.Get(context.Background(), "c1")
		if c.BudgetRemaining != 1_000_000 {
			t.Errorf("remaining = %d, want 1000000", c.BudgetRemaining)
		}
		if c.BudgetReserved != 0 {
			t.Errorf("reserved = %d, want 0", c.BudgetReserved)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewCampaignRepo()
		err := repo.Rollback(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("insufficient reserved", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000_000))

		err := repo.Rollback(context.Background(), "c1", 100)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})
}

func TestCampaignRepo_Reserve_Concurrent(t *testing.T) {
	t.Run("exactly as many succeed as budget allows", func(t *testing.T) {
		repo := NewCampaignRepo()
		addCampaign(t, repo, testCampaign("c1", "adv_1", "RU", 1_000))

		const goroutines = 200
		const amount = 10

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

		c, _ := repo.Get(context.Background(), "c1")
		if c.BudgetRemaining != 0 {
			t.Errorf("remaining = %d, want 0", c.BudgetRemaining)
		}
		if c.BudgetReserved != 1_000 {
			t.Errorf("reserved = %d, want 1000", c.BudgetReserved)
		}
	})
}
