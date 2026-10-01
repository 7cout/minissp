package memory

import (
	"context"
	"sync"

	"github.com/7cout/minissp/internal/auction/domain"
)

// CampaignRepo - in-memory реализация CampaignRepository
type CampaignRepo struct {
	mu        sync.RWMutex
	campaigns map[string]*domain.Campaign
}

// NewCampaignRepo создаёт пустой репозиторий кампаний
func NewCampaignRepo() *CampaignRepo {
	return &CampaignRepo{
		campaigns: make(map[string]*domain.Campaign),
	}
}

// Add добавляет кампанию в репозиторий
// Используется для наполнения данными при запуске
func (r *CampaignRepo) Add(c *domain.Campaign) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.campaigns[c.ID] = c
}

// Get возвращает кампанию по ID.
//
// Возвращает domain.ErrCampaignNotFound, если кампания не найдена.
func (r *CampaignRepo) Get(_ context.Context, id string) (*domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.campaigns[id]
	if !ok {
		return nil, domain.ErrCampaignNotFound
	}
	return c, nil
}

// ListByGeo возвращает кампании, подходящие по гео и имеющие бюджет
func (r *CampaignRepo) ListByGeo(_ context.Context, geo string) ([]domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Campaign
	for _, c := range r.campaigns {
		if c.GeoTarget != geo {
			continue
		}
		// Доступный бюджет: remaining - reserved
		if c.BudgetRemaining-c.BudgetReserved <= 0 {
			continue
		}
		result = append(result, *c)
	}
	return result, nil
}

// Reserve резервирует amount на бюджете кампании
//
// Возвращает domain.ErrCampaignNotFound, если кампании нет
// Возвращает domain.ErrInsufficientBudget, если не хватает денег
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
