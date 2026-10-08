package events

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store — хранилище исходящих событий (outbox).
//
// Запись попадает в Store в одной транзакции с бизнес-операцией.
// Воркер читает неопубликованные записи и публикует их через Publisher.
type Store interface {
	// Enqueue сохраняет событие в outbox.
	//
	// Для PostgresStore этот метод должен вызываться в той же
	// транзакции, что и бизнес-операция, — тогда outbox и бизнес
	// изменения консистентны. Обычно это делает TxManager, а не сервис
	// напрямую.
	Enqueue(ctx context.Context, event ImpressionEvent) error

	// FetchUnpublished возвращает до limit неопубликованных записей
	// в порядке появления.
	FetchUnpublished(ctx context.Context, limit int) ([]Record, error)

	// MarkPublished помечает запись как отправленную.
	MarkPublished(ctx context.Context, id string) error

	// MarkFailed регистрирует неудачную попытку: увеличивает attempts
	// и сохраняет текст ошибки.
	MarkFailed(ctx context.Context, id string, lastErr error) error
}

// Record — запись в outbox, готовая к публикации.
type Record struct {
	ID        string
	Topic     string
	Key       string
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
}

// MemoryStore — in-memory реализация Store.
//
// Используется для тестов и STORAGE=memory. Не переживает рестарт,
// не даёт настоящей атомарности с балансами — это dev-режим.
type MemoryStore struct {
	mu      sync.Mutex
	records map[string]*memoryRecord
}

type memoryRecord struct {
	rec       Record
	published bool
}

// NewMemoryStore создаёт пустой store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]*memoryRecord)}
}

// Enqueue сохраняет событие.
func (s *MemoryStore) Enqueue(_ context.Context, event ImpressionEvent) error {
	payload, err := event.Marshal()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	s.records[id] = &memoryRecord{
		rec: Record{
			ID:        id,
			Topic:     event.Topic(),
			Key:       event.Key(),
			Payload:   payload,
			CreatedAt: time.Now(),
		},
	}
	return nil
}

// FetchUnpublished возвращает неопубликованные записи.
func (s *MemoryStore) FetchUnpublished(_ context.Context, limit int) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Record, 0, limit)
	for _, r := range s.records {
		if r.published {
			continue
		}
		out = append(out, r.rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// MarkPublished помечает запись опубликованной.
func (s *MemoryStore) MarkPublished(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.records[id]; ok {
		r.published = true
	}
	return nil
}

// MarkFailed увеличивает attempts.
func (s *MemoryStore) MarkFailed(_ context.Context, id string, _ error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.records[id]; ok {
		r.rec.Attempts++
	}
	return nil
}
