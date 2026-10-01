//go:build integration

package postgres

import (
	"context"
	"testing"
)

func TestCreativeRepo_ListByCampaign(t *testing.T) {
	cleanTables(t)
	repo := NewCreativeRepo(testPool)
	ctx := context.Background()

	const (
		campaignID = "33333333-3333-3333-3333-333333333333"
		creative1  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		creative2  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)

	// Вставляем кампанию (нужна для FK).
	_, err := testPool.Exec(ctx, `
		INSERT INTO campaigns (id, advertiser_id, name, budget_total, budget_remaining, budget_reserved, geo_target)
		VALUES ($1, '44444444-4444-4444-4444-444444444444', 'Test', 1000000, 1000000, 0, 'RU')
	`, campaignID)
	if err != nil {
		t.Fatalf("insert campaign: %v", err)
	}

	// Вставляем два креатива.
	_, err = testPool.Exec(ctx, `
		INSERT INTO creatives (id, campaign_id, width, height, url, click_url)
		VALUES ($1, $2, 320, 50, 'https://x.com/a.jpg', 'https://x.com/click-a'),
		       ($3, $2, 320, 50, 'https://x.com/b.jpg', 'https://x.com/click-b')
	`, creative1, campaignID, creative2)
	if err != nil {
		t.Fatalf("insert creatives: %v", err)
	}

	t.Run("returns all creatives", func(t *testing.T) {
		creatives, err := repo.ListByCampaign(ctx, campaignID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(creatives) != 2 {
			t.Errorf("want 2, got %d", len(creatives))
		}
	})

	t.Run("empty for unknown campaign", func(t *testing.T) {
		creatives, err := repo.ListByCampaign(ctx, "99999999-9999-9999-9999-999999999999")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(creatives) != 0 {
			t.Errorf("want 0, got %d", len(creatives))
		}
	})
}
