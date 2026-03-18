// Package database implements the logging backend using PostgreSQL.
//
// This is the default backend. It writes operational data to the same
// PostgreSQL instance used by the main application. Suitable for small
// to medium deployments.
package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncontrol/croncontrol/internal/logging"
)

// Compile-time contract.
var _ logging.Backend = (*Backend)(nil)

// Backend stores operational data in PostgreSQL tables.
type Backend struct {
	pool *pgxpool.Pool
}

// New creates a database logging backend.
func New(pool *pgxpool.Pool) *Backend {
	return &Backend{pool: pool}
}

func (b *Backend) WriteRunOutput(ctx context.Context, runID, stream, content string) error {
	_, err := b.pool.Exec(ctx,
		`INSERT INTO run_output (run_id, stream, content, created_at) VALUES ($1, $2, $3, now())`,
		runID, stream, content,
	)
	return err
}

func (b *Backend) WriteHeartbeat(ctx context.Context, runID string, hb logging.Heartbeat) error {
	_, err := b.pool.Exec(ctx,
		`INSERT INTO heartbeats (run_id, total, current, progress, message, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		runID, hb.Total, hb.Current, hb.Progress, hb.Message, hb.CreatedAt,
	)
	return err
}

func (b *Backend) WriteJobAttempt(ctx context.Context, jobID string, attempt logging.JobAttemptLog) error {
	_, err := b.pool.Exec(ctx,
		`INSERT INTO job_attempts (id, job_id, attempt_number, started_at, finished_at, duration_ms,
		 response_code, response_body, error_message, worker_id, created_at)
		 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, $8, $9, now())`,
		jobID, attempt.AttemptNumber, attempt.StartedAt, attempt.FinishedAt,
		attempt.DurationMs, attempt.ResponseCode, attempt.ResponseBody,
		attempt.ErrorMessage, attempt.WorkerID,
	)
	return err
}

func (b *Backend) QueryRunOutput(ctx context.Context, runID, stream string) ([]logging.OutputChunk, error) {
	rows, err := b.pool.Query(ctx,
		`SELECT stream, content, created_at FROM run_output
		 WHERE run_id = $1 AND ($2 = '' OR stream = $2)
		 ORDER BY created_at`,
		runID, stream,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []logging.OutputChunk
	for rows.Next() {
		var c logging.OutputChunk
		if err := rows.Scan(&c.Stream, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (b *Backend) QueryHeartbeats(ctx context.Context, runID string) ([]logging.Heartbeat, error) {
	rows, err := b.pool.Query(ctx,
		`SELECT total, current, progress, message, created_at FROM heartbeats
		 WHERE run_id = $1 ORDER BY created_at`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hbs []logging.Heartbeat
	for rows.Next() {
		var h logging.Heartbeat
		if err := rows.Scan(&h.Total, &h.Current, &h.Progress, &h.Message, &h.CreatedAt); err != nil {
			return nil, err
		}
		hbs = append(hbs, h)
	}
	return hbs, rows.Err()
}

func (b *Backend) QueryJobAttempts(ctx context.Context, jobID string) ([]logging.JobAttemptLog, error) {
	rows, err := b.pool.Query(ctx,
		`SELECT attempt_number, started_at, finished_at, duration_ms,
		        response_code, response_body, error_message, COALESCE(worker_id, '')
		 FROM job_attempts WHERE job_id = $1 ORDER BY attempt_number`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []logging.JobAttemptLog
	for rows.Next() {
		var a logging.JobAttemptLog
		if err := rows.Scan(&a.AttemptNumber, &a.StartedAt, &a.FinishedAt, &a.DurationMs,
			&a.ResponseCode, &a.ResponseBody, &a.ErrorMessage, &a.WorkerID); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (b *Backend) Close() error {
	// Pool is shared with the main application; don't close it here.
	return nil
}

// Ensure the created_at fields default properly.
func init() {
	_ = time.Now // reference time package
}
