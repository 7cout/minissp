package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// Impression обрабатывает подтверждение показа.
//
// Идемпотентно: повторный вызов с тем же auction_id вернёт nil,
// но повторного списания не будет.
func (s *Service) Impression(ctx context.Context, auctionID string) error {
	// Уже обрабатывали — идемпотентный повтор.
	if s.isProcessed(auctionID) {
		return nil
	}

	record, ok := s.takeAuction(auctionID)
	if !ok {
		return domain.ErrAuctionNotFound
	}

	if err := s.commitToBidder(ctx, record); err != nil {
		// Возвращаем запись, чтобы можно было повторить.
		s.storeAuction(record)
		return fmt.Errorf("commit to bidder %s: %w", record.BidderID, err)
	}

	// Начисляем publisher'у долю.
	publisherShare := record.Price * (100 - CommissionPercent) / 100
	if err := s.publishers.AddBalance(ctx, record.PublisherID, publisherShare); err != nil {
		slog.ErrorContext(ctx, "failed to add publisher balance",
			"publisher_id", record.PublisherID,
			"auction_id", auctionID,
			"error", err,
		)
		// Коммит уже прошёл — деньги списаны. Возвращаем ошибку,
		// чтобы вызывающий знал о проблеме.
		return fmt.Errorf("add publisher balance: %w", err)
	}

	s.markProcessed(auctionID)
	return nil
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
