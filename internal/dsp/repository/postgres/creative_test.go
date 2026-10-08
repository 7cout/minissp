//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func testBannerCreative(campaignID string) *domain.Creative {
	return &domain.Creative{
		ID:         uuid.NewString(),
		CampaignID: campaignID,
		Type:       domain.CreativeTypeBanner,
		URL:        "https://cdn.example.com/banner.jpg",
		ClickURL:   "https://example.com/click",
		Banner:     &domain.Banner{Width: 320, Height: 50},
	}
}

func testVideoCreative(campaignID string) *domain.Creative {
	return &domain.Creative{
		ID:         uuid.NewString(),
		CampaignID: campaignID,
		Type:       domain.CreativeTypeVideo,
		URL:        "https://cdn.example.com/video.mp4",
		ClickURL:   "https://example.com/click",
		Video: &domain.Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4", "video/webm"},
		},
	}
}

func TestPostgresCreativeRepo_AddAndGet_Banner(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	creatives := NewCreativeRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
	camp := testCampaign(adv.ID, "RU", 1_000_000)
	_ = campaigns.Add(ctx, camp)

	cr := testBannerCreative(camp.ID)
	if err := creatives.Add(ctx, cr); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := creatives.Get(ctx, cr.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != domain.CreativeTypeBanner {
		t.Errorf("type = %q, want banner", got.Type)
	}
	if got.Banner == nil || got.Banner.Width != 320 || got.Banner.Height != 50 {
		t.Errorf("banner = %+v, want 320x50", got.Banner)
	}
	if got.Video != nil {
		t.Errorf("video should be nil for banner, got %+v", got.Video)
	}
}

func TestPostgresCreativeRepo_AddAndGet_Video(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	creatives := NewCreativeRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)
	camp := testCampaign(adv.ID, "RU", 1_000_000)
	_ = campaigns.Add(ctx, camp)

	cr := testVideoCreative(camp.ID)
	if err := creatives.Add(ctx, cr); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := creatives.Get(ctx, cr.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != domain.CreativeTypeVideo {
		t.Errorf("type = %q, want video", got.Type)
	}
	if got.Video == nil {
		t.Fatal("video is nil")
	}
	if got.Video.Width != 640 || got.Video.Height != 480 {
		t.Errorf("video size = %dx%d, want 640x480", got.Video.Width, got.Video.Height)
	}
	if got.Video.Duration != 15 {
		t.Errorf("duration = %d, want 15", got.Video.Duration)
	}
	if len(got.Video.MIMEs) != 2 {
		t.Errorf("mimes = %v, want 2 elements", got.Video.MIMEs)
	}
	if got.Banner != nil {
		t.Errorf("banner should be nil for video, got %+v", got.Banner)
	}
}

func TestPostgresCreativeRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	creatives := NewCreativeRepo(pool)

	_, err := creatives.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrCreativeNotFound) {
		t.Errorf("want ErrCreativeNotFound, got %v", err)
	}
}

func TestPostgresCreativeRepo_ListByCampaign(t *testing.T) {
	pool := newTestPool(t)
	advertisers := NewAdvertiserRepo(pool)
	campaigns := NewCampaignRepo(pool)
	creatives := NewCreativeRepo(pool)
	ctx := context.Background()

	adv := createTestAdvertiser(t, advertisers, 1_000_000_000)

	camp1 := testCampaign(adv.ID, "RU", 1_000_000)
	camp2 := testCampaign(adv.ID, "RU", 1_000_000)
	_ = campaigns.Add(ctx, camp1)
	_ = campaigns.Add(ctx, camp2)

	_ = creatives.Add(ctx, testBannerCreative(camp1.ID))
	_ = creatives.Add(ctx, testBannerCreative(camp1.ID))
	_ = creatives.Add(ctx, testBannerCreative(camp2.ID))

	t.Run("returns only matching creatives", func(t *testing.T) {
		got, err := creatives.ListByCampaign(ctx, camp1.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("want 2 creatives, got %d", len(got))
		}
	})

	t.Run("empty for unknown campaign", func(t *testing.T) {
		got, err := creatives.ListByCampaign(ctx, uuid.NewString())
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want 0 creatives, got %d", len(got))
		}
	})
}
