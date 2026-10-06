package memory

import (
	"context"
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
func (r *CampaignRepo) Add(c *domain.Campaign) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.campaigns[c.ID] = c
}

// Get возвращает кампанию по ID.
//
// Возвращает domain.ErrCampaignNotFound, если не найдена.
// Возвращает копию — вызывающий не может изменить данные в обход мьютекса.
//
// ВНИМАНИЕ: domain.Campaign сейчас состоит только из value-полей,
// поэтому shallow copy безопасно. Если добавятся указатели или слайсы —
// надо заменить на .Clone() (см. domain.Creative.Clone).
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
//
// Уменьшает budget_remaining, увеличивает budget_reserved.
// Возвращает ErrCampaignNotFound или ErrInsufficientBudget.
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

// Commit подтверждает списание резерва кампании.
//
// Уменьшает budget_reserved. НЕ списывает с advertiser —
// за это отвечает AdvertiserRepo.Spend, который вызывает сервис.
func (r *CampaignRepo) Commit(_ context.Context, campaignID string, amount int64) error {
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
	return nil
}

// Uncommit отменяет Commit — возвращает деньги из budget_reserved
// обратно в budget_reserved... нет, увеличивает reserved.
//
// Используется сервисом для компенсации, если после успешного
// Commit не удалось списать деньги с advertiser'а.
func (r *CampaignRepo) Uncommit(_ context.Context, campaignID string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.campaigns[campaignID]
	if !ok {
		return domain.ErrCampaignNotFound
	}

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
