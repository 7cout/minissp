package seed

import (
	"context"
	"fmt"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/repository/memory"
)

// PopulateMemory наполняет in-memory репозитории SSP тестовыми данными.
//
// Возвращает ошибку, если репозитории её вернули. В memory-реализации
// это означает, что seed-данные дублируются.
func PopulateMemory(
	ctx context.Context,
	publishers *memory.PublisherRepo,
	slots *memory.SlotRepo,
) error {
	if err := publishers.Add(ctx, &domain.Publisher{
		ID:      PublisherT2,
		Name:    "T2",
		APIKey:  "ssp_dev_secret_key_777",
		Balance: 0,
	}); err != nil {
		return fmt.Errorf("seed publisher: %w", err)
	}

	slots.Add(&domain.Slot{
		ID:          SlotHomeBanner,
		PublisherID: PublisherT2,
		Name:        "home_banner",
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	})

	return nil
}
