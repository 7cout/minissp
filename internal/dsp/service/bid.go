package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/dsp/domain"
	dspmetrics "github.com/7cout/minissp/internal/dsp/metrics"
)

// GetBid подбирает кампанию и креатив под запрос, резервирует бюджет
// и возвращает ставку.
//
// Возвращает domain.ErrNoEligibleCampaign, если ни одна кампания
// не подходит или у всех недостаточно бюджета.
func (s *Service) GetBid(ctx context.Context, req domain.BidRequest) (bid *domain.Bid, err error) {
	start := time.Now()
	defer func() {
		dspmetrics.BidDuration.Observe(time.Since(start).Seconds())
		dspmetrics.BidsTotal.WithLabelValues(bidStatus(err)).Inc()
	}()

	// 1. Кампании по гео с доступным бюджетом.
	campaigns, err := s.campaigns.ListByGeo(ctx, req.Geo)
	if err != nil {
		return nil, fmt.Errorf("list campaigns by geo %s: %w", req.Geo, err)
	}

	// 2. Ставка считается от bid_floor.
	price := s.calculateBid(req.BidFloor)
	if price <= 0 {
		return nil, domain.ErrNoEligibleCampaign
	}

	// 3. Пробуем каждую кампанию: есть ли подходящий креатив и хватит ли бюджета.
	for _, campaign := range campaigns {
		creative, err := s.findMatchingCreative(ctx, campaign.ID, req)
		if err != nil {
			return nil, err
		}
		if creative == nil {
			continue
		}

		// 4. Резервируем бюджет. Если не хватает — пробуем следующую.
		if err := s.campaigns.Reserve(ctx, campaign.ID, price); err != nil {
			if errors.Is(err, domain.ErrInsufficientBudget) {
				continue
			}
			return nil, fmt.Errorf("reserve budget for campaign %s: %w", campaign.ID, err)
		}

		return &domain.Bid{
			ID:          uuid.NewString(),
			ImpID:       req.ImpID,
			CampaignID:  campaign.ID,
			CreativeID:  creative.ID,
			CreativeURL: creative.URL,
			ClickURL:    creative.ClickURL,
			Price:       price,
		}, nil
	}

	return nil, domain.ErrNoEligibleCampaign
}

// calculateBid считает ставку от bid_floor по стратегии DSP.
func (s *Service) calculateBid(bidFloor int64) int64 {
	if bidFloor <= 0 {
		return 0
	}
	return bidFloor * s.bidMultiplierPercent / 100
}

// findMatchingCreative ищет креатив кампании, подходящий по размеру слота.
// Возвращает nil, если не нашёл.
func (s *Service) findMatchingCreative(
	ctx context.Context,
	campaignID string,
	req domain.BidRequest,
) (*domain.Creative, error) {
	creatives, err := s.creatives.ListByCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list creatives for campaign %s: %w", campaignID, err)
	}

	for i := range creatives {
		if fitsSlot(creatives[i], req) {
			return &creatives[i], nil
		}
	}
	return nil, nil
}

// fitsSlot проверяет, что креатив подходит слоту по типу и параметрам.
func fitsSlot(c domain.Creative, req domain.BidRequest) bool {
	if c.Type != req.Type {
		return false
	}

	switch req.Type {
	case domain.CreativeTypeBanner:
		if c.Banner == nil || req.Banner == nil {
			return false
		}
		return c.Banner.Width == req.Banner.Width &&
			c.Banner.Height == req.Banner.Height

	case domain.CreativeTypeVideo:
		if c.Video == nil || req.Video == nil {
			return false
		}
		return c.Video.Width == req.Video.Width &&
			c.Video.Height == req.Video.Height

	// native и audio — заглушки, всегда false
	default:
		return false
	}
}

// bidStatus превращает ошибку GetBid в метку для счётчика.
func bidStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, domain.ErrNoEligibleCampaign):
		return "no_eligible"
	default:
		return "error"
	}
}
