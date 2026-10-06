package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// StartRollbackWorker запускает фоновую задачу:
//   - откатывает просроченные аукционы в DSP;
//   - чистит старые записи об обработанных impression.
func (s *Service) StartRollbackWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("rollback worker stopped")
				return
			case <-ticker.C:
				s.processExpired(ctx)
				s.cleanProcessed(time.Now())
			}
		}
	}()
}

// processExpired откатывает просроченные аукционы.
func (s *Service) processExpired(ctx context.Context) {
	expired := s.listExpiredAuctions(time.Now())
	if len(expired) == 0 {
		return
	}

	slog.InfoContext(ctx, "rolling back expired auctions", "count", len(expired))

	for _, record := range expired {
		taken, ok := s.takeAuction(record.AuctionID)
		if !ok {
			// Запись уже забрана — Impression успел раньше, либо
			// другой тик воркера её обработал. Пропускаем.
			continue
		}

		if err := s.rollbackInBidder(ctx, taken); err != nil {
			slog.WarnContext(ctx, "failed to rollback auction",
				"auction_id", taken.AuctionID,
				"bidder", taken.BidderID,
				"error", err,
			)
			s.storeAuction(taken)
		}
	}
}

// rollbackInBidder отправляет Rollback конкретному биддеру.
func (s *Service) rollbackInBidder(ctx context.Context, record domain.AuctionRecord) error {
	bidder, ok := s.biddersBy[record.BidderID]
	if !ok {
		return nil // биддер удалён — не наша проблема
	}
	return bidder.Rollback(ctx, record.CampaignID, record.Price)
}
