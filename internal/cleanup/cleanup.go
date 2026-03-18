// Package cleanup handles periodic deletion of old terminal records.
//
// Respects configured retention periods. Deletes in batches to avoid
// long transactions. Only deletes terminal-state records.
package cleanup

import (
	"context"
	"log/slog"
	"time"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config for the cleanup scheduler.
type Config struct {
	RunRetention   time.Duration
	JobRetention   time.Duration
	AuditRetention time.Duration
	BatchSize      int32
	Interval       time.Duration
}

// Scheduler runs periodic cleanup of old records.
type Scheduler struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	config  Config
	stop    chan struct{}
}

// New creates a new cleanup Scheduler.
func New(pool *pgxpool.Pool, cfg Config) *Scheduler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 24 * time.Hour
	}
	return &Scheduler{
		queries: db.New(pool),
		pool:    pool,
		config:  cfg,
		stop:    make(chan struct{}),
	}
}

// Start begins the cleanup loop.
func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
	slog.Info("cleanup scheduler started", "interval", s.config.Interval)
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stop)
}

// RunNow triggers an immediate cleanup cycle (for API-triggered manual cleanup).
func (s *Scheduler) RunNow(ctx context.Context) {
	s.clean(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.clean(ctx)
		case <-s.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) clean(ctx context.Context) {
	start := time.Now()
	slog.Info("cleanup: starting")

	totalDeleted := int64(0)

	// Delete old runs
	if s.config.RunRetention > 0 {
		cutoff := time.Now().UTC().Add(-s.config.RunRetention)
		deleted, err := s.queries.DeleteOldRuns(ctx, db.DeleteOldRunsParams{
			FinishedAt: dbutil.Timestamptz(cutoff),
			Limit:      s.config.BatchSize,
		})
		if err != nil {
			slog.Error("cleanup: delete old runs", "error", err)
		} else {
			totalDeleted += deleted
			if deleted > 0 {
				slog.Info("cleanup: deleted runs", "count", deleted, "cutoff", cutoff)
			}
		}
	}

	// Delete old jobs
	if s.config.JobRetention > 0 {
		cutoff := time.Now().UTC().Add(-s.config.JobRetention)
		deleted, err := s.queries.DeleteOldJobs(ctx, db.DeleteOldJobsParams{
			UpdatedAt: dbutil.Timestamptz(cutoff),
			Limit:     s.config.BatchSize,
		})
		if err != nil {
			slog.Error("cleanup: delete old jobs", "error", err)
		} else {
			totalDeleted += deleted
			if deleted > 0 {
				slog.Info("cleanup: deleted jobs", "count", deleted, "cutoff", cutoff)
			}
		}
	}

	// Delete old audit entries
	if s.config.AuditRetention > 0 {
		cutoff := time.Now().UTC().Add(-s.config.AuditRetention)
		deleted, err := s.queries.DeleteOldAudit(ctx, db.DeleteOldAuditParams{
			CreatedAt: dbutil.Timestamptz(cutoff),
			Limit:     s.config.BatchSize,
		})
		if err != nil {
			slog.Error("cleanup: delete old audit", "error", err)
		} else {
			totalDeleted += deleted
			if deleted > 0 {
				slog.Info("cleanup: deleted audit entries", "count", deleted, "cutoff", cutoff)
			}
		}
	}

	duration := time.Since(start)
	slog.Info("cleanup: complete", "total_deleted", totalDeleted, "duration", duration)
}
