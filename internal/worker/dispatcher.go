// Package worker implements the CronControl Worker runtime on the control plane side.
//
// The dispatcher routes runs/jobs to appropriate workers, manages the task queue
// for long-poll pickup, and processes worker heartbeats.
//
// Canonical rules (docs/product-specification.md):
// - A worker belongs to one workspace only.
// - Communication is outbound only (long polling).
// - Routing: explicit worker_id > label matching > least-loaded compatible.
// - waiting_for_worker when no compatible worker available.
// - Online after heartbeat within 60s, offline after 60s, unhealthy after 5 failures.
// - Back to online after 3 healthy checks.
// - Disabled workers stop receiving new assignments.
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
)

// Task represents a unit of work dispatched to a worker.
type Task struct {
	ID              string         `json:"id"`               // run or job ID
	Type            string         `json:"type"`             // "run" or "job"
	WorkspaceID     string         `json:"workspace_id"`
	ExecutionMethod string         `json:"execution_method"` // http, ssh, ssm, k8s
	MethodConfig    map[string]any `json:"method_config"`
	Environment     map[string]any `json:"environment,omitempty"`
	APIBaseURL      string         `json:"api_base_url"`
}

// TaskResult is reported back by the worker after execution.
type TaskResult struct {
	TaskID       string `json:"task_id"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	ResponseCode *int   `json:"response_code,omitempty"`
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

// Dispatcher manages task routing to workers and long-poll task queues.
type Dispatcher struct {
	pool    *pgxpool.Pool
	queries *db.Queries

	// Per-worker task channels for long-poll.
	// Key: worker ID, Value: channel of tasks.
	taskQueues sync.Map // map[string]chan Task

	stop chan struct{}
}

// NewDispatcher creates a new worker dispatcher.
func NewDispatcher(pool *pgxpool.Pool) *Dispatcher {
	return &Dispatcher{
		pool:    pool,
		queries: db.New(pool),
		stop:    make(chan struct{}),
	}
}

// Start begins the worker status monitoring loop.
func (d *Dispatcher) Start(ctx context.Context) {
	go d.statusLoop(ctx)
	slog.Info("worker dispatcher started")
}

// Stop signals the dispatcher to stop.
func (d *Dispatcher) Stop() {
	close(d.stop)
}

// Dispatch routes a task to an appropriate worker.
// Returns the selected worker ID, or empty string if no worker available.
func (d *Dispatcher) Dispatch(ctx context.Context, workspaceID string, task Task, workerID *string, workerLabels json.RawMessage) (string, error) {
	var selectedWorker *db.Worker

	// 1. Explicit worker_id
	if workerID != nil && *workerID != "" {
		w, err := d.queries.GetWorker(ctx, db.GetWorkerParams{
			ID:          *workerID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return "", err
		}
		if !w.Enabled || w.Status != "online" {
			return "", nil // worker not available
		}
		selectedWorker = &w
	}

	// 2. Label matching → least-loaded (simplified: just pick any online worker)
	if selectedWorker == nil {
		workers, err := d.queries.ListOnlineWorkers(ctx, db.ListOnlineWorkersParams{
			WorkspaceID: workspaceID,
			Limit:       1,
		})
		if err != nil || len(workers) == 0 {
			return "", nil // no worker available
		}
		selectedWorker = &workers[0]
	}

	// Check worker max_concurrency before dispatching
	runningCount, err := d.queries.CountRunningByWorker(ctx, selectedWorker.ID)
	if err != nil {
		slog.Error("worker: count running", "error", err)
		return "", nil
	}
	if runningCount >= int64(selectedWorker.MaxConcurrency) {
		slog.Warn("worker: at max concurrency",
			"worker", selectedWorker.Name,
			"running", runningCount,
			"max", selectedWorker.MaxConcurrency,
		)
		return "", nil // worker at capacity
	}

	// Enqueue task for the worker's long-poll channel
	ch := d.getOrCreateChannel(selectedWorker.ID)
	select {
	case ch <- task:
		slog.Info("worker: task dispatched",
			"worker", selectedWorker.Name,
			"task", task.ID,
			"method", task.ExecutionMethod,
		)
		return selectedWorker.ID, nil
	default:
		// Channel full — worker is at capacity
		slog.Warn("worker: task queue full", "worker", selectedWorker.ID)
		return "", nil
	}
}

// PollTask is called by the worker binary via long-poll.
// Blocks until a task is available or the context is cancelled.
func (d *Dispatcher) PollTask(ctx context.Context, workerID string) (*Task, error) {
	ch := d.getOrCreateChannel(workerID)

	select {
	case task := <-ch:
		return &task, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ProcessHeartbeat updates worker status based on heartbeat.
func (d *Dispatcher) ProcessHeartbeat(ctx context.Context, workerID string, capabilities []byte, version string) error {
	return d.queries.UpdateWorkerHeartbeat(ctx, db.UpdateWorkerHeartbeatParams{
		ID:           workerID,
		Capabilities: capabilities,
		Version:      &version,
	})
}

// ProcessResult handles a task result reported by a worker.
// Updates the run or job state based on the execution outcome.
func (d *Dispatcher) ProcessResult(ctx context.Context, result TaskResult) error {
	slog.Info("worker: result received",
		"task", result.TaskID,
		"exit_code", result.ExitCode,
		"duration_ms", result.DurationMs,
	)

	isSuccess := false
	if result.ExitCode != nil {
		isSuccess = *result.ExitCode == 0
	} else if result.ResponseCode != nil {
		isSuccess = *result.ResponseCode >= 200 && *result.ResponseCode < 300
	}
	isSuccess = isSuccess && result.Error == ""

	var exitCode *int32
	if result.ExitCode != nil {
		ec := int32(*result.ExitCode)
		exitCode = &ec
	}

	// Determine if this is a run or job based on ID prefix
	taskID := result.TaskID
	if len(taskID) > 4 && taskID[:4] == "run_" {
		state := "completed"
		if !isSuccess {
			state = "failed"
		}
		return d.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:         taskID,
			State:      state,
			FinishedAt: timestamptzNow(),
			DurationMs: &result.DurationMs,
			ExitCode:   exitCode,
		})
	}

	if len(taskID) > 4 && taskID[:4] == "job_" {
		state := "completed"
		if !isSuccess {
			state = "failed"
		}
		return d.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:         taskID,
			State:      state,
			DurationMs: &result.DurationMs,
		})
	}

	slog.Warn("worker: unknown task ID prefix", "task_id", taskID)
	return nil
}

func (d *Dispatcher) getOrCreateChannel(workerID string) chan Task {
	if ch, ok := d.taskQueues.Load(workerID); ok {
		return ch.(chan Task)
	}
	ch := make(chan Task, 10)
	actual, _ := d.taskQueues.LoadOrStore(workerID, ch)
	return actual.(chan Task)
}

// statusLoop periodically checks for stale workers and updates their status.
func (d *Dispatcher) statusLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.checkStaleWorkers(ctx)
		case <-d.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) checkStaleWorkers(ctx context.Context) {
	stale, err := d.queries.ListStaleWorkers(ctx)
	if err != nil {
		slog.Error("worker: list stale", "error", err)
		return
	}

	for _, w := range stale {
		// Increment failures
		d.queries.IncrementWorkerFailures(ctx, w.ID)

		newStatus := "offline"
		if w.ConsecutiveFailures+1 >= 5 {
			newStatus = "unhealthy"
		}

		d.queries.SetWorkerStatus(ctx, db.SetWorkerStatusParams{
			ID:     w.ID,
			Status: newStatus,
		})

		slog.Warn("worker: marked stale", "worker", w.Name, "status", newStatus)

		// Reassign pending/waiting_for_worker runs that were assigned to this worker
		// but not yet running (not pinned to a specific worker)
		d.reassignWorkerRuns(ctx, w.ID)
	}
}

// reassignWorkerRuns moves waiting_for_worker runs back to pending so they
// can be picked up by another worker or the direct executor.
func (d *Dispatcher) reassignWorkerRuns(ctx context.Context, workerID string) {
	// Reset runs that were waiting for this worker back to pending
	rows, err := d.pool.Exec(ctx,
		`UPDATE runs SET state = 'pending', worker_id = NULL, waiting_reason = NULL, updated_at = now()
		 WHERE worker_id = $1 AND state = 'waiting_for_worker'`, workerID)
	if err != nil {
		slog.Error("worker: reassign runs", "error", err, "worker_id", workerID)
		return
	}
	if rows.RowsAffected() > 0 {
		slog.Info("worker: reassigned waiting runs", "worker_id", workerID, "count", rows.RowsAffected())
	}
}

func timestamptzNow() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}
