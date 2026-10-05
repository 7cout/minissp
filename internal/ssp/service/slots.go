package service

import (
	"context"
	"errors"
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
// Идемпотентно: если слот с таким именем уже есть у этого
// Publisher'а — возвращает существующий.
func (s *Service) RegisterSlot(ctx context.Context, publisherID string, req domain.Slot) (*domain.Slot, error) {
	existing, err := s.slots.GetByName(ctx, publisherID, req.Name)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrSlotNotFound) {
		return nil, fmt.Errorf("lookup slot by name: %w", err)
	}

	req.ID = uuid.NewString()
	req.PublisherID = publisherID

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid slot: %w", err)
	}

	s.slots.Add(&req)
	return &req, nil
}
