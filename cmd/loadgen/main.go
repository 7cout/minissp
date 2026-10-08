// Command loadgen запускает N publisher'ов одновременно: сначала
// регистрирует N слотов на стороне SSP, потом запускает N
// параллельных циклов RunAuction → Impression со случайной частотой.
//
// Все publisher'ы используют один api-key seed'ового publisher'а
// (T2), то есть с точки зрения SSP это один издатель с N слотами —
// реалистичная модель: у одного приложения много рекламных мест.
//
// Примеры:
//
//	go run ./cmd/loadgen --count 20 --rps-min 1 --rps-max 10 --duration 30s
//	go run ./cmd/loadgen --count 50 --rps-min 5 --rps-max 20 --duration 1m --verbose
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/7cout/minissp/internal/publisher"
	sspv1 "github.com/7cout/minissp/proto/gen/ssp/v1"
)

const apiKeyMetadata = "api-key"

// config — параметры нагрузочного запуска.
type config struct {
	SSPAddr         string
	APIKey          string
	Count           int
	RPSMin          int
	RPSMax          int
	Geo             string
	Duration        time.Duration
	ImpressionDelay time.Duration
	Verbose         bool
}

func parseFlags() config {
	var cfg config

	flag.StringVar(&cfg.SSPAddr, "ssp", "localhost:50051", "SSP gRPC address")
	flag.StringVar(&cfg.APIKey, "api-key", "", "publisher api-key (default from SSP_API_KEY/DSP_API_KEY env)")
	flag.IntVar(&cfg.Count, "count", 10, "number of concurrent publishers")
	flag.IntVar(&cfg.RPSMin, "rps-min", 1, "min RPS per publisher")
	flag.IntVar(&cfg.RPSMax, "rps-max", 10, "max RPS per publisher")
	flag.StringVar(&cfg.Geo, "geo", "RU", "geo for registered slots")
	flag.DurationVar(&cfg.Duration, "duration", 30*time.Second, "run duration")
	flag.DurationVar(&cfg.ImpressionDelay, "impression-delay", 50*time.Millisecond,
		"delay between RunAuction and Impression")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "log every request (very noisy at high count)")

	flag.Parse()

	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("SSP_API_KEY")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("DSP_API_KEY")
	}

	return cfg
}

