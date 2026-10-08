// Command publisher запускает эмулятор рекламной площадки.
//
// Примеры:
//
//	go run ./cmd/publisher --rps 10 --duration 30s
//	go run ./cmd/publisher --rps 100 --duration 1m --verbose
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/7cout/minissp/internal/publisher"
	sspv1 "github.com/7cout/minissp/proto/gen/ssp/v1"
)

func main() {
	if err := run(); err != nil {
		slog.Error("publisher failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := publisher.ParseFlags()
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.NewClient(
		cfg.SSPAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial ssp %s: %w", cfg.SSPAddr, err)
	}
	defer func() { _ = conn.Close() }()

	client := sspv1.NewSspServiceClient(conn)
	logger := slog.Default()

	p := publisher.New(cfg, client, logger)
	return p.Run(ctx)
}
