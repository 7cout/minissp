package seed

import (
	"github.com/7cout/minissp/internal/auction/domain"
	"github.com/7cout/minissp/internal/auction/repository/memory"
)

// PopulateMemory наполняет in-memory репозитории SSP тестовыми данными.
func PopulateMemory(slots *memory.SlotRepo) {
	slots.Add(&domain.Slot{
		ID:          SlotHomeBanner,
		PublisherID: PublisherT2,
		Name:        "home_banner",
		Banner:      domain.Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	})
}
