package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// RedisSlotCache — кэш слотов в Redis с TTL.
type RedisSlotCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisSlotCache создаёт кэш с заданным TTL.
func NewRedisSlotCache(client *redis.Client, ttl time.Duration) *RedisSlotCache {
	return &RedisSlotCache{
		client: client,
		ttl:    ttl,
	}
}

// Get возвращает слот из Redis.
//
// Возвращает ErrCacheMiss, если ключа нет.
func (c *RedisSlotCache) Get(ctx context.Context, publisherID, name string) (*domain.Slot, error) {
	key := cacheKey(publisherID, name)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var slot domain.Slot
	if err := json.Unmarshal(data, &slot); err != nil {
		// Битые данные — не отдаём ошибку наверх, лучше считаем промахом.
		return nil, fmt.Errorf("unmarshal slot %s: %w", key, err)
	}
	return &slot, nil
}

// Put кладёт слот в Redis.
func (c *RedisSlotCache) Put(ctx context.Context, slot *domain.Slot) error {
	key := cacheKey(slot.PublisherID, slot.Name)

	data, err := json.Marshal(slot)
	if err != nil {
		return fmt.Errorf("marshal slot %s: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

// Invalidate удаляет слот из Redis.
func (c *RedisSlotCache) Invalidate(ctx context.Context, publisherID, name string) error {
	key := cacheKey(publisherID, name)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del %s: %w", key, err)
	}
	return nil
}

// cacheKey формирует ключ в Redis.
func cacheKey(publisherID, name string) string {
	return "slot:" + publisherID + ":" + name
}
