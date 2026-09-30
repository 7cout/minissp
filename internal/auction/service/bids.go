package service

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/7cout/minissp/internal/auction/domain"
)

// bidTimeout - максимальное время ожидания ответа от всех биддеров
const bidTimeout = 100 * time.Millisecond

// collectBids опрашивает всех биддеров параллельно и собирает валидные ставки
//
// Биддеры, которые вернули ошибку или невалидную ставку, игнорируются -
// аукцион продолжается с теми, кто ответил. Если не ответил никто,
// возвращается пустой слайс (не ошибка)
//
// Ошибка возвращается только в случае отмены контекста
func (s *Service) collectBids(parentCtx context.Context, req domain.BidRequest) ([]domain.Bid, error) {
	// Проверяем, не отменён ли уже родительский контекст
	if err := parentCtx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(parentCtx, bidTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	results := make(chan domain.Bid, len(s.bidders))

	for _, bidder := range s.bidders {
		bidder := bidder
		g.Go(func() error {
			bid, err := bidder.GetBid(gCtx, req)
			if err != nil {
				slog.WarnContext(gCtx, "bidder failed", "error", err)
				return nil
			}
			if err := bid.Validate(); err != nil {
				slog.WarnContext(gCtx, "bidder returned invalid bid", "error", err)
				return nil
			}
			results <- *bid
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Если родительский контекст отменён во время работы - прерываемся.
	if err := parentCtx.Err(); err != nil {
		return nil, err
	}

	close(results)

	bids := make([]domain.Bid, 0, len(s.bidders))
	for bid := range results {
		bids = append(bids, bid)
	}

	return bids, nil
}
