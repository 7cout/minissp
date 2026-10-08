package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/domain"
)

// GetSlotByName возвращает слот по имени издателя.
//
// Cache-aside: сначала смотрим в кэш, при промахе идём в репозиторий
// и кладём результат в кэш. Падение кэша не ломает бизнес —
// это деградация, а не отказ: работаем через репозиторий,
// пишем warning.
func (s *Service) GetSlotByName(ctx context.Context, publisherID, name string) (*domain.Slot, error) {
	// 1. Пробуем кэш.
	slot, err := s.slotCache.Get(ctx, publisherID, name)
	if err == nil {
		return slot, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		// Не промах, а проблема с кэшем (сеть, Redis).
		// Не блокируем бизнес — логируем и идём в репозиторий.
		slog.WarnContext(ctx, "slot cache get failed — falling back to repo",
			"publisher_id", publisherID,
			"name", name,
			"error", err,
		)
	}

	// 2. Промах — идём в репозиторий.
	slot, err = s.slots.GetByName(ctx, publisherID, name)
	if err != nil {
		return nil, err
	}

	// 3. Кладём в кэш (best-effort).
	if err := s.slotCache.Put(ctx, slot); err != nil {
		slog.WarnContext(ctx, "slot cache put failed",
			"publisher_id", publisherID,
			"name", name,
			"error", err,
		)
	}

	return slot, nil
}

// RegisterSlot регистрирует слот.
//
// Идемпотентно: если слот с таким именем уже есть у этого Publisher'а —
// возвращает существующий. Конкурентные вызовы с одним именем
// возвращают один и тот же слот.
func (s *Service) RegisterSlot(ctx context.Context, publisherID string, req domain.Slot) (*domain.Slot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req.ID = uuid.NewString()
	req.PublisherID = publisherID

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid slot: %w", err)
	}

	slot, _, err := s.slots.AddIfAbsent(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("add slot: %w", err)
	}
	return slot, nil
}
