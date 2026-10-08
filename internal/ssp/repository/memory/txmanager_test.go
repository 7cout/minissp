package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/events"
)

func TestTxManager_RecordImpression(t *testing.T) {
	pubs := NewPublisherRepo()
	_ = pubs.Add(context.Background(), &domain.Publisher{
		ID:     "pub_1",
		Name:   "T2",
		APIKey: "key",
	})

	store := events.NewMemoryStore()
	tx := NewTxManager(pubs, store)
	ctx := context.Background()

	evt := events.ImpressionEvent{
		AuctionID:      "a_1",
		PublisherID:    "pub_1",
		PublisherShare: 1_200_000,
	}
	if err := tx.RecordImpression(ctx, "pub_1", 1_200_000, evt); err != nil {
		t.Fatalf("record: %v", err)
	}

	// Баланс начислен.
	p, _ := pubs.Get(ctx, "pub_1")
	if p.Balance != 1_200_000 {
		t.Errorf("balance = %d, want 1200000", p.Balance)
	}

	// Событие в outbox.
	records, _ := store.FetchUnpublished(ctx, 10)
	if len(records) != 1 {
		t.Errorf("outbox records = %d, want 1", len(records))
	}
}

func TestTxManager_RecordImpression_PublisherNotFound(t *testing.T) {
	pubs := NewPublisherRepo()
	store := events.NewMemoryStore()
	tx := NewTxManager(pubs, store)

	err := tx.RecordImpression(
		context.Background(),
		"missing",
		100,
		events.ImpressionEvent{AuctionID: "a_1"},
	)
	if !errors.Is(err, domain.ErrPublisherNotFound) {
		t.Errorf("want ErrPublisherNotFound, got %v", err)
	}

	// В outbox ничего не попало.
	records, _ := store.FetchUnpublished(context.Background(), 10)
	if len(records) != 0 {
		t.Errorf("outbox should be empty on failure, got %d", len(records))
	}
}
