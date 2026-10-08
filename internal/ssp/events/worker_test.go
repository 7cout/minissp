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

	published, failed := w.Tick(ctx)

	if published != 2 {
		t.Errorf("published = %d, want 2", published)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
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

	published, failed := w.Tick(ctx)

	if published != 0 {
		t.Errorf("published = %d, want 0", published)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}

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

	published, failed := w.Tick(context.Background())

	if published != 0 || failed != 0 {
		t.Errorf("published = %d, failed = %d, want 0, 0", published, failed)
	}
	if pub.callCount() != 0 {
		t.Errorf("no records — no calls, got %d", pub.callCount())
	}
}

// TestWorker_Run_DrainsBacklogWithoutWaiting проверяет адаптивность:
// при полной очереди воркер крутится без паузы и разгружает
// её быстрее, чем BatchSize/PollInterval.
func TestWorker_Run_DrainsBacklogWithoutWaiting(t *testing.T) {
	store := NewMemoryStore()
	pub := &fakePublisher{}
	// PollInterval большой — если бы воркер спал после каждого
	// Tick, разгрузка 300 записей заняла бы ~30 секунд.
	// Адаптивный воркер должен уложиться в разумное время.
	w := NewWorker(store, pub, WorkerConfig{
		BatchSize:    100,
		PollInterval: 10 * time.Second,
		ErrorBackoff: time.Second,
	})

	ctx := context.Background()
	for i := 0; i < 300; i++ {
		_ = store.Enqueue(ctx, testEvent("a_"+string(rune('0'+i%10))))
	}

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(runCtx)
		close(done)
	}()

	// Ждём, пока воркер разгрузит очередь (или таймаут).
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		left, _ := store.FetchUnpublished(ctx, 1000)
		if len(left) == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	<-done

	left, _ := store.FetchUnpublished(ctx, 1000)
	if len(left) != 0 {
		t.Errorf("unpublished = %d, want 0 (adaptive worker should drain without waiting)", len(left))
	}
}
