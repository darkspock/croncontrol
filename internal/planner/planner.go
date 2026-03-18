// Package planner materializes future runs from cron process configurations.
//
// The planner runs periodically and creates run records for upcoming scheduled times.
// It only handles schedule_type='cron'. Fixed-delay and on-demand are managed elsewhere.
package planner

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
)

// Planner materializes scheduled runs from cron configurations.
type Planner struct {
	queries  *db.Queries
	pool     *pgxpool.Pool
	interval time.Duration
	horizon  time.Duration
	stop     chan struct{}
}

// New creates a new Planner.
func New(pool *pgxpool.Pool, interval, horizon time.Duration) *Planner {
	return &Planner{
		queries:  db.New(pool),
		pool:     pool,
		interval: interval,
		horizon:  horizon,
		stop:     make(chan struct{}),
	}
}

// Start begins the planning loop in a goroutine.
func (p *Planner) Start(ctx context.Context) {
	go p.loop(ctx)
	slog.Info("planner started", "interval", p.interval, "horizon", p.horizon)
}

// Stop signals the planner to stop.
func (p *Planner) Stop() {
	close(p.stop)
	slog.Info("planner stopped")
}

func (p *Planner) loop(ctx context.Context) {
	// Run immediately on start
	p.plan(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.plan(ctx)
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// plan evaluates all enabled cron processes and creates runs for the upcoming horizon.
func (p *Planner) plan(ctx context.Context) {
	processes, err := p.queries.ListEnabledCronProcesses(ctx)
	if err != nil {
		slog.Error("planner: list cron processes", "error", err)
		return
	}

	now := time.Now().UTC()
	end := now.Add(p.horizon)
	created := 0

	for _, proc := range processes {
		if proc.Schedule == nil || *proc.Schedule == "" {
			continue
		}

		tz := "UTC"
		if proc.Timezone != nil && *proc.Timezone != "" {
			tz = *proc.Timezone
		}

		loc, err := time.LoadLocation(tz)
		if err != nil {
			slog.Error("planner: invalid timezone", "process", proc.ID, "timezone", tz, "error", err)
			continue
		}

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*proc.Schedule)
		if err != nil {
			slog.Error("planner: invalid cron expression", "process", proc.ID, "schedule", *proc.Schedule, "error", err)
			continue
		}

		// Iterate through scheduled times in the planning window
		next := schedule.Next(now.In(loc).Add(-time.Minute)).UTC()
		for !next.After(end) {
			if next.Before(now) {
				next = schedule.Next(next.In(loc)).UTC()
				continue
			}

			// Check if run already exists (idempotent)
			exists, err := p.queries.RunExists(ctx, db.RunExistsParams{
				ProcessID:   proc.ID,
				ScheduledAt: dbutil.Timestamptz(next),
				Origin:      "cron",
			})
			if err != nil {
				slog.Error("planner: check run exists", "error", err)
				break
			}
			if exists {
				next = schedule.Next(next.In(loc)).UTC()
				continue
			}

			// Create the run
			_, err = p.queries.CreateRun(ctx, db.CreateRunParams{
				ID:          id.NewRun(),
				WorkspaceID: proc.WorkspaceID,
				ProcessID:   proc.ID,
				ScheduledAt: dbutil.Timestamptz(next),
				State:       "pending",
				Origin:      "cron",
				MaxAttempts: proc.MaxAttempts,
				ActorType:   strPtr("system"),
				Tags:        proc.Tags,
			})
			if err != nil {
				slog.Error("planner: create run", "process", proc.ID, "scheduled_at", next, "error", err)
				break
			}

			created++
			next = schedule.Next(next.In(loc)).UTC()
		}
	}

	if created > 0 {
		slog.Info("planner: created runs", "count", created)
	}
}

func strPtr(s string) *string { return &s }
