package memory

import (
	"context"
	"sync"

	"github.com/7cout/minissp/internal/auction/domain"
)

// SlotRepo — in-memory реализация SlotRepository
type SlotRepo struct {
	mu    sync.RWMutex
	slots map[string]*domain.Slot
}

// NewSlotRepo создаёт пустой репозиторий слотов
func NewSlotRepo() *SlotRepo {
	return &SlotRepo{slots: make(map[string]*domain.Slot)}
}

// Add добавляет слот в репозиторий
func (r *SlotRepo) Add(slot *domain.Slot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[slot.ID] = slot
}

// Get возвращает слот по ID
// Возвращает domain.ErrSlotNotFound, если слот не найден
func (r *SlotRepo) Get(_ context.Context, id string) (*domain.Slot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slot, ok := r.slots[id]
	if !ok {
		return nil, domain.ErrSlotNotFound
	}
	return slot, nil
}
