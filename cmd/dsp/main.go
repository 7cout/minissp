// Command dsp запускает gRPC-сервер DSP (эмулятор Demand-Side Platform).
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

	"github.com/7cout/minissp/internal/dsp/handler"
	"github.com/7cout/minissp/internal/dsp/repository/memory"
	"github.com/7cout/minissp/internal/dsp/seed"
	"github.com/7cout/minissp/internal/dsp/service"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

const (
	defaultAddr     = ":50052"
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

	// Memory-репозитории.
	advertisers := memory.NewAdvertiserRepo()
	campaigns := memory.NewCampaignRepo()
	creatives := memory.NewCreativeRepo()

	if err := seed.Populate(ctx, advertisers, campaigns, creatives); err != nil {
		return fmt.Errorf("seed memory: %w", err)
	}
	slog.Info("seed data loaded",
		"advertisers", len(seed.AdvertiserIDs()),
		"campaigns", len(seed.CampaignIDs()),
		"creatives", len(seed.CreativeIDs()),
	)

	txManager := memory.NewTxManager(campaigns, advertisers)

	// Стратегия ставки — из env, дефолт 150%.
	multiplier := getEnvInt("DSP_BID_MULTIPLIER_PERCENT", 150)

	// Сервис DSP.
	svc := service.New(campaigns, creatives, txManager, multiplier)

	// API-key аутентификация.
	apiKeys := splitEnvList("DSP_API_KEYS")
	validator := handler.NewStaticAPIKeyValidator(apiKeys)
	if len(apiKeys) == 0 {
		slog.Warn("DSP_API_KEYS is empty — authentication is DISABLED")
	} else {
		slog.Info("DSP auth enabled", "keys_count", len(apiKeys))
	}

	// gRPC.
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(handler.AuthInterceptor(validator)),
	)
	pb.RegisterDspServiceServer(grpcServer, handler.NewDspServer(svc))
	reflection.Register(grpcServer)

	addr := getEnv("DSP_ADDR", defaultAddr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

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

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

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
