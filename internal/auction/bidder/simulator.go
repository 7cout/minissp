package bidder

import (
	"context"
	"math/rand"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/auction/domain"
)

// Simulator — фейковый биддер. Возвращает случайную ставку
// в заданном диапазоне. Используется для локального запуска
// и в тестах - чтобы не подключать реальные DSP
type Simulator struct {
	campaignID string
	minPrice   int64 // минимальная ставка
	maxPrice   int64 // максимальная ставка
}

// NewSimulator создаёт симулятор биддера
//
// campaignID — ID кампании, которую представляет биддер
// minPrice, maxPrice — диапазон случайных ставок в микроединицах
func NewSimulator(campaignID string, minPrice, maxPrice int64) *Simulator {
	return &Simulator{
		campaignID: campaignID,
		minPrice:   minPrice,
		maxPrice:   maxPrice,
	}
}

// GetBid возвращает случайную ставку
//
// Ставка обрезается bid floor из запроса: если сгенерированная цена
// меньше floor - биддер возвращает ошибку (отказывается от участия)
func (s *Simulator) GetBid(ctx context.Context, req domain.BidRequest) (*domain.Bid, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Случайная цена в диапазоне [minPrice, maxPrice]
	price := s.minPrice + rand.Int63n(s.maxPrice-s.minPrice+1)

	// Если цена ниже bid floor - биддер не участвует
	if price < req.Imp.BidFloor {
		return nil, domain.ErrNoBids
	}

	return &domain.Bid{
		ID:         uuid.NewString(),
		ImpID:      req.Imp.ID,
		CampaignID: s.campaignID,
		CreativeID: "creative_" + s.campaignID,
		Price:      price,
	}, nil
}
