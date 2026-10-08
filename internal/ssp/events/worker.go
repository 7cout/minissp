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
	//
	// Верхняя граница одного Tick. Если за один проход удалось
	// опубликовать ровно BatchSize — очередь точно не пуста,
	// и воркер сразу забирает следующую пачку.
	BatchSize int

	// PollInterval — минимальная пауза между проходами, когда
	// очередь пуста (или почти пуста).
	//
	// Не «тик каждые N секунд», а «спать N секунд, если делать
	// нечего». При накопленной очереди воркер крутится без паузы.
	PollInterval time.Duration

	// ErrorBackoff — пауза после неудачной публикации.
	//
	// Защита от busy-loop, если Kafka недоступна: иначе воркер
	// будет непрерывно долбить брокер и захлёстывать логи.
	ErrorBackoff time.Duration
}

// DefaultWorkerConfig — значения по умолчанию.
//
// Подобраны так, чтобы воркер справлялся с несколькими тысячами
// событий в секунду, но не занимал CPU вхолостую при простое.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:    500,
		PollInterval: 1 * time.Second,
		ErrorBackoff: 5 * time.Second,
	}
}

// Worker читает события из outbox и публикует их в Kafka.
//
// Работает в отдельной горутине (запускается через Run). Не блокирует
// бизнес-логику. Адаптивный цикл: если очередь не пуста — крутится
// без паузы, если пуста — спит PollInterval. Это снимает искусственный
// потолок «BatchSize / PollInterval» из старой реализации.
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
	if cfg.ErrorBackoff <= 0 {
		cfg.ErrorBackoff = def.ErrorBackoff
	}
	return &Worker{store: store, publisher: publisher, cfg: cfg}
}

// Run запускает цикл публикации. Возвращает при отмене ctx.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("events: worker started",
		"batch_size", w.cfg.BatchSize,
		"poll_interval", w.cfg.PollInterval,
		"error_backoff", w.cfg.ErrorBackoff,
	)

	for {
		if err := ctx.Err(); err != nil {
			slog.Info("events: worker stopped", "reason", err)
			return
		}

		published, failed := w.Tick(ctx)

		// Решаем, спать или сразу крутиться дальше.
		//
		// Спим, если:
		//   - забрали меньше BatchSize (очередь разгружена);
		//   - были ошибки публикации (не хотим busy-loop).
		//
		// Не спим, если очередь явно не пуста — продолжаем
		// обрабатывать следующую пачку.
		var sleep time.Duration
		switch {
		case failed > 0:
			sleep = w.cfg.ErrorBackoff
		case published < w.cfg.BatchSize:
			sleep = w.cfg.PollInterval
		default:
			sleep = 0
		}

		if sleep == 0 {
			continue
		}

		select {
		case <-ctx.Done():
			slog.Info("events: worker stopped", "reason", ctx.Err())
			return
		case <-time.After(sleep):
		}
	}
}

// Tick выполняет один проход: забирает пачку, публикует, помечает.
//
// Возвращает количество успешно опубликованных и количество
// неудачных записей. Экспортирован, чтобы тесты могли вызывать
// его напрямую.
func (w *Worker) Tick(ctx context.Context) (published, failed int) {
	records, err := w.store.FetchUnpublished(ctx, w.cfg.BatchSize)
	if err != nil {
		slog.WarnContext(ctx, "events: fetch unpublished failed", "error", err)
		return 0, 0
	}
	if len(records) == 0 {
		w.updateUnpublishedGauge(ctx)
		return 0, 0
	}

	for _, r := range records {
		err := w.publisher.Publish(ctx, r.Topic, r.Key, r.Payload)
		if err != nil {
			// Контекст отменён — выходим тихо, всё вернётся
			// на следующем запуске.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return published, failed
			}
			sspmetrics.EventsPublishErrorsTotal.Inc()
			failed++
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

	w.updateUnpublishedGauge(ctx)
	return published, failed
}

// updateUnpublishedGauge обновляет метрику ssp_events_unpublished.
//
// Best-effort: ошибка обновления не должна ломать цикл публикации.
func (w *Worker) updateUnpublishedGauge(ctx context.Context) {
	n, err := w.store.CountUnpublished(ctx)
	if err != nil {
		return
	}
	sspmetrics.EventsUnpublished.Set(float64(n))
}
