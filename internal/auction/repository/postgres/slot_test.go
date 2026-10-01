//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/auction/domain"
)

func TestSlotRepo_Get(t *testing.T) {
	cleanTables(t)
	repo := NewSlotRepo(testPool)

	const (
		slotID      = "11111111-1111-1111-1111-111111111111"
		publisherID = "22222222-2222-2222-2222-222222222222"
	)

	// Вставляем данные.
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO ad_slots (id, publisher_id, name, width, height, geo, min_price)
		VALUES ($1, $2, 'home_banner', 320, 50, 'RU', 1000000)
	`, slotID, publisherID)
	if err != nil {
		t.Fatalf("insert slot: %v", err)
	}

	t.Run("found", func(t *testing.T) {
		s, err := repo.Get(context.Background(), slotID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.ID != slotID {
			t.Errorf("id = %s, want %s", s.ID, slotID)
		}
		if s.Banner.Width != 320 || s.Banner.Height != 50 {
			t.Errorf("banner = %dx%d, want 320x50", s.Banner.Width, s.Banner.Height)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.Get(context.Background(), "99999999-9999-9999-9999-999999999999")
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})

	t.Run("invalid id format", func(t *testing.T) {
		_, err := repo.Get(context.Background(), "not-a-uuid")
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Errorf("want ErrInvalidID, got %v", err)
		}
	})
}
