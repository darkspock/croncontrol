// Package dependency resolves process dependencies after run completion.
//
// When a run reaches a final terminal state, this package checks if
// dependent processes should be triggered.
package dependency

import (
	"context"
	"log/slog"
	"time"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/runstate"
)

// Resolver evaluates process dependencies after run completion.
type Resolver struct {
	queries *db.Queries
}

// New creates a new dependency Resolver.
func New(queries *db.Queries) *Resolver {
	return &Resolver{queries: queries}
}

// Evaluate checks if any processes depend on the completed run's process
// and creates new runs for them if conditions are met.
//
// Must be called only when a run reaches a final terminal state.
func (r *Resolver) Evaluate(ctx context.Context, completedRun db.Run) error {
	state := runstate.State(completedRun.State)
	if !runstate.IsFinalTerminal(state) {
		return nil
	}

	processID := completedRun.ProcessID
	dependents, err := r.queries.GetDependentProcesses(ctx, &processID)
	if err != nil {
		return err
	}

	for _, dep := range dependents {
		if dep.DependencyType == nil {
			continue
		}

		switch *dep.DependencyType {
		case "after":
			// Trigger regardless of result
			if err := r.createDependentRun(ctx, dep, completedRun); err != nil {
				slog.Error("dependency: create run (after)", "process", dep.ID, "error", err)
			}

		case "after_success":
			// Trigger only if parent completed successfully
			if state == runstate.Completed {
				if err := r.createDependentRun(ctx, dep, completedRun); err != nil {
					slog.Error("dependency: create run (after_success)", "process", dep.ID, "error", err)
				}
			} else {
				slog.Info("dependency: skipped (parent not completed)", "process", dep.ID, "parent_state", state)
			}
		}
	}

	return nil
}

func (r *Resolver) createDependentRun(ctx context.Context, process db.Process, parentRun db.Run) error {
	now := time.Now().UTC()

	_, err := r.queries.CreateRun(ctx, db.CreateRunParams{
		ID:               id.NewRun(),
		WorkspaceID:      process.WorkspaceID,
		ProcessID:        process.ID,
		ScheduledAt:      dbutil.Timestamptz(now),
		State:            string(runstate.Pending),
		Origin:           "dependency",
		MaxAttempts:       process.MaxAttempts,
		ActorType:        strPtr("system"),
		TriggeredByRunID: &parentRun.ID,
		Tags:             process.Tags,
	})
	if err != nil {
		return err
	}

	slog.Info("dependency: triggered",
		"process", process.Name,
		"triggered_by", parentRun.ID,
	)
	return nil
}

// ValidateNoCycles checks that adding a dependency from childID → parentID
// does not create a circular dependency chain.
func ValidateNoCycles(ctx context.Context, queries *db.Queries, childID, parentID string) error {
	visited := map[string]bool{childID: true}
	current := parentID

	for current != "" {
		if visited[current] {
			return &CycleError{ProcessID: childID, CycleAt: current}
		}
		visited[current] = true

		proc, err := queries.GetProcess(ctx, db.GetProcessParams{
			ID:          current,
			WorkspaceID: "", // need workspace scoping
		})
		if err != nil {
			return nil // process not found, no cycle
		}

		if proc.DependsOnProcessID == nil {
			break
		}
		current = *proc.DependsOnProcessID
	}

	return nil
}

// CycleError indicates a circular dependency was detected.
type CycleError struct {
	ProcessID string
	CycleAt   string
}

func (e *CycleError) Error() string {
	return "circular dependency detected at process " + e.CycleAt
}

func strPtr(s string) *string { return &s }
