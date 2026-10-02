package seed

import (
	"github.com/7cout/minissp/internal/dsp/domain"
	"github.com/7cout/minissp/internal/dsp/repository/memory"
)

// PopulateMemory наполняет in-memory репозитории DSP тестовыми данными.
//
// Три рекламодателя — Nike, Adidas, Puma.
// У каждого — одна кампания с бюджетом и один баннер 320x50.
func PopulateMemory(
	advertisers *memory.AdvertiserRepo,
	campaigns *memory.CampaignRepo,
	creatives *memory.CreativeRepo,
) {
	advs := []domain.Advertiser{
		{ID: AdvertiserNike, Name: "Nike", Balance: 10_000_000_000},
		{ID: AdvertiserAdidas, Name: "Adidas", Balance: 10_000_000_000},
		{ID: AdvertiserPuma, Name: "Puma", Balance: 10_000_000_000},
	}
	for i := range advs {
		advertisers.Add(&advs[i])
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
		campaigns.Add(&domain.Campaign{
			ID:              c.id,
			AdvertiserID:    c.advertiserID,
			Name:            c.name,
			BudgetTotal:     5_000_000_000,
			BudgetRemaining: 5_000_000_000,
			BudgetReserved:  0,
			GeoTarget:       "RU",
		})

		creatives.Add(&domain.Creative{
			ID:         c.creativeID,
			CampaignID: c.id,
			Type:       domain.CreativeTypeBanner,
			Banner:     &domain.Banner{Width: 320, Height: 50},
			URL:        "https://cdn.example.com/" + c.name + ".jpg",
			ClickURL:   "https://example.com/click_" + c.name,
		})
	}
}
