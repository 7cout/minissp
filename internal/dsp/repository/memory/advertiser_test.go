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

func addAdvertiser(t *testing.T, repo *AdvertiserRepo, a *domain.Advertiser) {
	t.Helper()
	if err := repo.Add(context.Background(), a); err != nil {
		t.Fatalf("add advertiser: %v", err)
	}
}

func TestAdvertiserRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		addAdvertiser(t, repo, testAdvertiser("adv_1", 1_000_000))

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
		addAdvertiser(t, repo, testAdvertiser("adv_1", 1_000_000))

		got, _ := repo.Get(context.Background(), "adv_1")
		got.Balance = 0

		again, _ := repo.Get(context.Background(), "adv_1")
		if again.Balance != 1_000_000 {
			t.Errorf("balance changed via returned pointer: %d", again.Balance)
		}
	})
}

func TestAdvertiserRepo_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		err := repo.Add(context.Background(), testAdvertiser("adv_1", 1_000_000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		err := repo.Add(context.Background(), testAdvertiser("", 1_000_000))
		if err == nil {
			t.Error("want error for empty id, got nil")
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		repo := NewAdvertiserRepo()
		addAdvertiser(t, repo, testAdvertiser("adv_1", 1_000_000))

		err := repo.Add(context.Background(), testAdvertiser("adv_1", 500_000))
		if err == nil {
			t.Error("want error for duplicate id, got nil")
		}
	})
}

// --- TxManager ---

func TestTxManager_CommitWithSpend_HappyPath(t *testing.T) {
	campaigns := NewCampaignRepo()
	advertisers := NewAdvertiserRepo()
	tx := NewTxManager(campaigns, advertisers)
	ctx := context.Background()

	addAdvertiser(t, advertisers, testAdvertiser("adv_1", 10_000_000))
	addCampaign(t, campaigns, testCampaign("camp_1", "adv_1", "RU", 5_000_000))
	_ = campaigns.Reserve(ctx, "camp_1", 1_500_000)

	if err := tx.CommitWithSpend(ctx, "camp_1", 1_500_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, _ := campaigns.Get(ctx, "camp_1")
	if c.BudgetReserved != 0 {
		t.Errorf("campaign reserved = %d, want 0", c.BudgetReserved)
	}
	if c.BudgetRemaining != 3_500_000 {
		t.Errorf("campaign remaining = %d, want 3500000", c.BudgetRemaining)
	}

	a, _ := advertisers.Get(ctx, "adv_1")
	if a.Balance != 8_500_000 {
		t.Errorf("advertiser balance = %d, want 8500000", a.Balance)
	}
}

func TestTxManager_CommitWithSpend_CampaignNotFound(t *testing.T) {
	tx := NewTxManager(NewCampaignRepo(), NewAdvertiserRepo())

	err := tx.CommitWithSpend(context.Background(), "missing", 100)
	if !errors.Is(err, domain.ErrCampaignNotFound) {
		t.Errorf("want ErrCampaignNotFound, got %v", err)
	}
}

func TestTxManager_CommitWithSpend_InsufficientReserved(t *testing.T) {
	campaigns := NewCampaignRepo()
	advertisers := NewAdvertiserRepo()
	tx := NewTxManager(campaigns, advertisers)
	ctx := context.Background()

	addAdvertiser(t, advertisers, testAdvertiser("adv_1", 10_000_000))
	addCampaign(t, campaigns, testCampaign("camp_1", "adv_1", "RU", 5_000_000))
	// Не резервируем.

	err := tx.CommitWithSpend(ctx, "camp_1", 1_500_000)
	if !errors.Is(err, domain.ErrInsufficientBudget) {
		t.Errorf("want ErrInsufficientBudget, got %v", err)
	}

	a, _ := advertisers.Get(ctx, "adv_1")
	if a.Balance != 10_000_000 {
		t.Errorf("advertiser balance = %d, want unchanged", a.Balance)
	}
}

func TestTxManager_CommitWithSpend_AdvertiserNotFound(t *testing.T) {
	campaigns := NewCampaignRepo()
	advertisers := NewAdvertiserRepo()
	tx := NewTxManager(campaigns, advertisers)
	ctx := context.Background()

	// Кампания ссылается на advertiser, которого нет.
	addCampaign(t, campaigns, testCampaign("camp_1", "missing_adv", "RU", 5_000_000))
	_ = campaigns.Reserve(ctx, "camp_1", 1_500_000)

	err := tx.CommitWithSpend(ctx, "camp_1", 1_500_000)
	if !errors.Is(err, domain.ErrAdvertiserNotFound) {
		t.Errorf("want ErrAdvertiserNotFound, got %v", err)
	}

	// Резерв должен быть возвращён.
	c, _ := campaigns.Get(ctx, "camp_1")
	if c.BudgetReserved != 1_500_000 {
		t.Errorf("campaign reserved = %d, want 1500000 (restored)", c.BudgetReserved)
	}
}

func TestTxManager_CommitWithSpend_InsufficientBalance(t *testing.T) {
	campaigns := NewCampaignRepo()
	advertisers := NewAdvertiserRepo()
	tx := NewTxManager(campaigns, advertisers)
	ctx := context.Background()

	addAdvertiser(t, advertisers, testAdvertiser("adv_1", 100)) // меньше, чем 1_500_000
	addCampaign(t, campaigns, testCampaign("camp_1", "adv_1", "RU", 5_000_000))
	_ = campaigns.Reserve(ctx, "camp_1", 1_500_000)

	err := tx.CommitWithSpend(ctx, "camp_1", 1_500_000)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("want ErrInsufficientBalance, got %v", err)
	}

	// Резерв должен быть возвращён.
	c, _ := campaigns.Get(ctx, "camp_1")
	if c.BudgetReserved != 1_500_000 {
		t.Errorf("campaign reserved = %d, want 1500000 (restored)", c.BudgetReserved)
	}

	// Баланс не изменился.
	a, _ := advertisers.Get(ctx, "adv_1")
	if a.Balance != 100 {
		t.Errorf("advertiser balance = %d, want unchanged 100", a.Balance)
	}
}

func TestTxManager_CommitWithSpend_Concurrent(t *testing.T) {
	// 100 горутин параллельно списывают по 1 с баланса 10_000.
	// Все должны списаться (никаких потерь).
	campaigns := NewCampaignRepo()
	advertisers := NewAdvertiserRepo()
	tx := NewTxManager(campaigns, advertisers)
	ctx := context.Background()

	addAdvertiser(t, advertisers, testAdvertiser("adv_1", 10_000))
	addCampaign(t, campaigns, testCampaign("camp_1", "adv_1", "RU", 1_000_000))
	_ = campaigns.Reserve(ctx, "camp_1", 10_000)

	const goroutines = 100
	var wg sync.WaitGroup
	var succeeded atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tx.CommitWithSpend(ctx, "camp_1", 100); err == nil {
				succeeded.Add(1)
			}
		}()
	}
	wg.Wait()

	if succeeded.Load() != goroutines {
		t.Errorf("succeeded = %d, want %d", succeeded.Load(), goroutines)
	}

	a, _ := advertisers.Get(ctx, "adv_1")
	if a.Balance != 0 {
		t.Errorf("balance = %d, want 0", a.Balance)
	}

	c, _ := campaigns.Get(ctx, "camp_1")
	if c.BudgetReserved != 0 {
		t.Errorf("reserved = %d, want 0", c.BudgetReserved)
	}
}
