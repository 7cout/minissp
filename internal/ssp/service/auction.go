package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// bidWithSource — ставка вместе с биддером, который её прислал.
type bidWithSource struct {
	Bid    domain.Bid
	Bidder BidderClient
}

// RunAuction проводит аукцион second-price для указанного слота.
func (s *Service) RunAuction(ctx context.Context, req domain.BidRequest) (*domain.AuctionResult, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bid request: %w", err)
	}

	slot, err := s.slots.Get(ctx, req.SlotID)
	if err != nil {
		return nil, fmt.Errorf("get slot %s: %w", req.SlotID, err)
	}

	// Генерируем ImpID до сбора ставок — он уходит в DSP
	// как часть BidRequest и сохраняется в AuctionRecord.
	req.ImpID = uuid.NewString()

	bids, err := s.collectBids(ctx, req, slot)
	if err != nil {
		return nil, fmt.Errorf("collect bids: %w", err)
	}
	if len(bids) == 0 {
		return nil, domain.ErrNoBids
	}

	winner, price := selectWinner(bids, slot.MinPrice)

	auctionID := uuid.NewString()
	record := domain.AuctionRecord{
		AuctionID:   auctionID,
		ImpID:       req.ImpID, // ← теперь заполняем
		CampaignID:  winner.Bid.CampaignID,
		CreativeID:  winner.Bid.CreativeID,
		PublisherID: slot.PublisherID,
		SlotID:      slot.ID,
		BidderID:    winner.Bidder.Name(),
		Price:       price,
		CreatedAt:   time.Now(),
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("invalid auction record: %w", err)
	}

	s.storeAuction(record)

	return &domain.AuctionResult{
		AuctionID:   auctionID,
		CreativeURL: winner.Bid.CreativeURL,
		ClickURL:    winner.Bid.ClickURL,
	}, nil
}

// collectBids опрашивает всех биддеров параллельно.
func (s *Service) collectBids(
	ctx context.Context,
	req domain.BidRequest,
	slot *domain.Slot,
) ([]bidWithSource, error) {
	type result struct {
		bid    *domain.Bid
		bidder BidderClient
		err    error
	}

	results := make(chan result, len(s.bidders))

	var wg sync.WaitGroup
	for _, b := range s.bidders {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			bid, err := b.GetBid(ctx, req, slot)
			results <- result{bid: bid, bidder: b, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var bids []bidWithSource
	for r := range results {
		if r.err != nil {
			if errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded) {
				return nil, r.err
			}
			if errors.Is(r.err, domain.ErrNoBids) {
				continue
			}
			slog.WarnContext(ctx, "bidder failed",
				"bidder", r.bidder.Name(),
				"error", r.err,
			)
			continue
		}
		if r.bid == nil {
			continue
		}
		bids = append(bids, bidWithSource{Bid: *r.bid, Bidder: r.bidder})
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return bids, nil
}

// selectWinner выбирает победителя по модели second-price.
//
// Победитель — ставка с максимальной ценой.
// Цена = максимальная из: ставки второго по величине, bidFloor.
// Если ставка одна — цена = её ставка, но не ниже bidFloor.
//
// При равенстве ставок побеждает первая в слайсе (first-arrived).
func selectWinner(bids []bidWithSource, bidFloor int64) (bidWithSource, int64) {
	winner := bids[0]
	var second int64

	for _, b := range bids[1:] {
		switch {
		case b.Bid.Price > winner.Bid.Price:
			second = winner.Bid.Price
			winner = b
		case b.Bid.Price > second:
			second = b.Bid.Price
		}
	}

	price := second
	if price == 0 {
		price = winner.Bid.Price
	}
	if price < bidFloor {
		price = bidFloor
	}
	return winner, price
}

// storeAuction сохраняет запись аукциона.
func (s *Service) storeAuction(record domain.AuctionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auctions[record.AuctionID] = record
}

// takeAuction извлекает запись и удаляет её.
func (s *Service) takeAuction(auctionID string) (domain.AuctionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.auctions[auctionID]
	if !ok {
		return domain.AuctionRecord{}, false
	}
	delete(s.auctions, auctionID)
	return record, true
}

// listExpiredAuctions возвращает записи старше AuctionTTL.
func (s *Service) listExpiredAuctions(now time.Time) []domain.AuctionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var expired []domain.AuctionRecord
	for _, r := range s.auctions {
		if now.Sub(r.CreatedAt) > AuctionTTL {
			expired = append(expired, r)
		}
	}
	return expired
}

// markProcessed запоминает auction_id как обработанный.
func (s *Service) markProcessed(auctionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[auctionID] = time.Now()
}

// isProcessed проверяет, обрабатывался ли auction_id.
func (s *Service) isProcessed(auctionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.processed[auctionID]
	return ok
}

// cleanProcessed удаляет записи старше ProcessedTTL.
func (s *Service) cleanProcessed(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, ts := range s.processed {
		if now.Sub(ts) > ProcessedTTL {
			delete(s.processed, id)
		}
	}
}
