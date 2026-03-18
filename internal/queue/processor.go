// Package queue implements the durable job queue processor.
//
// Claims pending/retrying jobs, dispatches to execution methods,
// records attempts, handles retry with backoff, and manages expiration.
package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/executor"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/jobstate"
	"github.com/croncontrol/croncontrol/internal/logging"
)

// JobLogBackend writes job attempt data to the configured logging backend.
type JobLogBackend interface {
	WriteJobAttempt(ctx context.Context, jobID string, attempt logging.JobAttemptLog) error
}

// Processor claims and dispatches queued jobs.
type Processor struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	registry   *executor.Registry
	interval   time.Duration
	logBackend JobLogBackend
	stop       chan struct{}
}

// NewProcessor creates a new queue processor.
func NewProcessor(pool *pgxpool.Pool, registry *executor.Registry, interval time.Duration) *Processor {
	return &Processor{
		pool:     pool,
		queries:  db.New(pool),
		registry: registry,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// SetLogBackend sets the logging backend for writing job attempts.
func (p *Processor) SetLogBackend(lb JobLogBackend) {
	p.logBackend = lb
}

// Start begins the processing loop.
func (p *Processor) Start(ctx context.Context) {
	go p.loop(ctx)
	go p.expirationLoop(ctx)
	slog.Info("queue processor started", "interval", p.interval)
}

// Stop signals the processor to stop.
func (p *Processor) Stop() {
	close(p.stop)
	slog.Info("queue processor stopped")
}

func (p *Processor) loop(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.tick(ctx)
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) tick(ctx context.Context) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		slog.Error("queue: begin tx", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	qtx := p.queries.WithTx(tx)
	jobs, err := qtx.ClaimPendingJobs(ctx, 10)
	if err != nil {
		slog.Error("queue: claim jobs", "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("queue: commit claim", "error", err)
		return
	}

	for _, job := range jobs {
		go p.dispatch(ctx, job)
	}
}

func (p *Processor) dispatch(ctx context.Context, job db.ClaimPendingJobsRow) {
	log := slog.With("job_id", job.ID, "queue_id", job.QueueID)

	// Check queue concurrency
	running, err := p.queries.CountRunningByQueue(ctx, job.QueueID)
	if err != nil {
		log.Error("queue: count running", "error", err)
		return
	}
	if running >= int64(job.QueueConcurrency) {
		// Release — will be picked up next tick
		return
	}

	// Get execution method
	method, ok := p.registry.Get(job.ExecutionMethod)
	if !ok {
		log.Error("queue: unknown method", "method", job.ExecutionMethod)
		p.failJob(ctx, job.ID, 0)
		return
	}

	// Transition to running
	attempt := job.Attempt + 1
	err = p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
		ID:    job.ID,
		State: string(jobstate.Running),
		Attempt: &attempt,
	})
	if err != nil {
		log.Error("queue: update running", "error", err)
		return
	}

	// Snapshot config on first attempt
	if attempt == 1 {
		snapshot, _ := json.Marshal(map[string]any{
			"method_config":    job.QueueMethodConfig,
			"execution_method": job.ExecutionMethod,
		})
		p.queries.SnapshotJobConfig(ctx, db.SnapshotJobConfigParams{
			ID:              job.ID,
			EffectiveConfig: snapshot,
		})
	}

	// Create attempt record
	start := time.Now().UTC()
	var methodConfig map[string]any
	if len(job.QueueMethodConfig) > 0 {
		json.Unmarshal(job.QueueMethodConfig, &methodConfig)
	}

	requestJSON, _ := json.Marshal(methodConfig)
	attemptID := id.NewJobAttempt()
	p.queries.CreateJobAttempt(ctx, db.CreateJobAttemptParams{
		ID:            attemptID,
		JobID:         job.ID,
		AttemptNumber: attempt,
		StartedAt:     dbutil.Timestamptz(start),
		Request:       requestJSON,
	})

	// Execute
	log.Info("queue: dispatching", "method", job.ExecutionMethod, "attempt", attempt)
	params := executor.ExecuteParams{
		RunID:        job.ID,
		WorkspaceID:  job.WorkspaceID,
		MethodConfig: methodConfig,
	}

	result, _ := method.Execute(ctx, params)
	finished := time.Now().UTC()
	durationMs := finished.Sub(start).Milliseconds()

	// Record attempt result
	var responseCode *int32
	if result.ResponseCode != nil {
		rc := int32(*result.ResponseCode)
		responseCode = &rc
	}

	var errMsg *string
	if result.Error != nil {
		s := result.Error.Error()
		errMsg = &s
	}

	// Truncate response body
	respBody := result.ResponseBody
	truncated := false
	maxSize := int(job.MaxResponseSize)
	if len(respBody) > maxSize {
		respBody = respBody[:maxSize]
		truncated = true
	}

	p.queries.FinishJobAttempt(ctx, db.FinishJobAttemptParams{
		ID:           attemptID,
		FinishedAt:   dbutil.Timestamptz(finished),
		DurationMs:   &durationMs,
		ResponseCode: responseCode,
		ResponseBody: &respBody,
		Truncated:    truncated,
		ErrorMessage: errMsg,
	})

	// Write to logging backend
	if p.logBackend != nil {
		var rc *int
		if responseCode != nil {
			v := int(*responseCode)
			rc = &v
		}
		em := ""
		if errMsg != nil {
			em = *errMsg
		}
		fin := finished
		p.logBackend.WriteJobAttempt(ctx, job.ID, logging.JobAttemptLog{
			AttemptNumber: int(attempt),
			StartedAt:     start,
			FinishedAt:    &fin,
			DurationMs:    durationMs,
			ResponseCode:  rc,
			ResponseBody:  respBody,
			ErrorMessage:  em,
		})
	}

	// Determine max attempts for this job
	maxAttempts := job.QueueMaxAttempts
	if job.MaxAttempts != nil {
		maxAttempts = *job.MaxAttempts
	}

	if result.IsSuccess() {
		// Success
		durMs := durationMs
		p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:         job.ID,
			State:      string(jobstate.Completed),
			DurationMs: &durMs,
		})
		log.Info("queue: completed", "duration_ms", durationMs)
		return
	}

	// Failure — check retries
	if attempt < maxAttempts {
		nextAt := calculateBackoff(finished, attempt, getBackoffList(job))
		p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:            job.ID,
			State:         string(jobstate.Retrying),
			Attempt:       &attempt,
			NextAttemptAt: dbutil.Timestamptz(nextAt),
		})
		log.Info("queue: retrying", "attempt", attempt, "max", maxAttempts, "next_at", nextAt)
		return
	}

	// Final failure
	p.failJob(ctx, job.ID, durationMs)
	log.Info("queue: failed", "attempts", attempt)
}

