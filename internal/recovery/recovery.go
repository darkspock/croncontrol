// Package recovery handles startup recovery: orphan detection, missed-run creation,
// and fixed-delay chain resumption.
package recovery

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/runstate"
)

// Run performs all startup recovery tasks.
func Run(ctx context.Context, queries *db.Queries) error {
	slog.Info("recovery: starting")

	if err := detectOrphans(ctx, queries); err != nil {
		return err
	}

	if err := recoverCronProcesses(ctx, queries); err != nil {
		return err
	}

	if err := recoverFixedDelayProcesses(ctx, queries); err != nil {
		return err
	}

	slog.Info("recovery: complete")
	return nil
}

// detectOrphans marks all runs that were 'running' before restart as 'hung'.
func detectOrphans(ctx context.Context, queries *db.Queries) error {
	runs, err := queries.ListRunningRuns(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, run := range runs {
		err := queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:         run.ID,
			State:      string(runstate.Hung),
			FinishedAt: dbutil.Timestamptz(now),
		})
		if err != nil {
			slog.Error("recovery: mark orphan hung", "run_id", run.ID, "error", err)
			continue
		}
		slog.Warn("recovery: orphaned run marked hung", "run_id", run.ID, "process_id", run.ProcessID)
	}

	if len(runs) > 0 {
		slog.Info("recovery: orphans detected", "count", len(runs))
	}
	return nil
}

// recoverCronProcesses creates recovery runs for cron processes with miss_policy='execute'.
func recoverCronProcesses(ctx context.Context, queries *db.Queries) error {
	processes, err := queries.ListEnabledCronProcesses(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	totalCreated := 0

	for _, proc := range processes {
		if proc.MissPolicy == nil || *proc.MissPolicy != "execute" {
			continue
		}
		if proc.Schedule == nil || *proc.Schedule == "" {
			continue
		}
		// Skip restricted/suspended workspaces
		if proc.WorkspaceState != "active" {
			continue
		}

		// Find last executed run
		lastRun, err := queries.GetLastRunByProcess(ctx, proc.ID)
		if err != nil {
			continue // no previous runs, nothing to recover
		}

		// Parse schedule
		tz := "UTC"
		if proc.Timezone != nil && *proc.Timezone != "" {
			tz = *proc.Timezone
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			continue
		}

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*proc.Schedule)
		if err != nil {
			continue
		}

		// Find missed times between last run and now
		var lastTime time.Time
		if lastRun.ScheduledAt.Valid {
			lastTime = lastRun.ScheduledAt.Time
		} else {
			continue
		}

		created := 0
		maxRecovery := int(proc.MaxRecoverySlots)
		next := schedule.Next(lastTime.In(loc)).UTC()

		for next.Before(now) && created < maxRecovery {
			// Check if run already exists
			exists, err := queries.RunExists(ctx, db.RunExistsParams{
				ProcessID:   proc.ID,
				ScheduledAt: dbutil.Timestamptz(next),
				Origin:      "recovery",
			})
			if err != nil {
				break
			}
			if !exists {
				_, err = queries.CreateRun(ctx, db.CreateRunParams{
					ID:          id.NewRun(),
					WorkspaceID: proc.WorkspaceID,
					ProcessID:   proc.ID,
					ScheduledAt: dbutil.Timestamptz(next),
					State:       string(runstate.Pending),
					Origin:      "recovery",
					MaxAttempts: proc.MaxAttempts,
					ActorType:   strPtr("system"),
					Tags:        proc.Tags,
				})
				if err != nil {
					slog.Error("recovery: create cron recovery run", "error", err)
					break
				}
				created++
			}

			next = schedule.Next(next.In(loc)).UTC()
		}

		if created > 0 {
			slog.Info("recovery: cron recovery runs created",
				"process", proc.Name, "count", created, "max", maxRecovery)
			totalCreated += created
		}
	}

	if totalCreated > 0 {
		slog.Info("recovery: total cron recovery runs", "count", totalCreated)
	}
	return nil
}

// recoverFixedDelayProcesses resumes fixed-delay chains that were interrupted.
func recoverFixedDelayProcesses(ctx context.Context, queries *db.Queries) error {
	processes, err := queries.ListEnabledFixedDelayProcesses(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	created := 0

	for _, proc := range processes {
		if proc.WorkspaceState != "active" {
			continue
		}

		lastRun, err := queries.GetLastRunByProcess(ctx, proc.ID)
		if err != nil {
			// No previous runs — create initial run
			_, err = queries.CreateRun(ctx, db.CreateRunParams{
				ID:          id.NewRun(),
				WorkspaceID: proc.WorkspaceID,
				ProcessID:   proc.ID,
				ScheduledAt: dbutil.Timestamptz(now),
				State:       string(runstate.Pending),
				Origin:      "recovery",
				MaxAttempts: proc.MaxAttempts,
				ActorType:   strPtr("system"),
				Tags:        proc.Tags,
			})
			if err == nil {
				created++
			}
			continue
		}

		state := runstate.State(lastRun.State)

		// If last run is in a terminal state and no pending run exists, resume chain
		if runstate.IsFinalTerminal(state) || state == runstate.Hung {
			_, err = queries.CreateRun(ctx, db.CreateRunParams{
				ID:          id.NewRun(),
				WorkspaceID: proc.WorkspaceID,
				ProcessID:   proc.ID,
				ScheduledAt: dbutil.Timestamptz(now),
				State:       string(runstate.Pending),
				Origin:      "recovery",
				MaxAttempts: proc.MaxAttempts,
				ActorType:   strPtr("system"),
				Tags:        proc.Tags,
			})
			if err == nil {
				created++
				slog.Info("recovery: fixed-delay chain resumed", "process", proc.Name)
			}
		}
	}

	if created > 0 {
		slog.Info("recovery: fixed-delay recovery runs", "count", created)
	}
	return nil
}

func strPtr(s string) *string { return &s }
