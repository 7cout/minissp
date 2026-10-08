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

func TestTxManager_CommitWithSpend_HappyPath(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 10_000_000)
	c := testCampaign(adv.ID, "RU", 5_000_000)
	_ = campaigns.Add(ctx, c)
	_ = campaigns.Reserve(ctx, c.ID, 1_500_000)

	if err := tx.CommitWithSpend(ctx, c.ID, 1_500_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := campaigns.Get(ctx, c.ID)
	if got.BudgetReserved != 0 {
		t.Errorf("campaign reserved = %d, want 0", got.BudgetReserved)
	}
	if got.BudgetRemaining != 3_500_000 {
		t.Errorf("campaign remaining = %d, want 3500000", got.BudgetRemaining)
	}

	a, _ := advertisers.Get(ctx, adv.ID)
	if a.Balance != 8_500_000 {
		t.Errorf("advertiser balance = %d, want 8500000", a.Balance)
	}
}

func TestTxManager_CommitWithSpend_CampaignNotFound(t *testing.T) {
	pool := newTestPool(t)
	tx := NewTxManager(pool)

	err := tx.CommitWithSpend(context.Background(), uuid.NewString(), 100)
	if !errors.Is(err, domain.ErrCampaignNotFound) {
		t.Errorf("want ErrCampaignNotFound, got %v", err)
	}
}

func TestTxManager_CommitWithSpend_InsufficientReserved(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 10_000_000)
	c := testCampaign(adv.ID, "RU", 5_000_000)
	_ = campaigns.Add(ctx, c)
	// Не резервируем.

	err := tx.CommitWithSpend(ctx, c.ID, 1_500_000)
	if !errors.Is(err, domain.ErrInsufficientBudget) {
		t.Errorf("want ErrInsufficientBudget, got %v", err)
	}

	// Ничего не изменилось.
	a, _ := advertisers.Get(ctx, adv.ID)
	if a.Balance != 10_000_000 {
		t.Errorf("advertiser balance = %d, want unchanged", a.Balance)
	}
	got, _ := campaigns.Get(ctx, c.ID)
	if got.BudgetReserved != 0 {
		t.Errorf("reserved = %d, want 0 (unchanged)", got.BudgetReserved)
	}
}

func TestTxManager_CommitWithSpend_InsufficientBalance(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	// Advertiser с маленьким балансом; кампания — с нормальным бюджетом.
	adv := createTestAdvertiser(t, advertisers, 100)
	c := testCampaign(adv.ID, "RU", 5_000_000)
	_ = campaigns.Add(ctx, c)
	_ = campaigns.Reserve(ctx, c.ID, 1_500_000)

	err := tx.CommitWithSpend(ctx, c.ID, 1_500_000)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("want ErrInsufficientBalance, got %v", err)
	}

	// Резерв должен быть возвращён (транзакция откатилась).
	got, _ := campaigns.Get(ctx, c.ID)
	if got.BudgetReserved != 1_500_000 {
		t.Errorf("reserved = %d, want 1500000 (rolled back)", got.BudgetReserved)
	}

	// Баланс advertiser'а не изменился.
	a, _ := advertisers.Get(ctx, adv.ID)
	if a.Balance != 100 {
		t.Errorf("advertiser balance = %d, want unchanged 100", a.Balance)
	}
}

func TestTxManager_CommitWithSpend_Concurrent(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	tx := NewTxManager(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 10_000)
	c := testCampaign(adv.ID, "RU", 1_000_000)
	_ = campaigns.Add(ctx, c)
	_ = campaigns.Reserve(ctx, c.ID, 10_000)

	const goroutines = 100
	var wg sync.WaitGroup
	var succeeded atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tx.CommitWithSpend(ctx, c.ID, 100); err == nil {
				succeeded.Add(1)
			}
		}()
	}
	wg.Wait()

	if succeeded.Load() != goroutines {
		t.Errorf("succeeded = %d, want %d", succeeded.Load(), goroutines)
	}

	a, _ := advertisers.Get(ctx, adv.ID)
	if a.Balance != 0 {
		t.Errorf("advertiser balance = %d, want 0", a.Balance)
	}
	got, _ := campaigns.Get(ctx, c.ID)
	if got.BudgetReserved != 0 {
		t.Errorf("reserved = %d, want 0", got.BudgetReserved)
	}
}
