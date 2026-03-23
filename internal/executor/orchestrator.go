package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/runstate"
)

// OnRunTerminal is called when a run reaches a final terminal state.
// Used for dependency resolution and fixed-delay chain creation.
type OnRunTerminal func(ctx context.Context, run db.Run, proc db.Process)

// LogBackend writes operational data to the configured logging backend.
type LogBackend interface {
	WriteRunOutput(ctx context.Context, runID, stream, content string) error
}

// WorkerDispatchFunc routes a run to the worker runtime.
type WorkerDispatchFunc func(ctx context.Context, run db.Run, proc db.Process) error

// Orchestrator claims pending runs and dispatches them to execution methods.
type Orchestrator struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	registry       *Registry
	interval       time.Duration
	onRunTerminal  OnRunTerminal
	workerDispatch WorkerDispatchFunc
	logBackend     LogBackend
	stop           chan struct{}
	running        sync.Map // map[runID]Handle
}

// NewOrchestrator creates a new executor orchestrator.
func NewOrchestrator(pool *pgxpool.Pool, registry *Registry, interval time.Duration) *Orchestrator {
	return &Orchestrator{
		pool:     pool,
		queries:  db.New(pool),
		registry: registry,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// SetLogBackend sets the logging backend for writing run output.
func (o *Orchestrator) SetLogBackend(lb LogBackend) {
	o.logBackend = lb
}

// SetOnRunTerminal sets the callback for when runs reach terminal state.
func (o *Orchestrator) SetOnRunTerminal(fn OnRunTerminal) {
	o.onRunTerminal = fn
}

// SetWorkerDispatch sets the callback used to route worker-runtime runs.
func (o *Orchestrator) SetWorkerDispatch(fn WorkerDispatchFunc) {
	o.workerDispatch = fn
}

func (o *Orchestrator) Start(ctx context.Context) {
	go o.loop(ctx)
	slog.Info("executor started", "interval", o.interval)
}

func (o *Orchestrator) Stop() {
	close(o.stop)
	slog.Info("executor stopped")
}

func (o *Orchestrator) Kill(runID string) error {
	val, ok := o.running.Load(runID)
	if ok {
		handle := val.(Handle)
		method, ok := o.registry.Get(handle.MethodName)
		if !ok {
			return fmt.Errorf("executor: kill: method %q not found in registry", handle.MethodName)
		}
		return method.Kill(context.Background(), handle)
	}
	handle, ok, err := o.loadPersistedRunHandle(context.Background(), runID)
	if err != nil || !ok {
		return err
	}
	method, ok := o.registry.Get(handle.MethodName)
	if !ok {
		return fmt.Errorf("executor: kill: method %q not found in registry", handle.MethodName)
	}
	return method.Kill(context.Background(), handle)
}

func (o *Orchestrator) loop(ctx context.Context) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			o.tick(ctx)
			o.processKillRequests(ctx)
			o.processAsyncRuns(ctx)
		case <-o.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (o *Orchestrator) tick(ctx context.Context) {
	// Claim pending runs
	tx, err := o.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	qtx := o.queries.WithTx(tx)
	runs, err := qtx.ClaimPendingRuns(ctx, 10)
	if err != nil {
		return
	}
	retrying, err := qtx.ClaimRetryingRuns(ctx, 10)
	if err != nil {
		return
	}
	runs = append(runs, retrying...)

	if err := tx.Commit(ctx); err != nil {
		return
	}

	for _, run := range runs {
		go o.dispatch(ctx, run)
	}
}

func (o *Orchestrator) processKillRequests(ctx context.Context) {
	runs, err := o.queries.ListKillRequestedRuns(ctx)
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	for _, run := range runs {
		wg.Add(1)
		go func(run db.ListKillRequestedRunsRow) {
			defer wg.Done()
			log := slog.With("run_id", run.ID)
			if err := o.Kill(run.ID); err != nil {
				log.Error("executor: kill failed", "error", err)
				return
			}
			// Intentionally not transitioning state here. The killed state is only
			// written once the async poll confirms the process was actually killed.
			// Until then, the run stays in kill_requested and will be re-listed on
			// the next tick. This is by design per the epic spec.
			log.Debug("executor: kill signal sent, awaiting poll confirmation")
		}(run)
	}
	wg.Wait()
}

func (o *Orchestrator) dispatch(ctx context.Context, run db.Run) {
	log := slog.With("run_id", run.ID, "process_id", run.ProcessID)

	proc, err := o.queries.GetProcess(ctx, db.GetProcessParams{
		ID:          run.ProcessID,
		WorkspaceID: run.WorkspaceID,
	})
	if err != nil {
		log.Error("executor: get process", "error", err)
		return
	}

	// Check parallelism (cron only)
	if proc.ScheduleType == "cron" && !proc.AllowParallel {
		activeCount, err := o.queries.CountActiveByProcess(ctx, proc.ID)
		if err != nil {
			log.Error("executor: count active", "error", err)
			return
		}
		if activeCount > 1 {
			switch proc.OnOverlap {
			case "skip":
				o.updateState(ctx, run.ID, runstate.Skipped)
				log.Info("executor: run skipped (overlap)")
				o.handleTerminal(ctx, run, proc, runstate.Skipped)
				return
			case "queue":
				o.updateState(ctx, run.ID, runstate.Queued)
				log.Info("executor: run queued (overlap)")
				return
			}
		}
	}

	// Check if worker runtime is required
	if proc.Runtime == "worker" {
		if o.workerDispatch != nil {
			if err := o.workerDispatch(ctx, run, proc); err != nil {
				log.Error("executor: worker dispatch failed", "error", err)
				o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
					ID:            run.ID,
					State:         string(runstate.WaitingForWorker),
					WaitingReason: strPtr("Worker dispatch failed: " + err.Error()),
				})
				return
			}
			return
		}
		o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:            run.ID,
			State:         string(runstate.WaitingForWorker),
			WaitingReason: strPtr("Waiting for available worker"),
		})
		log.Info("executor: waiting for worker", "runtime", proc.Runtime)
		// Worker dispatcher will pick this up via its own polling
		return
	}

	method, ok := o.registry.Get(proc.ExecutionMethod)
	if !ok {
		log.Error("executor: unknown method", "method", proc.ExecutionMethod)
		o.updateStateFailed(ctx, run.ID, -1)
		return
	}

	// Transition to running
	now := time.Now().UTC()
	attempt := run.Attempt + 1
	o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
		ID:        run.ID,
		State:     string(runstate.Running),
		StartedAt: dbutil.Timestamptz(now),
		Attempt:   &attempt,
	})

	// Snapshot config on first attempt
	if attempt == 1 {
		snapshot, _ := json.Marshal(map[string]any{
			"execution_method": proc.ExecutionMethod,
			"method_config":    string(proc.MethodConfig),
			"runtime":          proc.Runtime,
		})
		o.queries.SnapshotRunConfig(ctx, db.SnapshotRunConfigParams{
			ID:              run.ID,
			EffectiveConfig: snapshot,
		})
	}

	// Create run attempt record
	attemptID := id.NewRunAttempt()
	o.queries.CreateRunAttempt(ctx, db.CreateRunAttemptParams{
		ID:            attemptID,
		RunID:         run.ID,
		AttemptNumber: attempt,
		StartedAt:     dbutil.Timestamptz(now),
	})

	// Build params
	var methodConfig map[string]any
	if len(proc.MethodConfig) > 0 {
		json.Unmarshal(proc.MethodConfig, &methodConfig)
	}

	params := StartParams{
		RunID:        run.ID,
		WorkspaceID:  run.WorkspaceID,
		MethodConfig: methodConfig,
	}

	// Execute
	log.Info("executor: dispatching", "method", proc.ExecutionMethod, "attempt", attempt)
	startResult, err := method.Start(ctx, params)
	if err != nil {
		slog.Error("executor: start failed", "run", run.ID, "method", proc.ExecutionMethod, "error", err)
		o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:            run.ID,
			State:         string(runstate.Failed),
			FinishedAt:    dbutil.Timestamptz(time.Now().UTC()),
			WaitingReason: strPtr(fmt.Sprintf("start failed: %s", err.Error())),
		})
		return
	}
	handle := startResult.Handle
	if handle.MethodName == "" {
		handle = Handle{MethodName: proc.ExecutionMethod, RunID: run.ID, Data: make(map[string]any)}
	}
	o.running.Store(run.ID, handle)
	defer o.running.Delete(run.ID)
	if startResult.Result == nil {
		if err := o.saveRunExecutionHandle(ctx, run.ID, handle); err != nil {
			log.Error("executor: save async handle failed", "error", err)
			o.updateStateFailed(ctx, run.ID, -1)
			return
		}
		log.Info("executor: async execution accepted", "method", proc.ExecutionMethod)
		return
	}
	result := *startResult.Result

	finished := time.Now().UTC()
	durationMs := finished.Sub(now).Milliseconds()

	currentRun, err := o.queries.GetRun(ctx, db.GetRunParams{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
	})
	if err == nil && currentRun.State == string(runstate.KillRequested) {
		if method.SupportsKill() && !result.IsSuccess() {
			ec := int32(137)
			if result.ExitCode != nil {
				ec = int32(*result.ExitCode)
			}
			o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
				ID:         run.ID,
				State:      string(runstate.Killed),
				FinishedAt: dbutil.Timestamptz(finished),
				DurationMs: &durationMs,
				ExitCode:   &ec,
			})
			log.Info("executor: killed", "duration_ms", durationMs)
			o.handleTerminal(ctx, run, proc, runstate.Killed)
			return
		}
		if !method.SupportsKill() {
			log.Info("executor: kill requested but method has no kill support", "method", proc.ExecutionMethod)
		}
	}
	o.finalizeRunResult(ctx, run.ID, run.WorkspaceID, attemptID, attempt, now, proc, result)
	log.Info("executor: finished", "duration_ms", durationMs, "success", result.IsSuccess())
}

