// Package metrics содержит метрики SSP.
//
// Все метрики регистрируются автоматически через promauto —
// достаточно импортировать пакет из main, как они появятся
// в /metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ImpressionsTotal — счётчик Impression по статусу.
//
//   - ok         — Impression успешно обработан;
//   - not_found  — аукцион не найден в резерве;
//   - error      — внутренняя ошибка.
var ImpressionsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "ssp",
		Name:      "impressions_total",
		Help:      "Total Impression calls by status.",
	},
	[]string{"status"},
)

// AuctionDuration — длительность RunAuction.
var AuctionDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: "ssp",
		Name:      "auction_duration_seconds",
		Help:      "Duration of RunAuction in seconds.",
		Buckets:   prometheus.DefBuckets,
	},
)

// EventsPublishedTotal — сколько событий успешно ушло в Kafka.
var EventsPublishedTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "ssp",
		Name:      "events_published_total",
		Help:      "Total events successfully published to Kafka.",
	},
)

// EventsPublishErrorsTotal — неудачные попытки публикации.
var EventsPublishErrorsTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "ssp",
		Name:      "events_publish_errors_total",
		Help:      "Total failed event publish attempts.",
	},
)

// EventsUnpublished — gauge, сколько сейчас событий в outbox.
//
// Обновляется воркером после каждого тика (прямой COUNT из store).
var EventsUnpublished = promauto.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "ssp",
		Name:      "events_unpublished",
		Help:      "Number of events currently waiting in outbox.",
	},
)
