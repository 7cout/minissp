//go:build integration

package reserve_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/7cout/minissp/internal/ssp/reserve"
)

// integrationRedisDB — отдельная БД в Redis (индекс 15).
//
// Зачем отдельная:
//   - DB=0 занята slot cache и остальными dev-данными;
//   - тест делает FlushDB перед каждым сценарием, чтобы гарантировать
//     чистое состояние — на DB=0 это уничтожило бы чужие ключи.
//
// ВАЖНО: не запускай против прода/стейджа, где Redis слушает на DB=15.
// На dev-контейнере и в CI — безопасно.
const integrationRedisDB = 15

// TestContract_RealRedis прогоняет тот же набор сценариев, что и
// TestContract (см. contract_test.go), но против настоящего Redis
// из docker-compose вместо miniredis.
//
// Зачем нужен отдельный прогон:
//   - miniredis — это переписанный на Go «эмулятор», а не оригинал.
//     Тонкие расхождения в Lua, поведении ZADD/SETEX, обработке nil
//     от redis.call() и т.п. теоретически возможны.
//   - Реальный Redis проверит, что Lua-скрипты и типы команд работают
//     так, как мы ожидаем.
//
// Если Redis недоступен — тест скипается, чтобы не ломать
// `task test:integration` у тех, кто поднимает только Postgres.
func TestContract_RealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   integrationRedisDB,
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		t.Skipf("redis not available at %s (db=%d): %v",
			addr, integrationRedisDB, err)
	}

	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	// Общий для всех реализаций набор — тот же, что у memory и miniredis.
	for _, tc := range contractTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Сценарии рассчитывают на пустой Redis.
			if err := client.FlushDB(context.Background()).Err(); err != nil {
				t.Fatalf("flush db before %s: %v", tc.name, err)
			}

			m := reserve.NewRedis(client, reserve.RedisOptions{
				TTL:           contractTTL,
				ProcessedTTL:  time.Minute,
				ScanInterval:  time.Millisecond,
				TickBatchSize: 100,
				KeyPrefix:     "reserve",
				ShardTag:      "test",
			})

			tc.run(t, m)
		})
	}
}
