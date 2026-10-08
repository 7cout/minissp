package memory

import (
	"context"
	"fmt"
	"strings"
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
//
// Возвращает ошибку при пустом ID или дубликате api_key.
func (r *PublisherRepo) Add(_ context.Context, p *domain.Publisher) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("publisher id is required")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return fmt.Errorf("publisher api_key is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[p.ID]; exists {
		return fmt.Errorf("publisher with id %s already exists", p.ID)
	}
	if _, exists := r.byAPIKey[p.APIKey]; exists {
		return fmt.Errorf("publisher with api_key already exists")
	}

	r.byID[p.ID] = p
	r.byAPIKey[p.APIKey] = p
	return nil
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
