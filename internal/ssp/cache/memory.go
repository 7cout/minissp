package cache

import (
	"context"
	"sync"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// MemorySlotCache — in-memory кэш слотов с TTL.
//
// Используется в тестах вместо Redis. Продовый код использует
// RedisSlotCache или NoopSlotCache.
type MemorySlotCache struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
	ttl     time.Duration
}

type memoryEntry struct {
	slot      domain.Slot
	expiresAt time.Time
}

// NewMemorySlotCache создаёт кэш с заданным TTL.
func NewMemorySlotCache(ttl time.Duration) *MemorySlotCache {
	return &MemorySlotCache{
		entries: make(map[string]memoryEntry),
		ttl:     ttl,
	}
}

// Get возвращает слот из кэша.
func (c *MemorySlotCache) Get(_ context.Context, publisherID, name string) (*domain.Slot, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(publisherID, name)
	entry, ok := c.entries[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	if time.Now().After(entry.expiresAt) {
		return nil, ErrCacheMiss
	}
	cp := entry.slot.Clone()
	return &cp, nil
}

// Put кладёт слот в кэш.
func (c *MemorySlotCache) Put(_ context.Context, slot *domain.Slot) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[cacheKey(slot.PublisherID, slot.Name)] = memoryEntry{
		slot:      slot.Clone(),
		expiresAt: time.Now().Add(c.ttl),
	}
	return nil
}

// Invalidate удаляет слот из кэша.
func (c *MemorySlotCache) Invalidate(_ context.Context, publisherID, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey(publisherID, name))
	return nil
}
