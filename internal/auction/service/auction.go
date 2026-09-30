package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/auction/domain"
)

// RunAuction проводит аукцион для указанного слота
//
// Шаги:
//  1. Валидирует запрос
//  2. Достаёт слот из репозитория
//  3. Параллельно опрашивает биддеров (collectBids)
//  4. Выбирает победителя по second-price (selectWinner)
//  5. Резервирует бюджет победителя
//
// Возвращает domain.ErrNoBids, если ни один биддер не ответил
// Возвращает domain.ErrSlotNotFound, если слот не найден
// Возвращает domain.ErrInsufficientBudget, если у победителя не хватило бюджета
func (s *Service) RunAuction(ctx context.Context, req domain.BidRequest) (*domain.AuctionResult, error) {
	// Валидация входного запроса
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bid request: %w", err)
	}

	// Достаём слот
	slot, err := s.slots.Get(ctx, req.Imp.SlotID)
	if err != nil {
		return nil, fmt.Errorf("get slot %s: %w", req.Imp.SlotID, err)
	}

	// Собираем ставки параллельно
	bids, err := s.collectBids(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("collect bids: %w", err)
	}

	// Выбираем победителя
	winner, price, err := selectWinner(bids, slot.MinPrice)
	if err != nil {
		return nil, fmt.Errorf("select winner: %w", err)
	}

	// Резервируем бюджет победителя
	if err := s.campaigns.Reserve(ctx, winner.CampaignID, price); err != nil {
		return nil, fmt.Errorf("reserve budget for campaign %s: %w", winner.CampaignID, err)
	}

	// Формируем результат
	return &domain.AuctionResult{
		AuctionID:  uuid.NewString(),
		ImpID:      req.Imp.ID,
		BidID:      winner.ID,
		CampaignID: winner.CampaignID,
		CreativeID: winner.CreativeID,
		Price:      price,
	}, nil
}
