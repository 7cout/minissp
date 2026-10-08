package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CampaignRepo — in-memory реализация CampaignRepository.
type CampaignRepo struct {
	mu        sync.RWMutex
	campaigns map[string]*domain.Campaign
}

// NewCampaignRepo создаёт пустой репозиторий кампаний.
func NewCampaignRepo() *CampaignRepo {
	return &CampaignRepo{
		campaigns: make(map[string]*domain.Campaign),
	}
}

// Add добавляет кампанию.
//
// Возвращает ошибку при пустом ID или дубликате.
func (r *CampaignRepo) Add(_ context.Context, c *domain.Campaign) error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("campaign id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.campaigns[c.ID]; exists {
		return fmt.Errorf("campaign with id %s already exists", c.ID)
	}

	r.campaigns[c.ID] = c
	return nil
}

// Get возвращает кампанию по ID.
func (r *CampaignRepo) Get(_ context.Context, id string) (*domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.campaigns[id]
	if !ok {
		return nil, domain.ErrCampaignNotFound
	}
	cp := *c
	return &cp, nil
}

// ListByGeo возвращает кампании, подходящие по гео и имеющие
// доступный бюджет (budget_remaining > 0).
func (r *CampaignRepo) ListByGeo(_ context.Context, geo string) ([]domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Campaign
	for _, c := range r.campaigns {
		if c.GeoTarget != geo {
			continue
		}
		if c.BudgetRemaining <= 0 {
			continue
		}
		result = append(result, *c)
	}
	return result, nil
}

// Reserve резервирует amount на бюджете кампании.
func (r *CampaignRepo) Reserve(_ context.Context, campaignID string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.campaigns[campaignID]
	if !ok {
		return domain.ErrCampaignNotFound
	}
	if c.BudgetRemaining < amount {
		return domain.ErrInsufficientBudget
	}

	c.BudgetRemaining -= amount
	c.BudgetReserved += amount
	return nil
}

// Rollback отменяет резерв, возвращая деньги в budget_remaining.
func (r *CampaignRepo) Rollback(_ context.Context, campaignID string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.campaigns[campaignID]
	if !ok {
		return domain.ErrCampaignNotFound
	}
	if c.BudgetReserved < amount {
		return domain.ErrInsufficientBudget
	}

	c.BudgetReserved -= amount
	c.BudgetRemaining += amount
	return nil
}

// commitLocked снимает резерв с кампании без взятия мьютекса.
//
// Вызывается только из TxManager.CommitWithSpend, который уже держит
// r.mu. Возвращает advertiser_id кампании, чтобы TxManager знал,
// с кого списывать деньги.
func (r *CampaignRepo) commitLocked(campaignID string, amount int64) (advertiserID string, err error) {
	c, ok := r.campaigns[campaignID]
	if !ok {
		return "", domain.ErrCampaignNotFound
	}
	if c.BudgetReserved < amount {
		return "", domain.ErrInsufficientBudget
	}

	c.BudgetReserved -= amount
	return c.AdvertiserID, nil
}
