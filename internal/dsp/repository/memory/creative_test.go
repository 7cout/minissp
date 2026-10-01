package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func testCreative(id, campaignID string) *domain.Creative {
	return &domain.Creative{
		ID:         id,
		CampaignID: campaignID,
		Type:       domain.CreativeTypeBanner,
		Banner:     &domain.Banner{Width: 320, Height: 50},
		URL:        "https://cdn.example.com/banner.jpg",
		ClickURL:   "https://nike.com/summer",
	}
}

func TestCreativeRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewCreativeRepo()
		repo.Add(testCreative("cr_1", "camp_1"))

		got, err := repo.Get(context.Background(), "cr_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "cr_1" {
			t.Errorf("id = %q, want cr_1", got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewCreativeRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrCreativeNotFound) {
			t.Errorf("want ErrCreativeNotFound, got %v", err)
		}
	})
}

func TestCreativeRepo_ListByCampaign(t *testing.T) {
	t.Run("returns only matching creatives", func(t *testing.T) {
		repo := NewCreativeRepo()
		repo.Add(testCreative("cr_1", "camp_1"))
		repo.Add(testCreative("cr_2", "camp_1"))
		repo.Add(testCreative("cr_3", "camp_2"))

		creatives, err := repo.ListByCampaign(context.Background(), "camp_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(creatives) != 2 {
			t.Errorf("want 2 creatives, got %d", len(creatives))
		}
	})

	t.Run("empty for unknown campaign", func(t *testing.T) {
		repo := NewCreativeRepo()
		repo.Add(testCreative("cr_1", "camp_1"))

		creatives, err := repo.ListByCampaign(context.Background(), "unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(creatives) != 0 {
			t.Errorf("want 0 creatives, got %d", len(creatives))
		}
	})
}
