// Package file implements the logging backend using JSON-lines files.
//
// Each data type (output, heartbeats, attempts) writes to separate
// directories organized by date. Suitable for high-volume environments
// where PostgreSQL storage is too expensive for operational logs.
package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/croncontrol/croncontrol/internal/logging"
)

// Compile-time contract.
var _ logging.Backend = (*Backend)(nil)

// Backend stores operational data as JSON-lines files on disk.
type Backend struct {
	baseDir string
	mu      sync.Mutex
}

// New creates a file logging backend rooted at baseDir.
func New(baseDir string) (*Backend, error) {
	for _, sub := range []string{"output", "heartbeats", "attempts"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return &Backend{baseDir: baseDir}, nil
}

type outputEntry struct {
	RunID     string    `json:"run_id"`
	Stream    string    `json:"stream"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type heartbeatEntry struct {
	RunID string            `json:"run_id"`
	HB    logging.Heartbeat `json:"heartbeat"`
}

type attemptEntry struct {
	JobID   string               `json:"job_id"`
	Attempt logging.JobAttemptLog `json:"attempt"`
}

func (b *Backend) WriteRunOutput(_ context.Context, runID, stream, content string) error {
	entry := outputEntry{
		RunID:     runID,
		Stream:    stream,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return b.appendJSON("output", runID, entry)
}

func (b *Backend) WriteHeartbeat(_ context.Context, runID string, hb logging.Heartbeat) error {
	entry := heartbeatEntry{RunID: runID, HB: hb}
	return b.appendJSON("heartbeats", runID, entry)
}

func (b *Backend) WriteJobAttempt(_ context.Context, jobID string, attempt logging.JobAttemptLog) error {
	entry := attemptEntry{JobID: jobID, Attempt: attempt}
	return b.appendJSON("attempts", jobID, entry)
}

func (b *Backend) QueryRunOutput(_ context.Context, runID, stream string) ([]logging.OutputChunk, error) {
	path := b.filePath("output", runID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var chunks []logging.OutputChunk
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer
	for scanner.Scan() {
		var e outputEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if stream != "" && e.Stream != stream {
			continue
		}
		chunks = append(chunks, logging.OutputChunk{
			Stream:    e.Stream,
			Content:   e.Content,
			CreatedAt: e.CreatedAt,
		})
	}
	return chunks, scanner.Err()
}

func (b *Backend) QueryHeartbeats(_ context.Context, runID string) ([]logging.Heartbeat, error) {
	path := b.filePath("heartbeats", runID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var hbs []logging.Heartbeat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e heartbeatEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		hbs = append(hbs, e.HB)
	}
	return hbs, scanner.Err()
}

func (b *Backend) QueryJobAttempts(_ context.Context, jobID string) ([]logging.JobAttemptLog, error) {
	path := b.filePath("attempts", jobID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var attempts []logging.JobAttemptLog
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e attemptEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		attempts = append(attempts, e.Attempt)
	}
	return attempts, scanner.Err()
}

func (b *Backend) Close() error { return nil }

func (b *Backend) appendJSON(subdir, id string, entry any) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := b.filePath(subdir, id)

	b.mu.Lock()
	defer b.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (b *Backend) filePath(subdir, id string) string {
	date := time.Now().Format("2006-01-02")
	return filepath.Join(b.baseDir, subdir, fmt.Sprintf("%s_%s.jsonl", date, id))
}
