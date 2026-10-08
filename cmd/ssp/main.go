// Command ssp запускает gRPC-сервер SSP (Supply-Side Platform).
//
// Хранилище выбирается env STORAGE: memory (по умолчанию) или postgres.
// Для postgres seed не заливается автоматически — используй cmd/seed.
//
// Кэш слотов: NoopSlotCache для memory, RedisSlotCache для postgres.
// Резервы аукционов: MemoryManager (in-memory) сейчас; RedisManager
// будет добавлен следующим шагом.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/7cout/minissp/internal/db"
	"github.com/7cout/minissp/internal/ssp/bidder"
	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/handler"
	"github.com/7cout/minissp/internal/ssp/repository/memory"
	ssppostgres "github.com/7cout/minissp/internal/ssp/repository/postgres"
	"github.com/7cout/minissp/internal/ssp/reserve"
	"github.com/7cout/minissp/internal/ssp/seed"
	"github.com/7cout/minissp/internal/ssp/service"
	pb "github.com/7cout/minissp/proto/gen/ssp/v1"
)

const (
	defaultAddr     = ":50051"
	shutdownTimeout = 10 * time.Second
	warmUpTimeout   = 3 * time.Second
	slotCacheTTL    = 5 * time.Minute
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

	// 1. Зависимости: репозитории, кэш, резервы.
	d, err := buildDeps(ctx)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	defer d.cleanup()

	// 2. Клиенты к DSP.
	bidders, err := buildBidders(ctx)
	if err != nil {
		return fmt.Errorf("build bidders: %w", err)
	}
	slog.Info("dsp clients ready", "count", len(bidders))

	// 3. Сервис SSP.
	svc := service.New(service.Options{
		Slots:      d.slots,
		Publishers: d.publishers,
		Bidders:    bidders,
		SlotCache:  d.slotCache,
		Reserve:    d.reserve,
	})
	defer func() {
		if err := svc.Close(); err != nil {
			slog.Error("close service", "error", err)
		}
	}()

	// 4. Worker отката просроченных резервов.
	svc.StartReserveWorker(ctx)

	// 5. gRPC-сервер с auth interceptor.
	authResolver := newPublisherResolver(d.publishers)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(handler.AuthInterceptor(authResolver)),
	)
	pb.RegisterSspServiceServer(grpcServer, handler.NewSspServer(svc))
	reflection.Register(grpcServer)

	// 6. Слушаем.
	addr := getEnv("SSP_ADDR", defaultAddr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	// 7. Запуск.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("SSP gRPC server listening", "addr", addr)
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

// deps — собранные зависимости SSP.
type deps struct {
	slots      service.SlotRepository
	publishers service.PublisherRepository
	slotCache  cache.SlotCache
	reserve    reserve.Manager
	cleanup    func()
}

// buildDeps выбирает реализацию репозиториев, кэша и резервов по env STORAGE.
func buildDeps(ctx context.Context) (deps, error) {
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
			return deps{}, fmt.Errorf("postgres pool: %w", err)
		}
		slog.Info("storage: postgres")

		slotCache, cacheCleanup := buildSlotCache(ctx)

		return deps{
			slots:      ssppostgres.NewSlotRepo(pool),
			publishers: ssppostgres.NewPublisherRepo(pool),
			slotCache:  slotCache,
			reserve:    reserve.NewMemory(reserve.DefaultMemoryOptions()),
			cleanup: func() {
				cacheCleanup()
				pool.Close()
			},
		}, nil

	default:
		slog.Info("storage: memory")
		publishers := memory.NewPublisherRepo()
		slots := memory.NewSlotRepo()
		if err := seed.Populate(ctx, publishers, slots); err != nil {
			return deps{}, fmt.Errorf("seed memory: %w", err)
		}
		slog.Info("seed data loaded", "publishers", 1, "slots", 1)

		return deps{
			slots:      slots,
			publishers: publishers,
			slotCache:  cache.NoopSlotCache{},
			reserve:    reserve.NewMemory(reserve.DefaultMemoryOptions()),
			cleanup:    func() {},
		}, nil
	}
}

// buildSlotCache подключается к Redis. Если недоступен — Noop.
func buildSlotCache(ctx context.Context) (cache.SlotCache, func()) {
	addr := getEnv("REDIS_ADDR", "localhost:6379")

	client, err := db.NewRedisClient(ctx, db.RedisConfig{Addr: addr})
	if err != nil {
		slog.Warn("redis not available — slot cache disabled",
			"addr", addr,
			"error", err,
		)
		return cache.NoopSlotCache{}, func() {}
	}

	slog.Info("slot cache: redis", "addr", addr)
	return cache.NewRedisSlotCache(client, slotCacheTTL), func() { _ = client.Close() }
}

// buildBidders создаёт gRPC-клиентов ко всем известным DSP
// и прогревает соединения.
func buildBidders(ctx context.Context) ([]service.BidderClient, error) {
	type dspConfig struct {
		name   string
		addr   string
		apiKey string
	}

	configs := []dspConfig{
		{
			name:   "dsp-nike",
			addr:   getEnv("DSP_NIKE_ADDR", "127.0.0.1:50052"),
			apiKey: getEnv("DSP_API_KEY", ""),
		},
	}

	var clients []service.BidderClient
	for _, cfg := range configs {
		c, err := bidder.NewClient(cfg.name, cfg.addr, cfg.apiKey)
		if err != nil {
			for _, cl := range clients {
				_ = cl.Close()
			}
			return nil, fmt.Errorf("create bidder %s: %w", cfg.name, err)
		}

		warmUpCtx, cancel := context.WithTimeout(ctx, warmUpTimeout)
		err = c.WarmUp(warmUpCtx)
		cancel()

		if err != nil {
			slog.Warn("dsp warm up failed — first RPC may be slow",
				"dsp", cfg.name,
				"addr", cfg.addr,
				"error", err,
			)
		} else {
			slog.Info("dsp connection ready", "dsp", cfg.name)
		}

		clients = append(clients, c)
	}

	return clients, nil
}

// publisherResolver адаптирует PublisherRepository к интерфейсу
// handler.PublisherResolver.
type publisherResolver struct {
	repo service.PublisherRepository
}

func newPublisherResolver(repo service.PublisherRepository) *publisherResolver {
	return &publisherResolver{repo: repo}
}

func (r *publisherResolver) Resolve(ctx context.Context, apiKey string) (string, error) {
	p, err := r.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
