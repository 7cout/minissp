//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// createTestAdvertiser создаёт advertiser для FK-связи кампании.
func createTestAdvertiser(t *testing.T, repo *AdvertiserRepo, balance int64) *domain.Advertiser {
	t.Helper()

	a := &domain.Advertiser{
		ID:      uuid.NewString(),
		Name:    "Test Advertiser",
		Balance: balance,
	}
	if err := repo.Add(context.Background(), a); err != nil {
		t.Fatalf("create test advertiser: %v", err)
	}
	return a
}

func testCampaign(advertiserID, geo string, total int64) *domain.Campaign {
	return &domain.Campaign{
		ID:              uuid.NewString(),
		AdvertiserID:    advertiserID,
		Name:            "Test Campaign",
		BudgetTotal:     total,
		BudgetRemaining: total,
		BudgetReserved:  0,
		GeoTarget:       geo,
	}
}

func TestPostgresCampaignRepo_AddAndGet(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
	c := testCampaign(adv.ID, "RU", 5_000_000_000)

	if err := campaigns.Add(ctx, c); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := campaigns.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AdvertiserID != adv.ID {
		t.Errorf("advertiser_id = %q, want %q", got.AdvertiserID, adv.ID)
	}
	if got.GeoTarget != "RU" {
		t.Errorf("geo = %q, want RU", got.GeoTarget)
	}
	if got.BudgetRemaining != c.BudgetRemaining {
		t.Errorf("remaining = %d, want %d", got.BudgetRemaining, c.BudgetRemaining)
	}
}

func TestPostgresCampaignRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	campaigns := NewCampaignRepo(pool)

	_, err := campaigns.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrCampaignNotFound) {
		t.Errorf("want ErrCampaignNotFound, got %v", err)
	}
}

func TestPostgresCampaignRepo_ListByGeo(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
	_ = campaigns.Add(ctx, testCampaign(adv.ID, "RU", 1_000_000))
	_ = campaigns.Add(ctx, testCampaign(adv.ID, "US", 1_000_000))
	_ = campaigns.Add(ctx, testCampaign(adv.ID, "RU", 1_000_000))

	t.Run("filters by geo", func(t *testing.T) {
		got, err := campaigns.ListByGeo(ctx, "RU")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("want 2 campaigns, got %d", len(got))
		}
	})

	t.Run("filters out zero budget", func(t *testing.T) {
		got, _ := campaigns.ListByGeo(ctx, "RU")
		target := got[0]
		_ = campaigns.Reserve(ctx, target.ID, target.BudgetRemaining)

		after, _ := campaigns.ListByGeo(ctx, "RU")
		if len(after) != 1 {
			t.Errorf("want 1 campaign with budget, got %d", len(after))
		}
	})
}

func TestPostgresCampaignRepo_Reserve(t *testing.T) {
	t.Run("success — budget updated correctly", func(t *testing.T) {
		pool := newTestPool(t)
		advertisers := NewAdvertiserRepo(pool)
		campaigns := NewCampaignRepo(pool)
		ctx := context.Background()

		adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
		c := testCampaign(adv.ID, "RU", 1_000_000)
		_ = campaigns.Add(ctx, c)

		if err := campaigns.Reserve(ctx, c.ID, 300_000); err != nil {
			t.Fatalf("reserve: %v", err)
		}

		got, _ := campaigns.Get(ctx, c.ID)
		if got.BudgetRemaining != 700_000 {
			t.Errorf("remaining = %d, want 700000", got.BudgetRemaining)
		}
		if got.BudgetReserved != 300_000 {
			t.Errorf("reserved = %d, want 300000", got.BudgetReserved)
		}
	})

	t.Run("not found", func(t *testing.T) {
		pool := newTestPool(t)
		campaigns := NewCampaignRepo(pool)

		err := campaigns.Reserve(context.Background(), uuid.NewString(), 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("insufficient budget", func(t *testing.T) {
		pool := newTestPool(t)
		advertisers := NewAdvertiserRepo(pool)
		campaigns := NewCampaignRepo(pool)
		ctx := context.Background()

		adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
		c := testCampaign(adv.ID, "RU", 1_000_000)
		_ = campaigns.Add(ctx, c)

		err := campaigns.Reserve(ctx, c.ID, 2_000_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}

		got, _ := campaigns.Get(ctx, c.ID)
		if got.BudgetRemaining != 1_000_000 {
			t.Errorf("remaining = %d, want 1000000 (unchanged)", got.BudgetRemaining)
		}
	})
}

func TestPostgresCampaignRepo_Rollback(t *testing.T) {
	t.Run("success — reserved decreased, remaining restored", func(t *testing.T) {
		pool := newTestPool(t)
		advertisers := NewAdvertiserRepo(pool)
		campaigns := NewCampaignRepo(pool)
		ctx := context.Background()

		adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
		c := testCampaign(adv.ID, "RU", 1_000_000)
		_ = campaigns.Add(ctx, c)
		_ = campaigns.Reserve(ctx, c.ID, 300_000)

		if err := campaigns.Rollback(ctx, c.ID, 300_000); err != nil {
			t.Fatalf("rollback: %v", err)
		}

		got, _ := campaigns.Get(ctx, c.ID)
		if got.BudgetRemaining != 1_000_000 {
			t.Errorf("remaining = %d, want 1000000", got.BudgetRemaining)
		}
		if got.BudgetReserved != 0 {
			t.Errorf("reserved = %d, want 0", got.BudgetReserved)
		}
	})
}

func TestPostgresCampaignRepo_Reserve_Concurrent(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
	c := testCampaign(adv.ID, "RU", 1_000)
	_ = campaigns.Add(ctx, c)

	const goroutines = 200
	const amount = 10

	var wg sync.WaitGroup
	var succeeded atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := campaigns.Reserve(ctx, c.ID, amount); err == nil {
				succeeded.Add(1)
			}
		}()
	}
	wg.Wait()

	if succeeded.Load() != 100 {
		t.Errorf("succeeded = %d, want exactly 100", succeeded.Load())
	}

	got, _ := campaigns.Get(ctx, c.ID)
	if got.BudgetRemaining != 0 {
		t.Errorf("remaining = %d, want 0", got.BudgetRemaining)
	}
	if got.BudgetReserved != 1_000 {
		t.Errorf("reserved = %d, want 1000", got.BudgetReserved)
	}
}
