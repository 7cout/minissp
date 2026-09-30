package memory

import (
	"context"
	"sync"

	"github.com/7cout/minissp/internal/auction/domain"
)

// CreativeRepo - in-memory реализация CreativeRepository
type CreativeRepo struct {
	mu        sync.RWMutex
	creatives map[string]*domain.Creative
}

// NewCreativeRepo создаёт пустой репозиторий креативов
func NewCreativeRepo() *CreativeRepo {
	return &CreativeRepo{
		creatives: make(map[string]*domain.Creative),
	}
}

// Add добавляет креатив в репозиторий
func (r *CreativeRepo) Add(c *domain.Creative) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creatives[c.ID] = c
}

// ListByCampaign возвращает все креативы указанной кампании
func (r *CreativeRepo) ListByCampaign(_ context.Context, campaignID string) ([]domain.Creative, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Creative
	for _, c := range r.creatives {
		if c.CampaignID != campaignID {
			continue
		}
		result = append(result, *c)
	}
	return result, nil
}
