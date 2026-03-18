// Package monitor detects hung runs by checking execution and heartbeat timeouts.
//
// Runs periodically, queries all running runs, and applies timeout_action
// (kill, alert, or both) when timeouts are exceeded.
package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/runstate"
)

// KillFunc is called when a run needs to be killed.
type KillFunc func(runID string) error

// AlertFunc is called when a run needs an alert sent.
type AlertFunc func(ctx context.Context, run db.ListRunningRunsRow, reason string)

// Monitor checks running runs for timeouts.
type Monitor struct {
	queries  *db.Queries
	pool     *pgxpool.Pool
	interval time.Duration
	killFn   KillFunc
	alertFn  AlertFunc
	stop     chan struct{}
}

// New creates a new Monitor.
func New(pool *pgxpool.Pool, interval time.Duration, killFn KillFunc, alertFn AlertFunc) *Monitor {
	return &Monitor{
		queries:  db.New(pool),
		pool:     pool,
		interval: interval,
		killFn:   killFn,
		alertFn:  alertFn,
		stop:     make(chan struct{}),
	}
}

// Start begins the monitoring loop.
func (m *Monitor) Start(ctx context.Context) {
	go m.loop(ctx)
	slog.Info("monitor started", "interval", m.interval)
}

// Stop signals the monitor to stop.
func (m *Monitor) Stop() {
	close(m.stop)
	slog.Info("monitor stopped")
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.check(ctx)
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) check(ctx context.Context) {
	runs, err := m.queries.ListRunningRuns(ctx)
	if err != nil {
		slog.Error("monitor: list running runs", "error", err)
		return
	}

	now := time.Now().UTC()

	for _, run := range runs {
		if !run.StartedAt.Valid {
			continue
		}

		startedAt := run.StartedAt.Time
		hung := false
		reason := ""

		// Check execution timeout
		if run.ExecutionTimeout.Valid {
			timeout := intervalToDuration(run.ExecutionTimeout)
			if timeout > 0 && now.Sub(startedAt) > timeout {
				hung = true
				reason = "execution_timeout"
			}
		}

		// Check heartbeat timeout (non-HTTP only, when configured)
		if !hung && run.HeartbeatTimeout.Valid && run.ExecutionMethod != "http" {
			hbTimeout := intervalToDuration(run.HeartbeatTimeout)
			if hbTimeout > 0 {
				var lastHB time.Time
				if run.LastHeartbeatAt.Valid {
					lastHB = run.LastHeartbeatAt.Time
				} else {
					lastHB = startedAt // no heartbeat yet, count from start
				}
				if now.Sub(lastHB) > hbTimeout {
					hung = true
					reason = "heartbeat_timeout"
				}
			}
		}

		if !hung {
			continue
		}

		log := slog.With("run_id", run.ID, "reason", reason)
		log.Warn("monitor: run detected as hung")

		// Mark as hung
		err := m.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:         run.ID,
			State:      string(runstate.Hung),
			FinishedAt: dbutil.Timestamptz(now),
		})
		if err != nil {
			log.Error("monitor: update hung state", "error", err)
			continue
		}

		// Apply timeout_action
		action := "both"
		if run.TimeoutAction != "" {
			action = run.TimeoutAction
		}

		switch action {
		case "kill":
			m.doKill(run.ID, log)
		case "alert":
			m.doAlert(ctx, run, reason, log)
		case "both":
			m.doKill(run.ID, log)
			m.doAlert(ctx, run, reason, log)
		}
	}
}

func (m *Monitor) doKill(runID string, log *slog.Logger) {
	if m.killFn == nil {
		return
	}
	if err := m.killFn(runID); err != nil {
		log.Error("monitor: kill failed", "error", err)
	}
}

func (m *Monitor) doAlert(ctx context.Context, run db.ListRunningRunsRow, reason string, log *slog.Logger) {
	if m.alertFn == nil {
		return
	}
	m.alertFn(ctx, run, reason)
}

// intervalToDuration converts a pgtype.Interval to time.Duration.
// PostgreSQL intervals store months, days, and microseconds.
func intervalToDuration(iv pgtype.Interval) time.Duration {
	if !iv.Valid {
		return 0
	}
	// Microseconds is the primary component for short durations.
	// Days and months are approximated.
	d := time.Duration(iv.Microseconds) * time.Microsecond
	d += time.Duration(iv.Days) * 24 * time.Hour
	d += time.Duration(iv.Months) * 30 * 24 * time.Hour
	return d
}
