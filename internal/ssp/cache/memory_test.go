package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

func testBannerSlot() *domain.Slot {
	return &domain.Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}
}

func TestMemorySlotCache_Get_Miss(t *testing.T) {
	c := NewMemorySlotCache(time.Minute)

	_, err := c.Get(context.Background(), "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss, got %v", err)
	}
}

func TestMemorySlotCache_PutThenGet(t *testing.T) {
	c := NewMemorySlotCache(time.Minute)
	ctx := context.Background()

	slot := testBannerSlot()
	if err := c.Put(ctx, slot); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := c.Get(ctx, "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
	if got.Banner == nil || got.Banner.Width != 320 {
		t.Errorf("banner = %+v", got.Banner)
	}
}

func TestMemorySlotCache_TTLExpires(t *testing.T) {
	c := NewMemorySlotCache(20 * time.Millisecond)
	ctx := context.Background()

	if err := c.Put(ctx, testBannerSlot()); err != nil {
		t.Fatalf("put: %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	_, err := c.Get(ctx, "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after TTL, got %v", err)
	}
}

func TestMemorySlotCache_ReturnsDeepCopy(t *testing.T) {
	c := NewMemorySlotCache(time.Minute)
	ctx := context.Background()

	slot := testBannerSlot()
	_ = c.Put(ctx, slot)

	got, _ := c.Get(ctx, "pub_1", "home_banner")
	got.Banner.Width = 999

	again, _ := c.Get(ctx, "pub_1", "home_banner")
	if again.Banner.Width != 320 {
		t.Errorf("banner width changed via pointer: %d", again.Banner.Width)
	}
}

func TestMemorySlotCache_Invalidate(t *testing.T) {
	c := NewMemorySlotCache(time.Minute)
	ctx := context.Background()

	_ = c.Put(ctx, testBannerSlot())
	_ = c.Invalidate(ctx, "pub_1", "home_banner")

	_, err := c.Get(ctx, "pub_1", "home_banner")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("want ErrCacheMiss after invalidate, got %v", err)
	}
}

func TestMemorySlotCache_Concurrent(t *testing.T) {
	c := NewMemorySlotCache(time.Minute)
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Put(ctx, testBannerSlot()); err != nil {
				t.Errorf("put: %v", err)
			}
		}()
	}
	wg.Wait()

	// В кэше должен остаться один валидный слот.
	got, err := c.Get(ctx, "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("get after concurrent puts: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
}
