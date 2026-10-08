package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakePublisher — Publisher для тестов.
type fakePublisher struct {
	mu      sync.Mutex
	calls   []publishCall
	err     error
	blockCh chan struct{}
}

type publishCall struct {
	topic   string
	key     string
	payload []byte
}

func (f *fakePublisher) Publish(_ context.Context, topic, key string, payload []byte) error {
	if f.blockCh != nil {
		<-f.blockCh
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, publishCall{topic, key, payload})
	return f.err
}

func (f *fakePublisher) Close() error { return nil }

func (f *fakePublisher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestWorker_Tick_PublishesAndMarks(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{}
	w := NewWorker(store, pub, WorkerConfig{BatchSize: 10, PollInterval: time.Hour})
	ctx := context.Background()

	_ = store.Enqueue(ctx, testEvent("a_1"))
	_ = store.Enqueue(ctx, testEvent("a_2"))

	w.Tick(ctx)

	if pub.callCount() != 2 {
		t.Errorf("publish calls = %d, want 2", pub.callCount())
	}

	// После Tick ничего не должно остаться неопубликованным.
	left, _ := store.FetchUnpublished(ctx, 10)
	if len(left) != 0 {
		t.Errorf("unpublished left = %d, want 0", len(left))
	}
}

func TestWorker_Tick_PublishFails_RecordStays(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{err: errors.New("kafka down")}
	w := NewWorker(store, pub, WorkerConfig{BatchSize: 10, PollInterval: time.Hour})
	ctx := context.Background()

	_ = store.Enqueue(ctx, testEvent("a_1"))

	w.Tick(ctx)

	left, _ := store.FetchUnpublished(ctx, 10)
	if len(left) != 1 {
		t.Fatalf("unpublished = %d, want 1 (record must stay)", len(left))
	}
	if left[0].Attempts != 1 {
		t.Errorf("attempts = %d, want 1", left[0].Attempts)
	}
}

func TestWorker_Tick_EmptyStore(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{}
	w := NewWorker(store, pub, WorkerConfig{BatchSize: 10, PollInterval: time.Hour})

	w.Tick(context.Background())

	if pub.callCount() != 0 {
		t.Errorf("no records — no calls, got %d", pub.callCount())
	}
}
