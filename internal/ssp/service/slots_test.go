package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/reserve"
)

func TestService_GetSlotByName(t *testing.T) {
	svc, slots, _ := newTestService()
	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	got, err := svc.GetSlotByName(context.Background(), "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
}

func TestService_GetSlotByName_NotFound(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.GetSlotByName(context.Background(), "pub_1", "missing")
	if !errors.Is(err, domain.ErrSlotNotFound) {
		t.Errorf("want ErrSlotNotFound, got %v", err)
	}
}

// TestService_GetSlotByName_CacheAsides проверяет, что после первого
// GetSlotByName слот попал в кэш и второй вызов уже не идёт в репозиторий.
func TestService_GetSlotByName_CacheAsides(t *testing.T) {
	slots := newFakeSlotRepo()
	pubs := newFakePublisherRepo()
	cacheFake := newFakeSlotCache()

	svc := New(Options{
		Slots:      slots,
		Publishers: pubs,
		Bidders:    nil,
		SlotCache:  cacheFake,
		Reserve:    reserve.NewMemory(reserve.DefaultMemoryOptions()),
	})
	ctx := context.Background()

	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	// Первый Get — промах, идём в репозиторий, кладём в кэш.
	if _, err := svc.GetSlotByName(ctx, "pub_1", "home_banner"); err != nil {
		t.Fatalf("first get: %v", err)
	}

	// Удаляем слот из репозитория: если сервис работает через кэш,
	// второй Get всё равно вернёт слот.
	slots.mu.Lock()
	delete(slots.slots, "slot_1")
	slots.mu.Unlock()

	got, err := svc.GetSlotByName(ctx, "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
}

// TestService_GetSlotByName_CacheDown — если кэш упал, сервис
// продолжает работать через репозиторий.
func TestService_GetSlotByName_CacheDown(t *testing.T) {
	slots := newFakeSlotRepo()
	pubs := newFakePublisherRepo()
	cacheFake := newFakeSlotCache()
	cacheFake.getErr = errors.New("redis down")
	cacheFake.putErr = errors.New("redis down")

	svc := New(Options{
		Slots:      slots,
		Publishers: pubs,
		Bidders:    nil,
		SlotCache:  cacheFake,
		Reserve:    reserve.NewMemory(reserve.DefaultMemoryOptions()),
	})

	addSlot(t, slots, testBannerSlot("slot_1", "pub_1", "home_banner"))

	got, err := svc.GetSlotByName(context.Background(), "pub_1", "home_banner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "slot_1" {
		t.Errorf("id = %q, want slot_1", got.ID)
	}
}

func TestService_RegisterSlot_HappyPath(t *testing.T) {
	svc, _, _ := newTestService()

	slot := domain.Slot{
		Name:     "home_banner",
		Geo:      "RU",
		MinPrice: 1_000_000,
		Type:     domain.CreativeTypeBanner,
		Banner:   &domain.Banner{Width: 320, Height: 50},
	}

	created, err := svc.RegisterSlot(context.Background(), "pub_1", slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == "" {
		t.Error("id not generated")
	}
	if created.PublisherID != "pub_1" {
		t.Errorf("publisher id = %q, want pub_1", created.PublisherID)
	}
	if created.Name != "home_banner" {
		t.Errorf("name = %q", created.Name)
	}
}

func TestService_RegisterSlot_Idempotent(t *testing.T) {
	svc, _, _ := newTestService()

	slot := domain.Slot{
		Name:     "home_banner",
		Geo:      "RU",
		MinPrice: 1_000_000,
		Type:     domain.CreativeTypeBanner,
		Banner:   &domain.Banner{Width: 320, Height: 50},
	}

	first, _ := svc.RegisterSlot(context.Background(), "pub_1", slot)
	second, _ := svc.RegisterSlot(context.Background(), "pub_1", slot)

	if first.ID != second.ID {
		t.Errorf("second call should return same slot: %q vs %q", first.ID, second.ID)
	}
}

func TestService_RegisterSlot_Invalid(t *testing.T) {
	svc, _, _ := newTestService()

	slot := domain.Slot{
		Name: "", // пустое
		Geo:  "RU",
		Type: domain.CreativeTypeBanner,
	}

	_, err := svc.RegisterSlot(context.Background(), "pub_1", slot)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
