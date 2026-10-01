package seed

import (
	"github.com/7cout/minissp/internal/auction/domain"
	"github.com/7cout/minissp/internal/auction/repository/memory"
)

// PopulateMemory наполняет in-memory репозитории тестовыми данными.
//
// Используется для запуска без PostgreSQL и в тестах.
// Данные совпадают с seed-миграцией PostgreSQL.
func PopulateMemory(
	slots *memory.SlotRepo,
	campaigns *memory.CampaignRepo,
	creatives *memory.CreativeRepo,
) {
	slots.Add(&domain.Slot{
		ID:          SlotHomeBanner,
		PublisherID: PublisherT2,
		Name:        "home_banner",
		Banner:      domain.Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	})

	type camp struct {
		id, advertiser, creative, name string
	}
	camps := []camp{
		{CampaignNike, AdvertiserNike, CreativeNike, "Nike Summer"},
		{CampaignAdidas, AdvertiserAdidas, CreativeAdidas, "Adidas Run"},
		{CampaignPuma, AdvertiserPuma, CreativePuma, "Puma Winter"},
	}

	for _, c := range camps {
		campaigns.Add(&domain.Campaign{
			ID:              c.id,
			AdvertiserID:    c.advertiser,
			Name:            c.name,
			BudgetTotal:     100_000_000_000,
			BudgetRemaining: 100_000_000_000,
			BudgetReserved:  0,
			GeoTarget:       "RU",
		})

		creatives.Add(&domain.Creative{
			ID:         c.creative,
			CampaignID: c.id,
			Banner:     domain.Banner{Width: 320, Height: 50},
			URL:        "https://cdn.example.com/" + c.name + ".jpg",
			ClickURL:   "https://example.com/click_" + c.name,
		})
	}
}
