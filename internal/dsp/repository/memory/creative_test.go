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

func testVideoCreative(id, campaignID string) *domain.Creative {
	return &domain.Creative{
		ID:         id,
		CampaignID: campaignID,
		Type:       domain.CreativeTypeVideo,
		Video: &domain.Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4", "video/webm"},
		},
		URL:      "https://cdn.example.com/video.mp4",
		ClickURL: "https://example.com/click",
	}
}

// addCreative — helper: падает с t.Fatal при ошибке Add.
func addCreative(t *testing.T, repo *CreativeRepo, c *domain.Creative) {
	t.Helper()
	if err := repo.Add(context.Background(), c); err != nil {
		t.Fatalf("add creative: %v", err)
	}
}

func TestCreativeRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testCreative("cr_1", "camp_1"))

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

	t.Run("returns deep copy — video mimes", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testVideoCreative("cr_1", "camp_1"))

		got, _ := repo.Get(context.Background(), "cr_1")
		got.Video.MIMEs[0] = "changed"

		again, _ := repo.Get(context.Background(), "cr_1")
		if again.Video.MIMEs[0] != "video/mp4" {
			t.Errorf("mimes changed via slice: %v", again.Video.MIMEs)
		}
	})

	t.Run("returns deep copy — banner", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testCreative("cr_1", "camp_1"))

		got, _ := repo.Get(context.Background(), "cr_1")
		got.Banner.Width = 999

		again, _ := repo.Get(context.Background(), "cr_1")
		if again.Banner.Width != 320 {
			t.Errorf("banner width changed via pointer: %d", again.Banner.Width)
		}
	})
}

func TestCreativeRepo_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewCreativeRepo()
		err := repo.Add(context.Background(), testCreative("cr_1", "camp_1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		repo := NewCreativeRepo()
		err := repo.Add(context.Background(), testCreative("", "camp_1"))
		if err == nil {
			t.Error("want error for empty id, got nil")
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testCreative("cr_1", "camp_1"))

		err := repo.Add(context.Background(), testCreative("cr_1", "camp_2"))
		if err == nil {
			t.Error("want error for duplicate id, got nil")
		}
	})
}

func TestCreativeRepo_ListByCampaign(t *testing.T) {
	t.Run("returns only matching creatives", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testCreative("cr_1", "camp_1"))
		addCreative(t, repo, testCreative("cr_2", "camp_1"))
		addCreative(t, repo, testCreative("cr_3", "camp_2"))

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
		addCreative(t, repo, testCreative("cr_1", "camp_1"))

		creatives, err := repo.ListByCampaign(context.Background(), "unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(creatives) != 0 {
			t.Errorf("want 0 creatives, got %d", len(creatives))
		}
	})

	t.Run("returns deep copy — banner", func(t *testing.T) {
		repo := NewCreativeRepo()
		addCreative(t, repo, testCreative("cr_1", "camp_1"))

		got, _ := repo.ListByCampaign(context.Background(), "camp_1")
		got[0].Banner.Width = 999

		again, _ := repo.ListByCampaign(context.Background(), "camp_1")
		if again[0].Banner.Width != 320 {
			t.Errorf("banner width changed via pointer: %d", again[0].Banner.Width)
		}
	})
}
