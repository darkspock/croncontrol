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
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/jobstate"
)

// Task represents a unit of work dispatched to a worker.
type Task struct {
	ID              string         `json:"id"`               // run or job ID
	Type            string         `json:"type"`             // "run" or "job"
	AttemptID       string         `json:"attempt_id,omitempty"`
	WorkspaceID     string         `json:"workspace_id"`
	ExecutionMethod string         `json:"execution_method"` // http, ssh, ssm, k8s
	MethodConfig    map[string]any `json:"method_config"`
	Environment     map[string]any `json:"environment,omitempty"`
	APIBaseURL      string         `json:"api_base_url"`
}

// TaskResult is reported back by the worker after execution.
type TaskResult struct {
	TaskID       string `json:"task_id"`
	AttemptID    string `json:"attempt_id,omitempty"`
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

	// 2. Label matching → least-loaded compatible worker
	if selectedWorker == nil {
		workers, err := d.queries.ListOnlineWorkers(ctx, db.ListOnlineWorkersParams{
			WorkspaceID: workspaceID,
			Limit:       100,
		})
		if err != nil || len(workers) == 0 {
			return "", nil // no worker available
		}

		selectedWorker = d.selectWorker(ctx, workers, workerLabels)
		if selectedWorker == nil {
			return "", nil
		}
	}

	// Check worker max_concurrency before dispatching
	runningCount, err := d.countActiveByWorker(ctx, selectedWorker.ID)
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

func (d *Dispatcher) selectWorker(ctx context.Context, workers []db.Worker, workerLabels json.RawMessage) *db.Worker {
	var selected *db.Worker
	var bestLoad int64 = 1<<62 - 1

	for i := range workers {
		w := &workers[i]
		if !matchesWorkerLabels(w.Labels, workerLabels) {
			continue
		}

		load, err := d.countActiveByWorker(ctx, w.ID)
		if err != nil {
			slog.Error("worker: count active", "worker", w.ID, "error", err)
			continue
		}
		if load >= int64(w.MaxConcurrency) {
			continue
		}
		if selected == nil || load < bestLoad {
			selected = w
			bestLoad = load
		}
	}

	return selected
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
		// Determine terminal state from the actual result, not just DB state.
		// If the task completed/failed before the kill took effect, honor the real outcome.
		state := "completed"
		row := d.pool.QueryRow(ctx, "SELECT state FROM runs WHERE id = $1", taskID)
		var currentState string
		row.Scan(&currentState)
		if !isSuccess {
			state = "failed"
		}
		// Only mark as killed if the result itself indicates the kill took effect
		// (non-success AND the DB state was kill_requested)
		if currentState == "kill_requested" && !isSuccess {
			state = "killed"
		}
		err := d.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:         taskID,
			State:      state,
			FinishedAt: timestamptzNow(),
			DurationMs: &result.DurationMs,
			ExitCode:   exitCode,
		})
		if err == nil && state == "killed" {
			return nil
		}
		return err
	}

	if len(taskID) > 4 && taskID[:4] == "job_" {
		row := d.pool.QueryRow(ctx, "SELECT workspace_id, state FROM jobs WHERE id = $1", taskID)
		var workspaceID, currentState string
		if err := row.Scan(&workspaceID, &currentState); err != nil {
			return err
		}

		job, err := d.queries.GetJobWithQueue(ctx, db.GetJobWithQueueParams{
			ID:          taskID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return err
		}

		if result.AttemptID != "" {
			if err := d.finishJobAttempt(ctx, job, result); err != nil {
				return err
			}
		}

		durMs := result.DurationMs
		// Only mark as killed if kill was requested AND the task did not complete successfully.
		// If the task finished before the kill took effect, honor the real outcome.
		if currentState == string(jobstate.KillRequested) && !isSuccess {
			return d.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
				ID:         taskID,
				State:      string(jobstate.Killed),
				DurationMs: &durMs,
			})
		}

		if isSuccess {
			return d.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
				ID:         taskID,
				State:      string(jobstate.Completed),
				DurationMs: &durMs,
			})
		}

		maxAttempts := job.QueueMaxAttempts
		if job.MaxAttempts != nil {
			maxAttempts = *job.MaxAttempts
		}
		if job.Attempt < maxAttempts {
			nextAttempt := job.Attempt + 1
			nextAt := calculateBackoff(time.Now().UTC(), nextAttempt, parseBackoffList(job.QueueRetryBackoff, job.RetryBackoff))
			return d.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
				ID:            taskID,
				State:         string(jobstate.Retrying),
				Attempt:       &nextAttempt,
				NextAttemptAt: dbutil.Timestamptz(nextAt),
				DurationMs:    &durMs,
			})
		}

		return d.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:         taskID,
			State:      string(jobstate.Failed),
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

