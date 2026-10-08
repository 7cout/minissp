package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/events"
	sspmetrics "github.com/7cout/minissp/internal/ssp/metrics"
	"github.com/7cout/minissp/internal/ssp/reserve"
)

// Impression обрабатывает подтверждение показа.
//
// Идемпотентно: параллельные или повторные вызовы с тем же auction_id
// возвращают nil, повторного списания не будет.
//
// Алгоритм:
//  1. Атомарно забираем запись из резерва (Consume).
//  2. Проверяем, что publisher существует.
//  3. Отправляем Commit в DSP — там списывается бюджет и деньги
//     advertiser'а.
//  4. В одной транзакции: начисляем publisher'у долю и кладём
//     событие Impression в outbox.
func (s *Service) Impression(ctx context.Context, auctionID string) (err error) {
	defer func() {
		sspmetrics.ImpressionsTotal.WithLabelValues(impressionStatus(err)).Inc()
	}()

	record, status, err := s.reserve.Consume(ctx, auctionID)
	if err != nil {
		if errors.Is(err, reserve.ErrNotFound) {
			return domain.ErrAuctionNotFound
		}
		return fmt.Errorf("consume reserve: %w", err)
	}
	if status == reserve.ConsumeAlready {
		return nil
	}

	// Предварительная проверка: publisher существует. Отсекает ошибку
	// до того, как мы дёрнули DSP. Если упало — возвращаем запись
	// в резерв, чтобы можно было повторить.
	if _, err := s.publishers.Get(ctx, record.PublisherID); err != nil {
		_ = s.reserve.Restore(ctx, record)
		return fmt.Errorf("get publisher %s: %w", record.PublisherID, err)
	}

	if err := s.commitToBidder(ctx, record); err != nil {
		_ = s.reserve.Restore(ctx, record)
		return fmt.Errorf("commit to bidder %s: %w", record.BidderID, err)
	}

	// Начисляем publisher'у долю и кладём событие в outbox —
	// атомарно, в одной транзакции.
	publisherShare := record.Price * (100 - CommissionPercent) / 100
	platformFee := record.Price - publisherShare

	evt := events.ImpressionEvent{
		AuctionID:      record.AuctionID,
		ImpID:          record.ImpID,
		CampaignID:     record.CampaignID,
		CreativeID:     record.CreativeID,
		PublisherID:    record.PublisherID,
		SlotID:         record.SlotID,
		BidderID:       record.BidderID,
		Price:          record.Price,
		PublisherShare: publisherShare,
		PlatformFee:    platformFee,
		CreatedAt:      record.CreatedAt,
		OccurredAt:     time.Now(),
	}

	if err := s.txManager.RecordImpression(ctx, record.PublisherID, publisherShare, evt); err != nil {
		slog.ErrorContext(ctx, "CRITICAL: commit succeeded but publisher balance not credited",
			"auction_id", auctionID,
			"publisher_id", record.PublisherID,
			"campaign_id", record.CampaignID,
			"price", record.Price,
			"publisher_share", publisherShare,
			"error", err,
		)
		return fmt.Errorf("record impression: %w", err)
	}

	return nil
}

// impressionStatus превращает ошибку Impression в метку для счётчика.
func impressionStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, domain.ErrAuctionNotFound):
		return "not_found"
	default:
		return "error"
	}
}

// commitToBidder отправляет Commit конкретному биддеру.
func (s *Service) commitToBidder(ctx context.Context, record domain.AuctionRecord) error {
	bidder, ok := s.biddersBy[record.BidderID]
	if !ok {
		return fmt.Errorf("bidder %q not found", record.BidderID)
	}
	return bidder.Commit(ctx, record.CampaignID, record.Price)
}

// GetPublisherBalance возвращает баланс издателя.
func (s *Service) GetPublisherBalance(ctx context.Context, publisherID string) (int64, error) {
	p, err := s.publishers.Get(ctx, publisherID)
	if err != nil {
		return 0, err
	}
	return p.Balance, nil
}
