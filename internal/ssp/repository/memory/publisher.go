package memory

import (
	"context"
	"sync"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// PublisherRepo — in-memory реализация PublisherRepository.
type PublisherRepo struct {
	mu       sync.RWMutex
	byID     map[string]*domain.Publisher
	byAPIKey map[string]*domain.Publisher
}

// NewPublisherRepo создаёт пустой репозиторий издателей.
func NewPublisherRepo() *PublisherRepo {
	return &PublisherRepo{
		byID:     make(map[string]*domain.Publisher),
		byAPIKey: make(map[string]*domain.Publisher),
	}
}

// Add добавляет издателя.
func (r *PublisherRepo) Add(p *domain.Publisher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[p.ID] = p
	r.byAPIKey[p.APIKey] = p
}

// Get возвращает издателя по ID.
//
// Возвращает domain.ErrPublisherNotFound, если издатель не найден.
// Возвращает копию.
func (r *PublisherRepo) Get(_ context.Context, id string) (*domain.Publisher, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrPublisherNotFound
	}
	cp := *p
	return &cp, nil
}

// GetByAPIKey возвращает издателя по api-key.
//
// Возвращает domain.ErrPublisherNotFound, если ключ невалиден.
// Возвращает копию.
func (r *PublisherRepo) GetByAPIKey(_ context.Context, apiKey string) (*domain.Publisher, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.byAPIKey[apiKey]
	if !ok {
		return nil, domain.ErrPublisherNotFound
	}
	cp := *p
	return &cp, nil
}

// AddBalance увеличивает баланс издателя.
//
// Возвращает domain.ErrPublisherNotFound, если издатель не найден.
func (r *PublisherRepo) AddBalance(_ context.Context, id string, amount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.byID[id]
	if !ok {
		return domain.ErrPublisherNotFound
	}
	p.Balance += amount
	return nil
}
