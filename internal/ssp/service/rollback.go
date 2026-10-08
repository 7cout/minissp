package service

import (
	"context"
	"log/slog"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// StartReserveWorker запускает обработку просроченных резервов.
//
// Делегирует в reserve.Manager.Subscribe: для memory — тикер,
// сканирующий карту; для Redis — подписка на expired-события.
// Не блокирует: воркер работает в фоне до отмены ctx.
func (s *Service) StartReserveWorker(ctx context.Context) {
	s.reserve.Subscribe(ctx, func(record domain.AuctionRecord) error {
		if err := s.rollbackInBidder(ctx, record); err != nil {
			slog.WarnContext(ctx, "failed to rollback auction",
				"auction_id", record.AuctionID,
				"bidder", record.BidderID,
				"error", err,
			)
			return err
		}
		return nil
	})
}

// TickReserve обрабатывает просроченные резервы синхронно.
//
// В проде откат делает воркер через StartReserveWorker. Этот метод
// используется в тестах и для ручной отладки, чтобы не ждать
// следующего тика.
func (s *Service) TickReserve(ctx context.Context) {
	s.reserve.Tick(ctx, func(record domain.AuctionRecord) error {
		return s.rollbackInBidder(ctx, record)
	})
}

// rollbackInBidder отправляет Rollback конкретному биддеру.
func (s *Service) rollbackInBidder(ctx context.Context, record domain.AuctionRecord) error {
	bidder, ok := s.biddersBy[record.BidderID]
	if !ok {
		return nil // биддер удалён — не наша проблема
	}
	return bidder.Rollback(ctx, record.CampaignID, record.Price)
}
