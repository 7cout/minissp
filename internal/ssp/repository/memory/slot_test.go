package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func testBannerSlot(id, publisherID, name string) *domain.Slot {
	return &domain.Slot{
		ID:          id,
		PublisherID: publisherID,
		Name:        name,
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}
}

func testVideoSlot(id, publisherID, name string) *domain.Slot {
	return &domain.Slot{
		ID:          id,
		PublisherID: publisherID,
		Name:        name,
		Geo:         "RU",
		MinPrice:    5_000_000,
		Type:        domain.CreativeTypeVideo,
		Video: &domain.Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4"},
		},
	}
}

func TestSlotRepo_Get(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

		got, err := repo.Get(context.Background(), "slot_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "slot_1" {
			t.Errorf("id = %q, want slot_1", got.ID)
		}
		if got.Type != domain.CreativeTypeBanner {
			t.Errorf("type = %q, want banner", got.Type)
		}
		if got.Banner == nil || got.Banner.Width != 320 {
			t.Errorf("banner not copied correctly: %+v", got.Banner)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewSlotRepo()

		_, err := repo.Get(context.Background(), "missing")
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})

	t.Run("returns deep copy — banner", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

		got, _ := repo.Get(context.Background(), "slot_1")
		got.Banner.Width = 999

		again, _ := repo.Get(context.Background(), "slot_1")
		if again.Banner.Width != 320 {
			t.Errorf("banner width changed via pointer: %d", again.Banner.Width)
		}
	})

	t.Run("returns deep copy — video mimes", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testVideoSlot("slot_1", "pub_1", "preroll"))

		got, _ := repo.Get(context.Background(), "slot_1")
		got.Video.MIMEs[0] = "changed"

		again, _ := repo.Get(context.Background(), "slot_1")
		if again.Video.MIMEs[0] != "video/mp4" {
			t.Errorf("mimes changed via slice: %v", again.Video.MIMEs)
		}
	})
}

func TestSlotRepo_GetByName(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))
		repo.Add(testBannerSlot("slot_2", "pub_2", "home_banner"))

		got, err := repo.GetByName(context.Background(), "pub_1", "home_banner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "slot_1" {
			t.Errorf("id = %q, want slot_1", got.ID)
		}
	})

	t.Run("not found — wrong publisher", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

		_, err := repo.GetByName(context.Background(), "pub_2", "home_banner")
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})

	t.Run("not found — wrong name", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.Add(testBannerSlot("slot_1", "pub_1", "home_banner"))

		_, err := repo.GetByName(context.Background(), "pub_1", "other")
		if !errors.Is(err, domain.ErrSlotNotFound) {
			t.Errorf("want ErrSlotNotFound, got %v", err)
		}
	})
}

func TestSlotRepo_AddIfAbsent(t *testing.T) {
	t.Run("first insert returns inserted=true", func(t *testing.T) {
		repo := NewSlotRepo()
		slot := testBannerSlot("slot_1", "pub_1", "home_banner")

		got, inserted := repo.AddIfAbsent(slot)
		if !inserted {
			t.Error("want inserted=true")
		}
		if got.ID != "slot_1" {
			t.Errorf("id = %q, want slot_1", got.ID)
		}
	})

	t.Run("second insert with same name returns existing", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.AddIfAbsent(testBannerSlot("slot_1", "pub_1", "home_banner"))

		_, inserted := repo.AddIfAbsent(testBannerSlot("slot_2", "pub_1", "home_banner"))
		if inserted {
			t.Error("want inserted=false")
		}

		// Убедимся, что в репозитории только один слот.
		count := 0
		repo.mu.RLock()
		for range repo.slots {
			count++
		}
		repo.mu.RUnlock()
		if count != 1 {
			t.Errorf("slots count = %d, want 1", count)
		}
	})

	t.Run("different publisher — same name is fine", func(t *testing.T) {
		repo := NewSlotRepo()
		repo.AddIfAbsent(testBannerSlot("slot_1", "pub_1", "home_banner"))

		_, inserted := repo.AddIfAbsent(testBannerSlot("slot_2", "pub_2", "home_banner"))
		if !inserted {
			t.Error("want inserted=true for different publisher")
		}
	})

	t.Run("concurrent — only one wins", func(t *testing.T) {
		repo := NewSlotRepo()

		const goroutines = 100
		var wg sync.WaitGroup
		ids := make([]string, goroutines)

		start := make(chan struct{})
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				slot := testBannerSlot(uuid.NewString(), "pub_1", "home_banner")
				got, _ := repo.AddIfAbsent(slot)
				ids[idx] = got.ID
			}(i)
		}
		close(start)
		wg.Wait()

		// Все должны получить один и тот же ID.
		want := ids[0]
		for i, id := range ids {
			if id != want {
				t.Errorf("goroutine %d got %q, want %q", i, id, want)
			}
		}

		// В репозитории ровно один слот.
		repo.mu.RLock()
		count := len(repo.slots)
		repo.mu.RUnlock()
		if count != 1 {
			t.Errorf("slots count = %d, want 1", count)
		}
	})
}
