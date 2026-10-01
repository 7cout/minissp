// Command auction запускает gRPC-сервер аукциона.
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

	"github.com/7cout/minissp/internal/auction/bidder"
	"github.com/7cout/minissp/internal/auction/handler"
	"github.com/7cout/minissp/internal/auction/repository/memory"
	"github.com/7cout/minissp/internal/auction/repository/postgres"
	"github.com/7cout/minissp/internal/auction/service"
	"github.com/7cout/minissp/internal/db"
	"github.com/7cout/minissp/internal/seed"
	pb "github.com/7cout/minissp/proto/gen/auction/v1"
)

const (
	grpcAddr        = ":50051"
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

	storage := os.Getenv("STORAGE")
	if storage == "" {
		storage = "postgres" // default
	}

	var (
		slots     service.SlotRepository
		campaigns service.CampaignRepository
		creatives service.CreativeRepository
	)

	switch storage {
	case "memory":
		memSlots := memory.NewSlotRepo()
		memCamps := memory.NewCampaignRepo()
		memCreatives := memory.NewCreativeRepo()
		seed.PopulateMemory(memSlots, memCamps, memCreatives)
		slots, campaigns, creatives = memSlots, memCamps, memCreatives
		slog.Info("using in-memory storage")

	case "postgres":
		pool, err := db.NewPostgresPool(ctx, db.PostgresConfig{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: os.Getenv("POSTGRES_DB"),
		})
		if err != nil {
			return err
		}
		defer pool.Close()

		slots = postgres.NewSlotRepo(pool)
		campaigns = postgres.NewCampaignRepo(pool)
		creatives = postgres.NewCreativeRepo(pool)
		slog.Info("using postgres storage")

	default:
		return fmt.Errorf("unknown STORAGE: %q", storage)
	}

	// Биддеры
	bidders := []service.BidderClient{
		bidder.NewSimulator(seed.CampaignNike, 1_000_000, 5_000_000),
		bidder.NewSimulator(seed.CampaignAdidas, 1_000_000, 8_000_000),
		bidder.NewSimulator(seed.CampaignPuma, 1_000_000, 12_000_000),
	}

	// Сервис
	svc := service.New(slots, campaigns, creatives, bidders)

	// gRPC
	grpcServer := grpc.NewServer()
	pb.RegisterAuctionServiceServer(grpcServer, handler.NewAuctionServer(svc))
	reflection.Register(grpcServer)

	// Слушаем
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}

	// Запуск
	errCh := make(chan error, 1)
	go func() {
		slog.Info("gRPC server listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// Ждём
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown
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
