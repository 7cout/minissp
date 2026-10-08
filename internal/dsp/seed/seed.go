package seed

import (
	"context"
	"fmt"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// AdvertiserAdder — узкий интерфейс для добавления рекламодателя.
type AdvertiserAdder interface {
	Add(ctx context.Context, a *domain.Advertiser) error
}

// CampaignAdder — узкий интерфейс для добавления кампании.
type CampaignAdder interface {
	Add(ctx context.Context, c *domain.Campaign) error
}

// CreativeAdder — узкий интерфейс для добавления креатива.
type CreativeAdder interface {
	Add(ctx context.Context, c *domain.Creative) error
}

// Populate наполняет хранилище тестовыми данными.
//
// Три рекламодателя — Nike, Adidas, Puma.
// У каждого — одна кампания с бюджетом и один баннер 320x50.
//
// Одна функция для memory и postgres — не надо дублировать
// логику раскладки domain-структур под разные хранилища.
func Populate(
	ctx context.Context,
	advertisers AdvertiserAdder,
	campaigns CampaignAdder,
	creatives CreativeAdder,
) error {
	advs := []domain.Advertiser{
		{ID: AdvertiserNike, Name: "Nike", Balance: 10_000_000_000},
		{ID: AdvertiserAdidas, Name: "Adidas", Balance: 10_000_000_000},
		{ID: AdvertiserPuma, Name: "Puma", Balance: 10_000_000_000},
	}
	for i := range advs {
		if err := advertisers.Add(ctx, &advs[i]); err != nil {
			return fmt.Errorf("seed advertiser %s: %w", advs[i].ID, err)
		}
	}

	type campData struct {
		id, advertiserID, name, creativeID string
	}
	camps := []campData{
		{CampaignNike, AdvertiserNike, "Nike Summer", CreativeNike},
		{CampaignAdidas, AdvertiserAdidas, "Adidas Run", CreativeAdidas},
		{CampaignPuma, AdvertiserPuma, "Puma Winter", CreativePuma},
	}

	for _, c := range camps {
		if err := campaigns.Add(ctx, &domain.Campaign{
			ID:              c.id,
			AdvertiserID:    c.advertiserID,
			Name:            c.name,
			BudgetTotal:     5_000_000_000,
			BudgetRemaining: 5_000_000_000,
			BudgetReserved:  0,
			GeoTarget:       "RU",
		}); err != nil {
			return fmt.Errorf("seed campaign %s: %w", c.id, err)
		}

		if err := creatives.Add(ctx, &domain.Creative{
			ID:         c.creativeID,
			CampaignID: c.id,
			Type:       domain.CreativeTypeBanner,
			Banner:     &domain.Banner{Width: 320, Height: 50},
			URL:        "https://cdn.example.com/" + c.name + ".jpg",
			ClickURL:   "https://example.com/click_" + c.name,
		}); err != nil {
			return fmt.Errorf("seed creative %s: %w", c.creativeID, err)
		}
	}

	return nil
}
