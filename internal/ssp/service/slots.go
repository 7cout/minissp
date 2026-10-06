package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// GetSlotByName возвращает слот по имени издателя.
func (s *Service) GetSlotByName(ctx context.Context, publisherID, name string) (*domain.Slot, error) {
	return s.slots.GetByName(ctx, publisherID, name)
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

	slot, _ := s.slots.AddIfAbsent(&req)
	return slot, nil
}
