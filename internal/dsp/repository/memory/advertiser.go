package memory

import (
	"context"
	"sync"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// AdvertiserRepo — in-memory реализация AdvertiserRepository.
type AdvertiserRepo struct {
	mu          sync.RWMutex
	advertisers map[string]*domain.Advertiser
}

// NewAdvertiserRepo создаёт новый репозиторий рекламодателей.
func NewAdvertiserRepo() *AdvertiserRepo {
	return &AdvertiserRepo{
		advertisers: make(map[string]*domain.Advertiser),
	}
}

// Add добавляет рекламодателя.
func (r *AdvertiserRepo) Add(a *domain.Advertiser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advertisers[a.ID] = a
}

// Get возвращает рекламодателя по ID.
//
// Возвращает domain.ErrAdvertiserNotFound, если не найден.
// Возвращает копию.
func (r *AdvertiserRepo) Get(_ context.Context, id string) (*domain.Advertiser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.advertisers[id]
	if !ok {
		return nil, domain.ErrAdvertiserNotFound
	}
	cp := *a
	return &cp, nil
}

// Spend списывает amount с баланса рекламодателя.
//
// Возвращает ErrAdvertiserNotFound или ErrInsufficientBalance.
func (r *AdvertiserRepo) Spend(_ context.Context, id string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.advertisers[id]
	if !ok {
		return domain.ErrAdvertiserNotFound
	}
	if a.Balance < amount {
		return domain.ErrInsufficientBalance
	}
	a.Balance -= amount
	return nil
}
