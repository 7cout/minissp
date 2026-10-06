// Command ssp запускает gRPC-сервер SSP (Supply-Side Platform).
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

	"github.com/7cout/minissp/internal/ssp/bidder"
	"github.com/7cout/minissp/internal/ssp/handler"
	"github.com/7cout/minissp/internal/ssp/repository/memory"
	"github.com/7cout/minissp/internal/ssp/seed"
	"github.com/7cout/minissp/internal/ssp/service"
	pb "github.com/7cout/minissp/proto/gen/ssp/v1"
)

const (
	defaultAddr     = ":50051"
	shutdownTimeout = 10 * time.Second
	rollbackTick    = 5 * time.Second
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

	// 1. Memory-репозитории.
	publishers := memory.NewPublisherRepo()
	slots := memory.NewSlotRepo()
	seed.PopulateMemory(publishers, slots)
	slog.Info("seed data loaded", "publishers", 1, "slots", 1)

	// 2. Клиенты к DSP.
	bidders, err := buildBidders()
	if err != nil {
		return fmt.Errorf("build bidders: %w", err)
	}
	slog.Info("dsp clients ready", "count", len(bidders))

	// 3. Сервис SSP.
	svc := service.New(slots, publishers, bidders)
	defer func() {
		if err := svc.Close(); err != nil {
			slog.Error("close service", "error", err)
		}
	}()

	// 4. Rollback worker.
	svc.StartRollbackWorker(ctx, rollbackTick)

	// 5. gRPC-сервер с auth interceptor.
	authResolver := newPublisherResolver(publishers)
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

// buildBidders создаёт gRPC-клиентов ко всем известным DSP.
//
// Адреса и ключи берём из env. Для pet-проекта — три DSP
// с разными стратегиями ставок, но все указывают на один процесс.
func buildBidders() ([]service.BidderClient, error) {
	type dspConfig struct {
		name   string
		addr   string
		apiKey string
	}

	configs := []dspConfig{
		{
			name:   "dsp-nike",
			addr:   getEnv("DSP_NIKE_ADDR", "localhost:50052"),
			apiKey: getEnv("DSP_API_KEY", ""),
		},
	}

	var clients []service.BidderClient
	for _, cfg := range configs {
		c, err := bidder.NewClient(cfg.name, cfg.addr, cfg.apiKey)
		if err != nil {
			// Закрываем уже созданные клиенты при ошибке.
			for _, cl := range clients {
				_ = cl.Close()
			}
			return nil, fmt.Errorf("create bidder %s: %w", cfg.name, err)
		}
		clients = append(clients, c)
	}

	return clients, nil
}

// publisherResolver адаптирует PublisherRepo к интерфейсу
// handler.PublisherResolver.
type publisherResolver struct {
	repo *memory.PublisherRepo
}

func newPublisherResolver(repo *memory.PublisherRepo) *publisherResolver {
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
