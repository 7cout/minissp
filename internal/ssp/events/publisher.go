package events

import "context"

// Publisher отправляет события во внешний брокер.
type Publisher interface {
	// Publish отправляет одну запись в топик.
	//
	// key используется для партиционирования (могут быть пустые).
	// Возвращает ошибку, если доставка не подтверждена брокером —
	// тогда воркер оставит запись в outbox и попробует ещё раз.
	Publish(ctx context.Context, topic, key string, payload []byte) error

	// Close освобождает ресурсы.
	Close() error
}

// NoopPublisher ничего не делает. Используется, когда Kafka не
// настроена (например, STORAGE=memory при локальной разработке).
type NoopPublisher struct{}

// Publish всегда возвращает nil.
func (NoopPublisher) Publish(_ context.Context, _, _ string, _ []byte) error { return nil }

// Close ничего не делает.
func (NoopPublisher) Close() error { return nil }
