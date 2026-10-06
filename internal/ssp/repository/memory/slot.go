package memory

import (
	"context"
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
// Если слот с таким ID уже есть — перезаписывает.
func (r *SlotRepo) Add(slot *domain.Slot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots[slot.ID] = slot
}

// AddIfAbsent атомарно добавляет слот, если с таким (publisher_id, name)
// ещё нет. Возвращает существующий слот и inserted=false, если такой
// уже был.
//
// В отличие от Add, не перезаписывает по ID — защищает от дублей
// при конкурентной регистрации.
//
// При переходе на PostgreSQL этот метод реализуется через
// INSERT ... ON CONFLICT (publisher_id, name) DO NOTHING RETURNING.
// Требуется UNIQUE-индекс на (publisher_id, name).
func (r *SlotRepo) AddIfAbsent(slot *domain.Slot) (existing *domain.Slot, inserted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.slots {
		if s.PublisherID == slot.PublisherID && s.Name == slot.Name {
			return cloneSlot(s), false
		}
	}

	r.slots[slot.ID] = slot
	return cloneSlot(slot), true
}

// Get возвращает слот по ID.
//
// Возвращает domain.ErrSlotNotFound, если слот не найден.
// Возвращает глубокую копию — вызывающий не может изменить
// данные в обход мьютекса.
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
