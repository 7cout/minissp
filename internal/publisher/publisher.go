package publisher

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sspv1 "github.com/7cout/minissp/proto/gen/ssp/v1"
)

// apiKeyMetadata — имя поля в gRPC metadata для api-key.
const apiKeyMetadata = "api-key"

// SspClient — узкий интерфейс к SSP. Специально не используем полный
// sspv1.SspServiceClient: Publisher'у нужны только эти четыре метода.
// Так легче мокать в тестах и соблюдается Interface Segregation.
type SspClient interface {
	GetSlotByName(ctx context.Context, in *sspv1.GetSlotByNameRequest, opts ...grpc.CallOption) (*sspv1.Slot, error)
	RunAuction(ctx context.Context, in *sspv1.BidRequest, opts ...grpc.CallOption) (*sspv1.BidResponse, error)
	Impression(ctx context.Context, in *sspv1.ImpressionRequest, opts ...grpc.CallOption) (*sspv1.ImpressionResponse, error)
	GetPublisherBalance(ctx context.Context, in *sspv1.GetPublisherBalanceRequest, opts ...grpc.CallOption) (*sspv1.GetPublisherBalanceResponse, error)
}

// Stats — снимок счётчиков.
type Stats struct {
	AuctionsAttempted int64
	AuctionsWon       int64
	AuctionsFailed    int64
	ImpressionsOK     int64
	ImpressionsFailed int64
}

// counters — внутренние атомарные счётчики.
type counters struct {
	auctionsAttempted atomic.Int64
	auctionsWon       atomic.Int64
	auctionsFailed    atomic.Int64
	impressionsOK     atomic.Int64
	impressionsFailed atomic.Int64
}

// Publisher — эмулятор площадки.
type Publisher struct {
	cfg    Config
	client SspClient
	logger *slog.Logger
	slotID string
	counts counters
}

// New создаёт Publisher. slotID заполняется позже, в Run — резолвится
// через GetSlotByName.
func New(cfg Config, client SspClient, logger *slog.Logger) *Publisher {
	return &Publisher{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

// Stats возвращает снимок счётчиков.
func (p *Publisher) Stats() Stats {
	return Stats{
		AuctionsAttempted: p.counts.auctionsAttempted.Load(),
		AuctionsWon:       p.counts.auctionsWon.Load(),
		AuctionsFailed:    p.counts.auctionsFailed.Load(),
		ImpressionsOK:     p.counts.impressionsOK.Load(),
		ImpressionsFailed: p.counts.impressionsFailed.Load(),
	}
}

// Run резолвит слот, читает стартовый баланс, крутит цикл RunAuction →
// Impression, по завершении пишет финальный отчёт.
//
// Возвращает ошибку только если что-то не удалось при инициализации
// (слот не найден, нет соединения). Ошибки отдельных запросов не
// прерывают цикл — они учитываются в счётчиках.
func (p *Publisher) Run(ctx context.Context) error {
	// 1. Резолвим слот.
	slotCtx := withAuth(ctx, p.cfg.APIKey)
	slot, err := p.client.GetSlotByName(slotCtx, &sspv1.GetSlotByNameRequest{
		Name: p.cfg.SlotName,
	})
	if err != nil {
		return fmt.Errorf("get slot %q: %w", p.cfg.SlotName, err)
	}
	p.slotID = slot.GetId()

	p.logger.Info("slot resolved",
		"name", slot.GetName(),
		"id", slot.GetId(),
		"geo", slot.GetGeo(),
		"min_price", slot.GetMinPrice(),
	)

	// 2. Стартовый баланс.
	initialBalance := p.readBalance(ctx)
	p.logger.Info("initial balance", "balance", initialBalance)

	// 3. Цикл.
	runCtx := ctx
	if p.cfg.Duration > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, p.cfg.Duration)
		defer cancel()
	}

	p.loop(runCtx, initialBalance)

	// 4. Финальный отчёт.
	finalBalance := p.readBalance(context.Background())
	p.logFinalReport(initialBalance, finalBalance)

	return nil
}

// loop — основной цикл с тикерами.
func (p *Publisher) loop(ctx context.Context, initialBalance int64) {
	auctionTicker := time.NewTicker(p.cfg.Interval())
	reportTicker := time.NewTicker(p.cfg.ReportInterval)
	defer auctionTicker.Stop()
	defer reportTicker.Stop()

	startedAt := time.Now()

	p.logger.Info("publisher started",
		"rps", p.cfg.RPS,
		"interval", p.cfg.Interval(),
		"impression_delay", p.cfg.ImpressionDelay,
	)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("publisher stopped", "reason", ctx.Err())
			return
		case <-auctionTicker.C:
			p.Tick(ctx)
		case <-reportTicker.C:
			p.logProgress(ctx, startedAt, initialBalance)
		}
	}
}

