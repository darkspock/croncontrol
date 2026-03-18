// Package logging provides a backend abstraction for operational data storage.
//
// Run output, heartbeats, and job attempt logs can be stored in different
// backends (PostgreSQL, file, OpenSearch) while metadata always stays in
// the main PostgreSQL database.
package logging

import (
	"context"
	"time"
)

// OutputChunk represents a chunk of run output (stdout or stderr).
type OutputChunk struct {
	Stream    string    `json:"stream"`    // "stdout" or "stderr"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Heartbeat represents a heartbeat event from a running process.
type Heartbeat struct {
	Total     int       `json:"total"`
	Current   int       `json:"current"`
	Progress  int       `json:"progress"` // percentage
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// JobAttemptLog represents a job attempt record for logging purposes.
type JobAttemptLog struct {
	AttemptNumber  int               `json:"attempt_number"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty"`
	DurationMs     int64             `json:"duration_ms"`
	ResponseCode   *int              `json:"response_code,omitempty"`
	ResponseBody   string            `json:"response_body,omitempty"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	WorkerID       string            `json:"worker_id,omitempty"`
}

// Backend is the interface for operational data storage backends.
// Implementations must be safe for concurrent use.
type Backend interface {
	// WriteRunOutput appends output for a run.
	WriteRunOutput(ctx context.Context, runID, stream, content string) error

	// WriteHeartbeat records a heartbeat event.
	WriteHeartbeat(ctx context.Context, runID string, hb Heartbeat) error

	// WriteJobAttempt records a job attempt.
	WriteJobAttempt(ctx context.Context, jobID string, attempt JobAttemptLog) error

	// QueryRunOutput retrieves output for a run.
	QueryRunOutput(ctx context.Context, runID, stream string) ([]OutputChunk, error)

	// QueryHeartbeats retrieves heartbeat events for a run.
	QueryHeartbeats(ctx context.Context, runID string) ([]Heartbeat, error)

	// QueryJobAttempts retrieves attempt logs for a job.
	QueryJobAttempts(ctx context.Context, jobID string) ([]JobAttemptLog, error)

	// Close releases any resources held by the backend.
	Close() error
}
