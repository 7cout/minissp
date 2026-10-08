package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/ssp/domain"
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
