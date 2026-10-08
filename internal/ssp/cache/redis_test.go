package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/7cout/minissp/internal/db"
)

// newTestRedisClient подключается к Redis из docker-compose.
// Если Redis недоступен — тест скипается.
func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	client, err := db.NewRedisClient(context.Background(), db.RedisConfig{
		Addr: "localhost:6379",
	})
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// flushCacheKey удаляет конкретный ключ перед тестом.
func flushCacheKey(t *testing.T, client *redis.Client, key string) {
	t.Helper()
	if err := client.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestRedisSlotCache_Get_Miss(t *testing.T) {
	client := newTestRedisClient(t)
	c := NewRedisSlotCache(client, time.Minute)

	flushCacheKey(t, client, "slot:pub_1:home_banner")

	_, err := c.Get(context.Background(), "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss, got %v", err)
	}
}

func TestRedisSlotCache_PutThenGet(t *testing.T) {
	client := newTestRedisClient(t)
	c := NewRedisSlotCache(client, time.Minute)
	ctx := context.Background()

	flushCacheKey(t, client, "slot:pub_1:home_banner")

	slot := testBannerSlot()
	if err := c.Put(ctx, slot); err != nil {
		t.Fatalf("put: %v", err)
	}
	t.Cleanup(func() { flushCacheKey(t, client, "slot:pub_1:home_banner") })

	got, err := c.Get(ctx, "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
	if got.Banner == nil || got.Banner.Width != 320 || got.Banner.Height != 50 {
		t.Errorf("banner = %+v, want 320x50", got.Banner)
	}
	if got.Type != "banner" {
		t.Errorf("type = %q, want banner", got.Type)
	}
}

func TestRedisSlotCache_TTLExpires(t *testing.T) {
	client := newTestRedisClient(t)
	c := NewRedisSlotCache(client, 100*time.Millisecond)
	ctx := context.Background()

	flushCacheKey(t, client, "slot:pub_1:home_banner")

	if err := c.Put(ctx, testBannerSlot()); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Первый Get сразу — должно быть попадание.
	if _, err := c.Get(ctx, "pub_1", "home_banner"); err != nil {
		t.Fatalf("get before TTL: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, err := c.Get(ctx, "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after TTL, got %v", err)
	}
}

func TestRedisSlotCache_Invalidate(t *testing.T) {
	client := newTestRedisClient(t)
	c := NewRedisSlotCache(client, time.Minute)
	ctx := context.Background()

	flushCacheKey(t, client, "slot:pub_1:home_banner")

	_ = c.Put(ctx, testBannerSlot())
	if err := c.Invalidate(ctx, "pub_1", "home_banner"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	_, err := c.Get(ctx, "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after invalidate, got %v", err)
	}
}
