// Command seed заливает seed-данные в PostgreSQL.
//
// Отдельная команда, а не часть запуска SSP: seed — это не схема,
// это данные для локальной разработки. Запускается один раз после
// migrate:up.
//
// Пример:
//
//	task seed
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/7cout/minissp/internal/db"
	ssppostgres "github.com/7cout/minissp/internal/ssp/repository/postgres"
	"github.com/7cout/minissp/internal/ssp/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := db.PostgresConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PORT", "5432"),
		User:     getEnv("POSTGRES_USER", "minissp"),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		Database: getEnv("POSTGRES_DB", "minissp"),
	}

	pool, err := db.NewPostgresPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	publishers := ssppostgres.NewPublisherRepo(pool)
	slots := ssppostgres.NewSlotRepo(pool)

	if err := seed.Populate(ctx, publishers, slots); err != nil {
		return fmt.Errorf("populate: %w", err)
	}

	slog.Info("seed data loaded")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
