// Package metrics содержит метрики DSP.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BidsTotal — счётчик GetBid по статусу.
//
//   - ok          — ставка найдена и зарезервирована;
//   - no_eligible — ни одна кампания не подошла;
//   - error       — внутренняя ошибка.
var BidsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "dsp",
		Name:      "bids_total",
		Help:      "Total GetBid calls by status.",
	},
	[]string{"status"},
)

// BidDuration — длительность GetBid.
var BidDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: "dsp",
		Name:      "bid_duration_seconds",
		Help:      "Duration of GetBid in seconds.",
		Buckets:   prometheus.DefBuckets,
	},
)

// CommitsTotal — счётчик Commit по статусу.
var CommitsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "dsp",
		Name:      "commits_total",
		Help:      "Total Commit calls by status.",
	},
	[]string{"status"},
)

// RollbacksTotal — счётчик Rollback по статусу.
var RollbacksTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "dsp",
		Name:      "rollbacks_total",
		Help:      "Total Rollback calls by status.",
	},
	[]string{"status"},
)
