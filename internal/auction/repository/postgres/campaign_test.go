//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func insertCampaign(t *testing.T, id string, total, remaining, reserved int64, geo string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO campaigns (id, advertiser_id, name, budget_total, budget_remaining, budget_reserved, geo_target)
		VALUES ($1, '44444444-4444-4444-4444-444444444444', 'Test', $2, $3, $4, $5)
	`, id, total, remaining, reserved, geo)
	if err != nil {
		t.Fatalf("insert campaign: %v", err)
	}
}

func TestCampaignRepo_Reserve(t *testing.T) {
	const campaignID = "33333333-3333-3333-3333-333333333333"

	t.Run("success — budget updated", func(t *testing.T) {
		cleanTables(t)
		repo := NewCampaignRepo(testPool)
		insertCampaign(t, campaignID, 1_000_000, 1_000_000, 0, "RU")

		err := repo.Reserve(context.Background(), campaignID, 300_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		c, _ := repo.Get(context.Background(), campaignID)
		if c.BudgetRemaining != 700_000 {
			t.Errorf("remaining = %d, want 700000", c.BudgetRemaining)
		}
		if c.BudgetReserved != 300_000 {
			t.Errorf("reserved = %d, want 300000", c.BudgetReserved)
		}
		if c.BudgetRemaining+c.BudgetReserved != c.BudgetTotal {
			t.Errorf("invariant violated")
		}
	})

	t.Run("insufficient budget", func(t *testing.T) {
		cleanTables(t)
		repo := NewCampaignRepo(testPool)
		insertCampaign(t, campaignID, 1_000_000, 1_000_000, 0, "RU")

		err := repo.Reserve(context.Background(), campaignID, 2_000_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})

	t.Run("campaign not found", func(t *testing.T) {
		cleanTables(t)
		repo := NewCampaignRepo(testPool)

		err := repo.Reserve(context.Background(), "99999999-9999-9999-9999-999999999999", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})
}

func TestCampaignRepo_Reserve_Concurrent(t *testing.T) {
	const campaignID = "33333333-3333-3333-3333-333333333333"

	t.Run("exactly as many succeed as budget allows", func(t *testing.T) {
		cleanTables(t)
		repo := NewCampaignRepo(testPool)
		insertCampaign(t, campaignID, 1_000, 1_000, 0, "RU")

		const goroutines = 200
		const amount = 10 // хватит ровно на 100

		var wg sync.WaitGroup
		var succeeded atomic.Int64

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := repo.Reserve(context.Background(), campaignID, amount)
				if err == nil {
					succeeded.Add(1)
				}
			}()
		}
		wg.Wait()

		if succeeded.Load() != 100 {
			t.Errorf("succeeded = %d, want exactly 100", succeeded.Load())
		}

		// Проверяем итоговое состояние.
		c, _ := repo.Get(context.Background(), campaignID)
		if c.BudgetRemaining != 0 {
			t.Errorf("remaining = %d, want 0", c.BudgetRemaining)
		}
		if c.BudgetReserved != 1_000 {
			t.Errorf("reserved = %d, want 1000", c.BudgetReserved)
		}
	})
}
