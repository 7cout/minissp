package events

import (
	"context"
	"testing"
	"time"
)

func testEvent(id string) ImpressionEvent {
	return ImpressionEvent{
		AuctionID:      id,
		ImpID:          "imp_" + id,
		CampaignID:     "camp_1",
		CreativeID:     "cr_1",
		PublisherID:    "pub_1",
		SlotID:         "slot_1",
		BidderID:       "dsp-nike",
		Price:          1_500_000,
		PublisherShare: 1_200_000,
		PlatformFee:    300_000,
		CreatedAt:      time.Now().Add(-time.Second),
		OccurredAt:     time.Now(),
	}
}

func TestMemoryStore_EnqueueAndFetch(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Enqueue(ctx, testEvent("a_1")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	records, err := s.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	if records[0].Topic != TopicImpression {
		t.Errorf("topic = %q, want %q", records[0].Topic, TopicImpression)
	}
	if records[0].Key != "a_1" {
		t.Errorf("key = %q, want a_1", records[0].Key)
	}
	if len(records[0].Payload) == 0 {
		t.Error("payload is empty")
	}
}

func TestMemoryStore_MarkPublished(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_ = s.Enqueue(ctx, testEvent("a_1"))
	records, _ := s.FetchUnpublished(ctx, 10)

	if err := s.MarkPublished(ctx, records[0].ID); err != nil {
		t.Fatalf("mark: %v", err)
	}

	after, _ := s.FetchUnpublished(ctx, 10)
	if len(after) != 0 {
		t.Errorf("want 0 unpublished, got %d", len(after))
	}
}

func TestMemoryStore_MarkFailed(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_ = s.Enqueue(ctx, testEvent("a_1"))
	records, _ := s.FetchUnpublished(ctx, 10)

	_ = s.MarkFailed(ctx, records[0].ID, nil)
	_ = s.MarkFailed(ctx, records[0].ID, nil)

	after, _ := s.FetchUnpublished(ctx, 10)
	if after[0].Attempts != 2 {
		t.Errorf("attempts = %d, want 2", after[0].Attempts)
	}
}

func TestMemoryStore_FetchLimit(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = s.Enqueue(ctx, testEvent("a_"+string(rune('0'+i))))
	}

	records, _ := s.FetchUnpublished(ctx, 3)
	if len(records) != 3 {
		t.Errorf("want 3 records (limit), got %d", len(records))
	}
}
