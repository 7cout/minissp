package db

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// newTestRedisClient подключается к Redis из docker-compose.
// Если REDIS_ADDR не задан — тест скипается.
func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := "localhost:6379"
	client, err := NewRedisClient(context.Background(), RedisConfig{Addr: addr})
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedisClient_Ping(t *testing.T) {
	client := newTestRedisClient(t)

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Errorf("ping: %v", err)
	}
}

func TestRedisClient_ExpiredEvent(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()

	// Подписываемся на канал событий истечения.
	pubsub := client.PSubscribe(ctx, "__keyevent@0__:expired")
	t.Cleanup(func() { _ = pubsub.Close() })

	// Убеждаемся, что подписка установлена.
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Кладём ключ с коротким TTL.
	//
	// В той же DB работают тесты slot cache (internal/ssp/cache/redis_test.go),
	// которые тоже пишут ключи с TTL и получают события в этот же канал.
	// Поэтому не берём первое событие, а ждём именно свой ключ.
	key := "test:expired"
	if err := client.Set(ctx, key, "value", 100*time.Millisecond).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case msg := <-pubsub.Channel():
			if msg.Payload == key {
				return // нашли своё событие
			}
			// Чужое событие (например, из тестов slot cache) — игнорируем.
			t.Logf("skipping unrelated expired key: %q", msg.Payload)
		case <-deadline:
			t.Fatal("did not receive expired event for our key — check notify-keyspace-events=Ex")
		}
	}
}
