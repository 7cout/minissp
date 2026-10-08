package seed

import (
	"context"
	"fmt"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// PublisherAdder — узкий интерфейс для добавления publisher'а.
//
// Реализуется и memory.PublisherRepo, и postgres.PublisherRepo —
// не надо дублировать seed-логику под каждое хранилище.
type PublisherAdder interface {
	Add(ctx context.Context, p *domain.Publisher) error
}

// SlotAdder — узкий интерфейс для добавления слота.
type SlotAdder interface {
	Add(ctx context.Context, s *domain.Slot) error
}

// Populate наполняет хранилище тестовыми данными.
//
// Идемпотентна на уровне хранилища: повторный вызов упадёт на
// unique violation. Для memory запускается при старте cmd/ssp,
// для postgres — отдельной командой cmd/seed.
func Populate(
	ctx context.Context,
	publishers PublisherAdder,
	slots SlotAdder,
) error {
	if err := publishers.Add(ctx, &domain.Publisher{
		ID:      PublisherT2,
		Name:    "T2",
		APIKey:  "ssp_dev_secret_key_777",
		Balance: 0,
	}); err != nil {
		return fmt.Errorf("seed publisher: %w", err)
	}

	if err := slots.Add(ctx, &domain.Slot{
		ID:          SlotHomeBanner,
		PublisherID: PublisherT2,
		Name:        "home_banner",
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}); err != nil {
		return fmt.Errorf("seed slot: %w", err)
	}

	return nil
}
