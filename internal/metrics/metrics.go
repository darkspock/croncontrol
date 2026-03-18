// Package metrics exposes Prometheus metrics for CronControl.
package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	db "github.com/croncontrol/croncontrol/internal/db"
)

var (
	ProcessesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "croncontrol",
		Name:      "processes_total",
		Help:      "Total number of configured processes",
	})

	RunsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "croncontrol",
		Name:      "runs_active",
		Help:      "Number of active runs by state",
	}, []string{"state"})

	RunsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "croncontrol",
		Name:      "runs_completed_total",
		Help:      "Total completed runs",
	})

	RunsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "croncontrol",
		Name:      "runs_failed_total",
		Help:      "Total failed runs",
	})

	RunDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "croncontrol",
		Name:      "run_duration_seconds",
		Help:      "Run execution duration in seconds",
		Buckets:   []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 600, 3600},
	})

	JobsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "croncontrol",
		Name:      "jobs_active",
		Help:      "Number of active jobs by state",
	}, []string{"state"})

	HeartbeatsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "croncontrol",
		Name:      "heartbeats_received_total",
		Help:      "Total heartbeats received",
	})

	APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "croncontrol",
		Name:      "api_requests_total",
		Help:      "Total API requests by method and status",
	}, []string{"method", "path", "status"})

	APILatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "croncontrol",
		Name:      "api_request_duration_seconds",
		Help:      "API request latency in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5},
	}, []string{"method", "path"})

	WorkersOnline = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "croncontrol",
		Name:      "workers_online",
		Help:      "Number of online workers",
	})
)

// Collector periodically updates gauge metrics from the database.
type Collector struct {
	queries  *db.Queries
	interval time.Duration
	stop     chan struct{}
}

// NewCollector creates a new metrics collector.
func NewCollector(queries *db.Queries, interval time.Duration) *Collector {
	return &Collector{
		queries:  queries,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins periodic metric collection.
func (c *Collector) Start(ctx context.Context) {
	go c.loop(ctx)
	slog.Info("metrics collector started", "interval", c.interval)
}

// Stop signals the collector to stop.
func (c *Collector) Stop() {
	close(c.stop)
}

func (c *Collector) loop(ctx context.Context) {
	c.collect(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect(ctx)
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	// Count runs by active states
	for _, state := range []string{"pending", "running", "retrying", "queued", "waiting_for_worker"} {
		count, err := c.queries.CountRuns(ctx, db.CountRunsParams{
			WorkspaceID: "",
			Column2:     "",
			Column3:     state,
			Column4:     "",
		})
		if err == nil {
			RunsActive.WithLabelValues(state).Set(float64(count))
		}
	}
}
