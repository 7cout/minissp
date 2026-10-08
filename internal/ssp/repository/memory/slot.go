package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// SlotRepo — in-memory реализация SlotRepository.
type SlotRepo struct {
	mu    sync.RWMutex
	slots map[string]*domain.Slot // по slot_id
}

// NewSlotRepo создаёт пустой репозиторий слотов.
func NewSlotRepo() *SlotRepo {
	return &SlotRepo{
		slots: make(map[string]*domain.Slot),
	}
}

// Add добавляет слот.
//
// Возвращает ошибку при пустом ID или дубликате ID.
// Для конкурентной регистрации используй AddIfAbsent.
func (r *SlotRepo) Add(_ context.Context, slot *domain.Slot) error {
	if strings.TrimSpace(slot.ID) == "" {
		return errors.New("slot id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.slots[slot.ID]; exists {
		return fmt.Errorf("slot with id %s already exists", slot.ID)
	}

	r.slots[slot.ID] = slot
	return nil
}

// AddIfAbsent атомарно добавляет слот, если с таким (publisher_id, name)
// ещё нет. Возвращает существующий слот и inserted=false, если такой
// уже был.
//
// При переходе на PostgreSQL этот метод реализуется через
// INSERT ... ON CONFLICT (publisher_id, name) DO NOTHING RETURNING.
// Требуется UNIQUE-индекс на (publisher_id, name).
func (r *SlotRepo) AddIfAbsent(_ context.Context, slot *domain.Slot) (*domain.Slot, bool, error) {
	if strings.TrimSpace(slot.ID) == "" {
		return nil, false, errors.New("slot id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.slots {
		if s.PublisherID == slot.PublisherID && s.Name == slot.Name {
			return cloneSlot(s), false, nil
		}
	}

	if _, exists := r.slots[slot.ID]; exists {
		return nil, false, fmt.Errorf("slot with id %s already exists", slot.ID)
	}

	r.slots[slot.ID] = slot
	return cloneSlot(slot), true, nil
}

// Get возвращает слот по ID.
//
// Возвращает domain.ErrSlotNotFound, если слот не найден.
// Возвращает глубокую копию.
func (r *SlotRepo) Get(_ context.Context, id string) (*domain.Slot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slot, ok := r.slots[id]
	if !ok {
		return nil, domain.ErrSlotNotFound
	}
	return cloneSlot(slot), nil
}

// GetByName возвращает слот по publisher_id и имени.
//
// Возвращает domain.ErrSlotNotFound, если слот не найден.
// Возвращает глубокую копию.
func (r *SlotRepo) GetByName(_ context.Context, publisherID, name string) (*domain.Slot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, slot := range r.slots {
		if slot.PublisherID == publisherID && slot.Name == name {
			return cloneSlot(slot), nil
		}
	}
	return nil, domain.ErrSlotNotFound
}

// cloneSlot создаёт глубокую копию слота, включая вложенные параметры.
func cloneSlot(s *domain.Slot) *domain.Slot {
	cp := *s
	if s.Banner != nil {
		b := *s.Banner
		cp.Banner = &b
	}
	if s.Video != nil {
		v := *s.Video
		if s.Video.MIMEs != nil {
			v.MIMEs = append([]string(nil), s.Video.MIMEs...)
		}
		cp.Video = &v
	}
	if s.Native != nil {
		n := *s.Native
		cp.Native = &n
	}
	if s.Audio != nil {
		a := *s.Audio
		cp.Audio = &a
	}
	return &cp
}