// handleTerminal handles post-completion work: fixed-delay chain + dependencies.
func (o *Orchestrator) handleTerminal(ctx context.Context, run db.Run, proc db.Process, state runstate.State) {
	// Fixed-delay chain
	if proc.ScheduleType == "fixed_delay" && runstate.ContinuesFixedDelayChain(state) {
		if proc.DelayDuration != nil {
			delay, err := time.ParseDuration(*proc.DelayDuration)
			if err == nil {
				nextAt := time.Now().UTC().Add(delay)
				o.queries.CreateRun(ctx, db.CreateRunParams{
					ID:          id.NewRun(),
					WorkspaceID: proc.WorkspaceID,
					ProcessID:   proc.ID,
					ScheduledAt: dbutil.Timestamptz(nextAt),
					State:       string(runstate.Pending),
					Origin:      "fixed_delay",
					MaxAttempts: proc.MaxAttempts,
					ActorType:   strPtr("system"),
					Tags:        proc.Tags,
				})
				slog.Info("executor: fixed-delay chain created", "process", proc.Name, "next_at", nextAt)
			}
		}
	}

	// Dependency callback
	if o.onRunTerminal != nil {
		o.onRunTerminal(ctx, run, proc)
	}
}

func (o *Orchestrator) updateState(ctx context.Context, runID string, state runstate.State) {
	o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
		ID:    runID,
		State: string(state),
	})
}

func (o *Orchestrator) updateStateFailed(ctx context.Context, runID string, exitCode int) {
	now := time.Now().UTC()
	ec := int32(exitCode)
	o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
		ID:         runID,
		State:      string(runstate.Failed),
		FinishedAt: dbutil.Timestamptz(now),
		ExitCode:   &ec,
	})
}

func calculateBackoff(from time.Time, attempt int32, backoffs []time.Duration) time.Time {
	if len(backoffs) == 0 {
		return from.Add(time.Minute)
	}
	idx := int(attempt) - 1
	if idx >= len(backoffs) {
		idx = len(backoffs) - 1
	}
	return from.Add(backoffs[idx])
}

func parseBackoffList(s *string) []time.Duration {
	if s == nil {
		return nil
	}
	var result []time.Duration
	for _, part := range strings.Split(*s, ",") {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err == nil {
			result = append(result, d)
		}
	}
	return result
}

func strPtr(s string) *string { return &s }
