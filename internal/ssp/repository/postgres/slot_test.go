//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// createTestPublisher создаёт publisher для FK-связи.
func createTestPublisher(t *testing.T, pool *pgxpool.Pool) *domain.Publisher {
	t.Helper()

	pubRepo := NewPublisherRepo(pool)
	p := &domain.Publisher{
		ID:      uuid.NewString(),
		Name:    "T2",
		APIKey:  "test_key_" + uuid.NewString()[:8],
		Balance: 0,
	}
	if err := pubRepo.Add(context.Background(), p); err != nil {
		t.Fatalf("create test publisher: %v", err)
	}
	return p
}

func testBannerSlot(publisherID, name string) *domain.Slot {
	return &domain.Slot{
		ID:          uuid.NewString(),
		PublisherID: publisherID,
		Name:        name,
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}
}

func testVideoSlot(publisherID, name string) *domain.Slot {
	return &domain.Slot{
		ID:          uuid.NewString(),
		PublisherID: publisherID,
		Name:        name,
		Geo:         "RU",
		MinPrice:    5_000_000,
		Type:        domain.CreativeTypeVideo,
		Video: &domain.Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4", "video/webm"},
		},
	}
}

func TestPostgresSlotRepo_AddAndGet_Banner(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	slot := testBannerSlot(pub.ID, "home_banner")
	if err := repo.Add(ctx, slot); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := repo.Get(ctx, slot.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.ID != slot.ID {
		t.Errorf("id = %q, want %q", got.ID, slot.ID)
	}
	if got.PublisherID != pub.ID {
		t.Errorf("publisher_id = %q, want %q", got.PublisherID, pub.ID)
	}
	if got.Type != domain.CreativeTypeBanner {
		t.Errorf("type = %q, want banner", got.Type)
	}
	if got.Banner == nil {
		t.Fatal("banner is nil")
	}
	if got.Banner.Width != 320 || got.Banner.Height != 50 {
		t.Errorf("banner = %+v, want 320x50", got.Banner)
	}
	if got.Video != nil {
		t.Errorf("video should be nil for banner slot, got %+v", got.Video)
	}
}

func TestPostgresSlotRepo_AddAndGet_Video(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	slot := testVideoSlot(pub.ID, "preroll")
	if err := repo.Add(ctx, slot); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := repo.Get(ctx, slot.ID)
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
		t.Errorf("banner should be nil for video slot, got %+v", got.Banner)
	}
}

func TestPostgresSlotRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)

	_, err := repo.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrSlotNotFound) {
		t.Errorf("want ErrSlotNotFound, got %v", err)
	}
}

func TestPostgresSlotRepo_GetByName(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	slot := testBannerSlot(pub.ID, "home_banner")
	_ = repo.Add(ctx, slot)

	got, err := repo.GetByName(ctx, pub.ID, "home_banner")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != slot.ID {
		t.Errorf("id = %q, want %q", got.ID, slot.ID)
	}
}

func TestPostgresSlotRepo_GetByName_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)

	_, err := repo.GetByName(context.Background(), pub.ID, "missing")
	if !errors.Is(err, domain.ErrSlotNotFound) {
		t.Errorf("want ErrSlotNotFound, got %v", err)
	}
}

func TestPostgresSlotRepo_AddIfAbsent_InsertsFirst(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	slot := testBannerSlot(pub.ID, "home_banner")
	got, inserted, err := repo.AddIfAbsent(ctx, slot)
	if err != nil {
		t.Fatalf("add if absent: %v", err)
	}
	if !inserted {
		t.Error("want inserted=true")
	}
	if got.ID != slot.ID {
		t.Errorf("id = %q, want %q", got.ID, slot.ID)
	}
}

func TestPostgresSlotRepo_AddIfAbsent_ReturnsExisting(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	first := testBannerSlot(pub.ID, "home_banner")
	_, _, _ = repo.AddIfAbsent(ctx, first)

	// Второй слот с тем же (publisher_id, name), но другим ID.
	second := testBannerSlot(pub.ID, "home_banner")
	got, inserted, err := repo.AddIfAbsent(ctx, second)
	if err != nil {
		t.Fatalf("add if absent: %v", err)
	}
	if inserted {
		t.Error("want inserted=false")
	}
	if got.ID != first.ID {
		t.Errorf("returned id = %q, want existing %q", got.ID, first.ID)
	}

	// В БД должен быть ровно один слот.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM ssp.ad_slots WHERE publisher_id = $1 AND name = $2`,
		pub.ID, "home_banner",
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("slots count = %d, want 1", count)
	}
}

func TestPostgresSlotRepo_AddIfAbsent_DifferentPublisher(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub1 := createTestPublisher(t, pool)
	pub2 := createTestPublisher(t, pool)
	ctx := context.Background()

	_, _, _ = repo.AddIfAbsent(ctx, testBannerSlot(pub1.ID, "home_banner"))

	_, inserted, err := repo.AddIfAbsent(ctx, testBannerSlot(pub2.ID, "home_banner"))
	if err != nil {
		t.Fatalf("add if absent: %v", err)
	}
	if !inserted {
		t.Error("want inserted=true for different publisher")
	}
}

func TestPostgresSlotRepo_AddIfAbsent_Concurrent(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	const goroutines = 20
	results := make([]*domain.Slot, goroutines)
	errs := make([]error, goroutines)

	start := make(chan struct{})
	done := make(chan struct{}, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			<-start
			slot := testBannerSlot(pub.ID, "home_banner")
			got, _, err := repo.AddIfAbsent(ctx, slot)
			results[idx] = got
			errs[idx] = err
			done <- struct{}{}
		}(i)
	}
	close(start)
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Все вызовы должны вернуть один и тот же слот.
	first := results[0]
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if r.ID != first.ID {
			t.Errorf("goroutine %d returned id %q, want %q", i, r.ID, first.ID)
		}
	}

	// В БД — ровно один слот.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM ssp.ad_slots WHERE publisher_id = $1 AND name = $2`,
		pub.ID, "home_banner",
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("slots count = %d, want 1", count)
	}
}

func TestPostgresSlotRepo_CheckConstraints(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSlotRepo(pool)
	pub := createTestPublisher(t, pool)
	ctx := context.Background()

	t.Run("banner without width fails", func(t *testing.T) {
		slot := testBannerSlot(pub.ID, "broken_banner")
		slot.Banner.Width = 0

		err := repo.Add(ctx, slot)
		if err == nil {
			t.Error("want CHECK violation for banner_width=0, got nil")
		}
	})

	t.Run("negative min_price fails", func(t *testing.T) {
		slot := testBannerSlot(pub.ID, "neg_price")
		slot.MinPrice = -1

		err := repo.Add(ctx, slot)
		if err == nil {
			t.Error("want CHECK violation for min_price<0, got nil")
		}
	})
}