// Tick — одна итерация: RunAuction → (пауза) → Impression.
//
// Экспортирован, чтобы тесты могли вызывать его напрямую,
// без ожидания тикеров.
func (p *Publisher) Tick(ctx context.Context) {
	p.counts.auctionsAttempted.Add(1)

	reqID := uuid.NewString()
	bidCtx := withAuth(ctx, p.cfg.APIKey)

	resp, err := p.client.RunAuction(bidCtx, &sspv1.BidRequest{
		RequestId: reqID,
		SlotId:    p.slotID,
		UserId:    randomUserID(),
	})
	if err != nil {
		p.counts.auctionsFailed.Add(1)
		if p.cfg.Verbose {
			p.logger.WarnContext(ctx, "run auction failed", "error", err, "request_id", reqID)
		}
		return
	}
	p.counts.auctionsWon.Add(1)

	if p.cfg.Verbose {
		p.logger.InfoContext(ctx, "auction won",
			"auction_id", resp.GetAuctionId(),
			"creative_url", resp.GetCreativeUrl(),
		)
	}

	// Симуляция «показа баннера».
	if p.cfg.ImpressionDelay > 0 {
		select {
		case <-time.After(p.cfg.ImpressionDelay):
		case <-ctx.Done():
			return
		}
	}

	impCtx := withAuth(ctx, p.cfg.APIKey)
	if _, err := p.client.Impression(impCtx, &sspv1.ImpressionRequest{
		AuctionId: resp.GetAuctionId(),
	}); err != nil {
		p.counts.impressionsFailed.Add(1)
		if p.cfg.Verbose {
			p.logger.WarnContext(ctx, "impression failed",
				"error", err,
				"auction_id", resp.GetAuctionId(),
			)
		}
		return
	}
	p.counts.impressionsOK.Add(1)
}

// logProgress логирует промежуточную статистику и текущий баланс.
func (p *Publisher) logProgress(ctx context.Context, startedAt time.Time, initialBalance int64) {
	balance := p.readBalance(ctx)
	elapsed := time.Since(startedAt)

	attempted := p.counts.auctionsAttempted.Load()
	actualRPS := float64(attempted) / elapsed.Seconds()

	p.logger.Info("progress",
		"elapsed", elapsed.Round(time.Second),
		"rps_actual", fmt.Sprintf("%.1f", actualRPS),
		"auctions_attempted", attempted,
		"auctions_won", p.counts.auctionsWon.Load(),
		"auctions_failed", p.counts.auctionsFailed.Load(),
		"impressions_ok", p.counts.impressionsOK.Load(),
		"impressions_failed", p.counts.impressionsFailed.Load(),
		"balance", balance,
		"balance_delta", balance-initialBalance,
	)
}

// logFinalReport — финальный отчёт.
func (p *Publisher) logFinalReport(initialBalance, finalBalance int64) {
	s := p.Stats()
	p.logger.Info("final report",
		"auctions_attempted", s.AuctionsAttempted,
		"auctions_won", s.AuctionsWon,
		"auctions_failed", s.AuctionsFailed,
		"impressions_ok", s.ImpressionsOK,
		"impressions_failed", s.ImpressionsFailed,
		"balance_initial", initialBalance,
		"balance_final", finalBalance,
		"balance_delta", finalBalance-initialBalance,
	)
}

// readBalance запрашивает баланс publisher'а. Ошибки не фатальны.
func (p *Publisher) readBalance(ctx context.Context) int64 {
	balanceCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := p.client.GetPublisherBalance(
		withAuth(balanceCtx, p.cfg.APIKey),
		&sspv1.GetPublisherBalanceRequest{},
	)
	if err != nil {
		p.logger.WarnContext(ctx, "get balance failed", "error", err)
		return 0
	}
	return resp.GetBalance()
}

// withAuth добавляет api-key в gRPC metadata.
func withAuth(ctx context.Context, apiKey string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, apiKeyMetadata, apiKey)
}

// randomUserID — синтетический user_id.
func randomUserID() string {
	return "user_" + uuid.NewString()[:8]
}
