package cache

import (
	"context"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// NoopSlotCache ничего не делает — всегда miss, Put и Invalidate no-op.
//
// Используется для STORAGE=memory: репозиторий в памяти,
// кэш не даст прироста, но сохранит одинаковый код в сервисе.
type NoopSlotCache struct{}

// Get всегда возвращает ErrCacheMiss.
func (NoopSlotCache) Get(_ context.Context, _, _ string) (*domain.Slot, error) {
	return nil, ErrCacheMiss
}

// Put ничего не делает.
func (NoopSlotCache) Put(_ context.Context, _ *domain.Slot) error { return nil }

// Invalidate ничего не делает.
func (NoopSlotCache) Invalidate(_ context.Context, _, _ string) error { return nil }
