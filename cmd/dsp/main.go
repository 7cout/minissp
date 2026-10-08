// Command dsp запускает gRPC-сервер DSP (эмулятор Demand-Side Platform).
//
// Хранилище выбирается env STORAGE: memory (по умолчанию) или postgres.
// Для postgres seed не заливается автоматически — используй cmd/seed.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/7cout/minissp/internal/db"
	"github.com/7cout/minissp/internal/dsp/handler"
	"github.com/7cout/minissp/internal/dsp/repository/memory"
	dspostgres "github.com/7cout/minissp/internal/dsp/repository/postgres"
	"github.com/7cout/minissp/internal/dsp/seed"
	"github.com/7cout/minissp/internal/dsp/service"
	"github.com/7cout/minissp/internal/metrics"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

const (
	defaultAddr     = ":50052"
	defaultMetrics  = ":9101"
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Хранилище.
	campaigns, creatives, txManager, cleanup, err := buildStorage(ctx)
	if err != nil {
		return fmt.Errorf("build storage: %w", err)
	}
	defer cleanup()

	// 2. Metrics-сервер: HTTP на отдельном порту, чтобы не мешать gRPC.
	metricsAddr := getEnv("METRICS_ADDR", defaultMetrics)
	metricsErrCh := make(chan error, 1)
	go func() {
		if err := metrics.Serve(ctx, metricsAddr); err != nil {
			metricsErrCh <- err
		}
	}()

	// 3. Сервис DSP.
	multiplier := getEnvInt("DSP_BID_MULTIPLIER_PERCENT", 150)
	svc := service.New(campaigns, creatives, txManager, multiplier)

	// 4. API-key аутентификация.
	apiKeys := splitEnvList("DSP_API_KEYS")
	validator := handler.NewStaticAPIKeyValidator(apiKeys)
	if len(apiKeys) == 0 {
		slog.Warn("DSP_API_KEYS is empty — authentication is DISABLED")
	} else {
		slog.Info("DSP auth enabled", "keys_count", len(apiKeys))
	}

	// 5. gRPC.
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(handler.AuthInterceptor(validator)),
	)
	pb.RegisterDspServiceServer(grpcServer, handler.NewDspServer(svc))
	reflection.Register(grpcServer)

	// 6. Слушаем.
	addr := getEnv("DSP_ADDR", defaultAddr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	// 7. Запуск.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("DSP gRPC server listening",
			"addr", addr,
			"bid_multiplier_percent", multiplier,
		)
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// 8. Ждём сигнала или ошибки.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	case err := <-metricsErrCh:
		// Метрики — вспомогательная функция. Если упали, сервис
		// продолжает работать. Логируем и идём дальше.
		slog.Error("metrics server failed", "error", err)
	}

	// 9. Graceful shutdown.
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("server stopped gracefully")
	case <-time.After(shutdownTimeout):
		slog.Warn("graceful shutdown timeout, forcing stop")
		grpcServer.Stop()
	}

	return nil
}

// buildStorage выбирает реализацию репозиториев по env STORAGE.
//
// Для memory заливает seed-данные при старте. Для postgres seed
// не заливается — это отдельная команда cmd/seed.
//
// Возвращает campaigns, creatives, txManager и cleanup-функцию.
func buildStorage(ctx context.Context) (
	service.CampaignRepository,
	service.CreativeRepository,
	service.TransactionManager,
	func(),
	error,
) {
	switch getEnv("STORAGE", "memory") {
	case "postgres":
		cfg := db.PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "minissp"),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			Database: getEnv("POSTGRES_DB", "minissp"),
		}
		pool, err := db.NewPostgresPool(ctx, cfg)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("postgres pool: %w", err)
		}
		slog.Info("storage: postgres")

		campaigns := dspostgres.NewCampaignRepo(pool)
		creatives := dspostgres.NewCreativeRepo(pool)
		txManager := dspostgres.NewTxManager(pool)
		return campaigns, creatives, txManager, pool.Close, nil

	default:
		slog.Info("storage: memory")
		advertisers := memory.NewAdvertiserRepo()
		campaigns := memory.NewCampaignRepo()
		creatives := memory.NewCreativeRepo()

		if err := seed.Populate(ctx, advertisers, campaigns, creatives); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("seed memory: %w", err)
		}
		slog.Info("seed data loaded",
			"advertisers", len(seed.AdvertiserIDs()),
			"campaigns", len(seed.CampaignIDs()),
			"creatives", len(seed.CreativeIDs()),
		)

		txManager := memory.NewTxManager(campaigns, advertisers)
		return campaigns, creatives, txManager, func() {}, nil
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func splitEnvList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
