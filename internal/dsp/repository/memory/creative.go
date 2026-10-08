package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CreativeRepo — in-memory реализация CreativeRepository.
type CreativeRepo struct {
	mu        sync.RWMutex
	creatives map[string]*domain.Creative
}

// NewCreativeRepo создаёт пустой репозиторий креативов.
func NewCreativeRepo() *CreativeRepo {
	return &CreativeRepo{
		creatives: make(map[string]*domain.Creative),
	}
}

// Add добавляет креатив.
//
// Возвращает ошибку при пустом ID или дубликате.
func (r *CreativeRepo) Add(_ context.Context, c *domain.Creative) error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("creative id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.creatives[c.ID]; exists {
		return fmt.Errorf("creative with id %s already exists", c.ID)
	}

	r.creatives[c.ID] = c
	return nil
}

// Get возвращает креатив по ID.
//
// Возвращает domain.ErrCreativeNotFound, если не найден.
// Возвращает копию.
func (r *CreativeRepo) Get(_ context.Context, id string) (*domain.Creative, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.creatives[id]
	if !ok {
		return nil, domain.ErrCreativeNotFound
	}
	cp := c.Clone()
	return &cp, nil
}

// ListByCampaign возвращает все креативы указанной кампании.
func (r *CreativeRepo) ListByCampaign(_ context.Context, campaignID string) ([]domain.Creative, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Creative
	for _, c := range r.creatives {
		if c.CampaignID != campaignID {
			continue
		}
		result = append(result, c.Clone())
	}
	return result, nil
}
