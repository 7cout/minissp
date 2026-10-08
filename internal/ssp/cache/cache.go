package cache

import (
	"context"
	"errors"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// ErrCacheMiss — ключа в кэше нет или он истёк.
//
// Отдельная sentinel-ошибка, чтобы вызывающий мог отличить промах
// от реальной проблемы с кэшем (Redis недоступен, ошибка сети).
var ErrCacheMiss = errors.New("cache miss")

// SlotCache — кэш слотов по (publisher_id, name).
type SlotCache interface {
	// Get возвращает слот из кэша.
	//
	// Возвращает ErrCacheMiss, если ключа нет или TTL истёк.
	// Возвращает другую ошибку при проблеме с самим кэшем.
	Get(ctx context.Context, publisherID, name string) (*domain.Slot, error)

	// Put кладёт слот в кэш с TTL по умолчанию.
	Put(ctx context.Context, slot *domain.Slot) error

	// Invalidate удаляет слот из кэша.
	//
	// Вызывается при изменении слота. Сейчас не используется (слоты
	// только создаются), оставлено на будущее.
	Invalidate(ctx context.Context, publisherID, name string) error
}
