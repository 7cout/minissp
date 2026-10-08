package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig — параметры подключения к Redis.
type RedisConfig struct {
	Addr string
}

// NewRedisClient создаёт клиент Redis и проверяет соединение.
//
// go-redis сам управляет пулом соединений — отдельно настраивать
// не надо, дефолты рассчитаны на прод.
func NewRedisClient(ctx context.Context, cfg RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.Addr,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis %s: %w", cfg.Addr, err)
	}

	return client, nil
}
