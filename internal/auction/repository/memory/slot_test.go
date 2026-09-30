package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func testSlot(id string) *domain.Slot {
	return &domain.Slot{
		ID:          id,
		PublisherID: "pub_1",
		Name:        "home_banner",
		Banner:      domain.Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	}
}

func TestSlotRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewSlotRepo()
		slot := testSlot("slot_1")
		repo.Add(slot)

		got, err := repo.Get(context.Background(), "slot_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "slot_1" {
			t.Errorf("id = %q, want slot_1", got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewSlotRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})
}
