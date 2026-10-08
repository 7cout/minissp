package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	sspmetrics "github.com/7cout/minissp/internal/ssp/metrics"
)

// WorkerConfig — параметры воркера.
type WorkerConfig struct {
	// BatchSize — сколько записей разом забирать из outbox.
	BatchSize int

	// PollInterval — как часто проверять outbox.
	PollInterval time.Duration
}

// DefaultWorkerConfig — значения по умолчанию.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:    100,
		PollInterval: 2 * time.Second,
	}
}

// Worker читает события из outbox и публикует их в Kafka.
//
// Работает в отдельной горутине (запускается через Run). Не блокирует
// бизнес-логику. Если публикация упала — оставляет запись в outbox,
// следующий тик попробует снова.
type Worker struct {
	store     Store
	publisher Publisher
	cfg       WorkerConfig
}

// NewWorker создаёт воркер.
//
// Нулевые поля конфига заменяются значениями по умолчанию.
func NewWorker(store Store, publisher Publisher, cfg WorkerConfig) *Worker {
	def := DefaultWorkerConfig()
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = def.BatchSize
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = def.PollInterval
	}
	return &Worker{store: store, publisher: publisher, cfg: cfg}
}

// Run запускает цикл публикации. Возвращает при отмене ctx.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	slog.Info("events: worker started",
		"poll_interval", w.cfg.PollInterval,
		"batch_size", w.cfg.BatchSize,
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("events: worker stopped")
			return
		case <-ticker.C:
			w.Tick(ctx)
		}
	}
}

// Tick выполняет один проход: забирает пачку, публикует, помечает.
//
// Экспортирован, чтобы тесты могли вызывать его напрямую.
func (w *Worker) Tick(ctx context.Context) {
	records, err := w.store.FetchUnpublished(ctx, w.cfg.BatchSize)
	if err != nil {
		slog.WarnContext(ctx, "events: fetch unpublished failed", "error", err)
		return
	}

	published := 0
	for _, r := range records {
		err := w.publisher.Publish(ctx, r.Topic, r.Key, r.Payload)
		if err != nil {
			// Контекст отменён — выходим тихо, всё вернётся
			// на следующем запуске.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			sspmetrics.EventsPublishErrorsTotal.Inc()
			slog.WarnContext(ctx, "events: publish failed",
				"id", r.ID,
				"topic", r.Topic,
				"attempts", r.Attempts+1,
				"error", err,
			)
			if err := w.store.MarkFailed(ctx, r.ID, err); err != nil {
				slog.WarnContext(ctx, "events: mark failed",
					"id", r.ID, "error", err)
			}
			continue
		}

		if err := w.store.MarkPublished(ctx, r.ID); err != nil {
			// Публикация прошла, но отметку поставить не удалось —
			// в следующий тик опубликуем повторно. Consumer должен
			// быть идемпотентным (у нас ключ — auction_id).
			slog.WarnContext(ctx, "events: mark published failed",
				"id", r.ID, "error", err)
			continue
		}
		published++
	}

	if published > 0 {
		sspmetrics.EventsPublishedTotal.Add(float64(published))
		slog.InfoContext(ctx, "events: batch published", "count", published)
	}

	// Обновляем gauge после обработки.
	//
	// Делаем это всегда, даже если пачка была пустой — вдруг
	// предыдущий тик не смог обновить.
	if n, err := w.store.CountUnpublished(ctx); err == nil {
		sspmetrics.EventsUnpublished.Set(float64(n))
	}
}