func (p *Processor) failJob(ctx context.Context, jobID string, durationMs int64) {
	p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
		ID:         jobID,
		State:      string(jobstate.Failed),
		DurationMs: &durationMs,
	})
}

// expirationLoop periodically cancels expired pending jobs.
func (p *Processor) expirationLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			expired, err := p.queries.ListExpiredPendingJobs(ctx, 100)
			if err != nil {
				continue
			}
			for _, job := range expired {
				reason := "expired"
				p.queries.CancelJob(ctx, db.CancelJobParams{
					ID:           job.ID,
					CancelReason: &reason,
				})
				slog.Info("queue: job expired", "job_id", job.ID)
			}
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// calculateBackoff returns the next retry time based on the backoff list.
func calculateBackoff(from time.Time, attempt int32, backoffs []time.Duration) time.Time {
	if len(backoffs) == 0 {
		return from.Add(time.Minute) // default 1m
	}
	idx := int(attempt) - 1
	if idx >= len(backoffs) {
		idx = len(backoffs) - 1
	}
	return from.Add(backoffs[idx])
}

// getBackoffList parses the comma-separated backoff string.
func getBackoffList(job db.ClaimPendingJobsRow) []time.Duration {
	backoffStr := job.QueueRetryBackoff
	if job.RetryBackoff != nil {
		backoffStr = *job.RetryBackoff
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
