// Command auction запускает gRPC-сервер аукциона.
//
// Использует in-memory репозитории и симуляторы биддеров —
// чтобы запуститься без PostgreSQL, Redis и Kafka.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/7cout/minissp/internal/auction/bidder"
	"github.com/7cout/minissp/internal/auction/domain"
	"github.com/7cout/minissp/internal/auction/handler"
	"github.com/7cout/minissp/internal/auction/repository/memory"
	"github.com/7cout/minissp/internal/auction/service"
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
	// Репозитории (in-memory).
	slots := memory.NewSlotRepo()
	campaigns := memory.NewCampaignRepo()
	creatives := memory.NewCreativeRepo()

	seedData(slots, campaigns, creatives)

	// Биддеры (симуляторы).
	bidders := []service.BidderClient{
		bidder.NewSimulator("camp_1", 1_000_000, 5_000_000),
		bidder.NewSimulator("camp_2", 1_000_000, 8_000_000),
		bidder.NewSimulator("camp_3", 1_000_000, 12_000_000),
	}

	// Сервис аукциона.
	svc := service.New(slots, campaigns, creatives, bidders)

	// gRPC-сервер.
	grpcServer := grpc.NewServer()
	pb.RegisterAuctionServiceServer(grpcServer, handler.NewAuctionServer(svc))
	reflection.Register(grpcServer) // для grpcurl

	// Слушаем порт.
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}

	// Graceful shutdown по Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 7. Запускаем сервер в горутине.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("gRPC server listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// Ждём сигнала или ошибки сервера
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown: даём время завершить текущие запросы
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

// seedData наполняет репозитории тестовыми данными
func seedData(
	slots *memory.SlotRepo,
	campaigns *memory.CampaignRepo,
	creatives *memory.CreativeRepo,
) {
	// Слот
	slots.Add(&domain.Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Banner:      domain.Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	})

	// Кампании
	for i, id := range []string{"camp_1", "camp_2", "camp_3"} {
		campaigns.Add(&domain.Campaign{
			ID:              id,
			AdvertiserID:    "adv_" + id,
			Name:            "Test Campaign " + id,
			BudgetTotal:     100_000_000_000,
			BudgetRemaining: 100_000_000_000,
			BudgetReserved:  0,
			GeoTarget:       "RU",
		})

		// Креатив для каждой кампании
		creatives.Add(&domain.Creative{
			ID:         "creative_" + id,
			CampaignID: id,
			Banner:     domain.Banner{Width: 320, Height: 50},
			URL:        "https://cdn.example.com/banner_" + id + ".jpg",
			ClickURL:   "https://example.com/click_" + id,
		})

		_ = i
	}
}