func (d *Dispatcher) countActiveByWorker(ctx context.Context, workerID string) (int64, error) {
	// Count both running and kill_requested tasks — kill_requested tasks still consume
	// worker capacity until the kill is confirmed and the process exits.
	row := d.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM runs WHERE worker_id = $1 AND state IN ('running', 'kill_requested')) +
			(SELECT count(*) FROM jobs WHERE worker_id = $1 AND state IN ('running', 'kill_requested'))
	`, workerID)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (d *Dispatcher) finishJobAttempt(ctx context.Context, job db.GetJobWithQueueRow, result TaskResult) error {
	var responseCode *int32
	if result.ResponseCode != nil {
		rc := int32(*result.ResponseCode)
		responseCode = &rc
	}

	var errMsg *string
	if result.Error != "" {
		errMsg = &result.Error
	}

	respBody := result.ResponseBody
	truncated := false
	originalSize := int64(len(respBody))
	maxSize := int(job.MaxResponseSize)
	if maxSize > 0 && len(respBody) > maxSize {
		respBody = respBody[:maxSize]
		truncated = true
	}

	return d.queries.FinishJobAttempt(ctx, db.FinishJobAttemptParams{
		ID:           result.AttemptID,
		FinishedAt:   dbutil.Timestamptz(time.Now().UTC()),
		DurationMs:   &result.DurationMs,
		ResponseCode: responseCode,
		Truncated:    truncated,
		ResponseBody: &respBody,
		OriginalSize: &originalSize,
		ErrorMessage: errMsg,
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

func parseBackoffList(defaultBackoff string, override *string) []time.Duration {
	backoffStr := defaultBackoff
	if override != nil {
		backoffStr = *override
	}

	var result []time.Duration
	for _, part := range strings.Split(backoffStr, ",") {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		result = append(result, d)
	}
	return result
}

func matchesWorkerLabels(workerLabels, requiredLabels json.RawMessage) bool {
	if len(requiredLabels) == 0 || string(requiredLabels) == "null" {
		return true
	}

	var requiredArray []string
	if err := json.Unmarshal(requiredLabels, &requiredArray); err == nil {
		var workerArray []string
		if err := json.Unmarshal(workerLabels, &workerArray); err != nil {
			return false
		}
		available := make(map[string]struct{}, len(workerArray))
		for _, label := range workerArray {
			available[label] = struct{}{}
		}
		for _, label := range requiredArray {
			if _, ok := available[label]; !ok {
				return false
			}
		}
		return true
	}

	var requiredMap map[string]any
	if err := json.Unmarshal(requiredLabels, &requiredMap); err == nil {
		var workerMap map[string]any
		if err := json.Unmarshal(workerLabels, &workerMap); err != nil {
			return false
		}
		for key, value := range requiredMap {
			if workerValue, ok := workerMap[key]; !ok || !equalJSONValue(workerValue, value) {
				return false
			}
		}
		return true
	}

	return false
}

func equalJSONValue(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
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
