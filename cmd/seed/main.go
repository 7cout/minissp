// Command seed заливает seed-данные в PostgreSQL.
//
// Отдельная команда, а не часть запуска сервисов: seed — это не схема,
// это данные для локальной разработки. Запускается один раз после
// migrate:up.
//
// Заливает оба домена: SSP (publishers, ad_slots) и DSP (advertisers,
// campaigns, creatives).
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
	dspostgres "github.com/7cout/minissp/internal/dsp/repository/postgres"
	dspseed "github.com/7cout/minissp/internal/dsp/seed"
	ssppostgres "github.com/7cout/minissp/internal/ssp/repository/postgres"
	sspseed "github.com/7cout/minissp/internal/ssp/seed"
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

	// --- SSP ---

	sspPublishers := ssppostgres.NewPublisherRepo(pool)
	sspSlots := ssppostgres.NewSlotRepo(pool)

	if err := sspseed.Populate(ctx, sspPublishers, sspSlots); err != nil {
		return fmt.Errorf("seed ssp: %w", err)
	}
	slog.Info("ssp seed loaded")

	// --- DSP ---

	dspAdvertisers := dspostgres.NewAdvertiserRepo(pool)
	dspCampaigns := dspostgres.NewCampaignRepo(pool)
	dspCreatives := dspostgres.NewCreativeRepo(pool)

	if err := dspseed.Populate(ctx, dspAdvertisers, dspCampaigns, dspCreatives); err != nil {
		return fmt.Errorf("seed dsp: %w", err)
	}
	slog.Info("dsp seed loaded")

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