func (c config) validate() error {
	if c.SSPAddr == "" {
		return fmt.Errorf("ssp address is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api-key is required")
	}
	if c.Count <= 0 {
		return fmt.Errorf("count must be positive, got %d", c.Count)
	}
	if c.RPSMin <= 0 || c.RPSMax < c.RPSMin {
		return fmt.Errorf("invalid RPS range: [%d, %d]", c.RPSMin, c.RPSMax)
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if c.ImpressionDelay < 0 {
		return fmt.Errorf("impression-delay cannot be negative")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("loadgen failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := parseFlags()
	if err := cfg.validate(); err != nil {
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

	// 1. Регистрируем N слотов.
	slotNames, err := registerSlots(ctx, client, cfg)
	if err != nil {
		return fmt.Errorf("register slots: %w", err)
	}
	slog.Info("slots registered", "count", len(slotNames))

	// 2. Готовим N publisher'ов со случайным RPS.
	pubs := make([]*publisher.Publisher, cfg.Count)
	perRPS := make([]int, cfg.Count)

	logger := publisherLogger(cfg.Verbose)

	for i := 0; i < cfg.Count; i++ {
		rps := cfg.RPSMin + rand.Intn(cfg.RPSMax-cfg.RPSMin+1)
		perRPS[i] = rps

		pcfg := publisher.Config{
			SSPAddr:         cfg.SSPAddr,
			APIKey:          cfg.APIKey,
			SlotName:        slotNames[i],
			RPS:             rps,
			ImpressionDelay: cfg.ImpressionDelay,
			// Промежуточные прогресс-логи публикуем раз в 5 минут —
			// при N=50 обычный 5-секундный интервал даёт спам.
			ReportInterval: 5 * time.Minute,
			Verbose:        cfg.Verbose,
		}

		if err := pcfg.Validate(); err != nil {
			return fmt.Errorf("invalid config for publisher %d: %w", i, err)
		}

		pubs[i] = publisher.New(pcfg, client, logger)
	}

	totalRPS := 0
	for _, r := range perRPS {
		totalRPS += r
	}
	slog.Info("starting publishers",
		"count", cfg.Count,
		"duration", cfg.Duration,
		"total_rps", totalRPS,
	)

	// 3. Запускаем всех параллельно.
	runCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	startedAt := time.Now()
	var wg sync.WaitGroup

	for i, p := range pubs {
		wg.Add(1)
		go func(idx int, p *publisher.Publisher) {
			defer wg.Done()

			// Небольшой jitter при старте, чтобы publisher'ы не
			// ударили по SSP одной волной.
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			select {
			case <-time.After(jitter):
			case <-runCtx.Done():
				return
			}

			if err := p.Run(runCtx); err != nil {
				slog.Warn("publisher failed", "idx", idx, "error", err)
			}
		}(i, p)
	}

	wg.Wait()

	// 4. Агрегированный отчёт.
	printAggregateReport(pubs, perRPS, time.Since(startedAt))

	return nil
}

// registerSlots регистрирует N слотов с именами load_slot_1..N.
//
// RegisterSlot идемпотентен: если слот уже существует у этого
// publisher'а, возвращается тот же. Повторный запуск loadgen не
// ломается.
func registerSlots(
	ctx context.Context,
	client sspv1.SspServiceClient,
	cfg config,
) ([]string, error) {
	names := make([]string, cfg.Count)
	for i := 0; i < cfg.Count; i++ {
		names[i] = fmt.Sprintf("load_slot_%d", i+1)
	}

	reqCtx := metadata.AppendToOutgoingContext(ctx, apiKeyMetadata, cfg.APIKey)

	for _, name := range names {
		_, err := client.RegisterSlot(reqCtx, &sspv1.RegisterSlotRequest{
			Name:     name,
			Geo:      cfg.Geo,
			MinPrice: 1_000_000,
			Type:     sspv1.CreativeType_CREATIVE_TYPE_BANNER,
			Banner:   &sspv1.Banner{Width: 320, Height: 50},
		})
		if err != nil {
			return nil, fmt.Errorf("register %q: %w", name, err)
		}
	}
	return names, nil
}

// publisherLogger возвращает логгер для publisher'ов.
//
// В обычном режиме publisher'ы молчат, чтобы не спамить промежуточными
// отчётами. С --verbose показывают всё.
func publisherLogger(verbose bool) *slog.Logger {
	if verbose {
		return slog.Default()
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// printAggregateReport суммирует статистику по всем publisher'ам.
func printAggregateReport(pubs []*publisher.Publisher, perRPS []int, elapsed time.Duration) {
	var (
		totalAttempted int64
		totalWon       int64
		totalFailed    int64
		totalImpOK     int64
		totalImpFailed int64
	)

	for _, p := range pubs {
		s := p.Stats()
		totalAttempted += s.AuctionsAttempted
		totalWon += s.AuctionsWon
		totalFailed += s.AuctionsFailed
		totalImpOK += s.ImpressionsOK
		totalImpFailed += s.ImpressionsFailed
	}

	totalRPS := 0
	for _, r := range perRPS {
		totalRPS += r
	}

	actualRPS := float64(0)
	if elapsed.Seconds() > 0 {
		actualRPS = float64(totalAttempted) / elapsed.Seconds()
	}

	slog.Info("aggregate report",
		"publishers", len(pubs),
		"target_total_rps", totalRPS,
		"elapsed", elapsed.Round(time.Second),
		"actual_rps", fmt.Sprintf("%.1f", actualRPS),
		"auctions_attempted", totalAttempted,
		"auctions_won", totalWon,
		"auctions_failed", totalFailed,
		"impressions_ok", totalImpOK,
		"impressions_failed", totalImpFailed,
	)
}
