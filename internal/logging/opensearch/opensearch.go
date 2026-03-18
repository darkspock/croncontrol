// Package opensearch implements the logging backend using OpenSearch.
//
// Operational data is stored in date-partitioned indices for efficient
// querying and automatic rotation. Suitable for large-scale deployments
// that need full-text search across run output and job attempts.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/croncontrol/croncontrol/internal/logging"
)

// Compile-time contract.
var _ logging.Backend = (*Backend)(nil)

// Config holds OpenSearch connection settings.
type Config struct {
	URL      string // e.g., "http://localhost:9200"
	Username string
	Password string
}

// Backend stores operational data in OpenSearch indices.
type Backend struct {
	config Config
	client *http.Client
}

// New creates an OpenSearch logging backend.
func New(cfg Config) *Backend {
	return &Backend{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type outputDoc struct {
	RunID     string    `json:"run_id"`
	Stream    string    `json:"stream"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type heartbeatDoc struct {
	RunID     string    `json:"run_id"`
	Total     int       `json:"total"`
	Current   int       `json:"current"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type attemptDoc struct {
	JobID          string               `json:"job_id"`
	AttemptNumber  int                  `json:"attempt_number"`
	StartedAt      time.Time            `json:"started_at"`
	FinishedAt     *time.Time           `json:"finished_at,omitempty"`
	DurationMs     int64                `json:"duration_ms"`
	ResponseCode   *int                 `json:"response_code,omitempty"`
	ResponseBody   string               `json:"response_body,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	WorkerID       string               `json:"worker_id,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

func (b *Backend) WriteRunOutput(ctx context.Context, runID, stream, content string) error {
	doc := outputDoc{
		RunID:     runID,
		Stream:    stream,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return b.index(ctx, "croncontrol-output", doc)
}

func (b *Backend) WriteHeartbeat(ctx context.Context, runID string, hb logging.Heartbeat) error {
	doc := heartbeatDoc{
		RunID:     runID,
		Total:     hb.Total,
		Current:   hb.Current,
		Progress:  hb.Progress,
		Message:   hb.Message,
		CreatedAt: hb.CreatedAt,
	}
	return b.index(ctx, "croncontrol-heartbeats", doc)
}

func (b *Backend) WriteJobAttempt(ctx context.Context, jobID string, attempt logging.JobAttemptLog) error {
	doc := attemptDoc{
		JobID:         jobID,
		AttemptNumber: attempt.AttemptNumber,
		StartedAt:     attempt.StartedAt,
		FinishedAt:    attempt.FinishedAt,
		DurationMs:    attempt.DurationMs,
		ResponseCode:  attempt.ResponseCode,
		ResponseBody:  attempt.ResponseBody,
		ErrorMessage:  attempt.ErrorMessage,
		WorkerID:      attempt.WorkerID,
		CreatedAt:     time.Now(),
	}
	return b.index(ctx, "croncontrol-attempts", doc)
}

func (b *Backend) QueryRunOutput(ctx context.Context, runID, stream string) ([]logging.OutputChunk, error) {
	filters := []map[string]any{
		{"term": map[string]any{"run_id": runID}},
	}
	if stream != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"stream": stream}})
	}

	results, err := b.search(ctx, "croncontrol-output", filters, "created_at", 10000)
	if err != nil {
		return nil, err
	}

	var chunks []logging.OutputChunk
	for _, hit := range results {
		var doc outputDoc
		if err := json.Unmarshal(hit, &doc); err != nil {
			continue
		}
		chunks = append(chunks, logging.OutputChunk{
			Stream:    doc.Stream,
			Content:   doc.Content,
			CreatedAt: doc.CreatedAt,
		})
	}
	return chunks, nil
}

func (b *Backend) QueryHeartbeats(ctx context.Context, runID string) ([]logging.Heartbeat, error) {
	filters := []map[string]any{
		{"term": map[string]any{"run_id": runID}},
	}

	results, err := b.search(ctx, "croncontrol-heartbeats", filters, "created_at", 10000)
	if err != nil {
		return nil, err
	}

	var hbs []logging.Heartbeat
	for _, hit := range results {
		var doc heartbeatDoc
		if err := json.Unmarshal(hit, &doc); err != nil {
			continue
		}
		hbs = append(hbs, logging.Heartbeat{
			Total:     doc.Total,
			Current:   doc.Current,
			Progress:  doc.Progress,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		})
	}
	return hbs, nil
}

func (b *Backend) QueryJobAttempts(ctx context.Context, jobID string) ([]logging.JobAttemptLog, error) {
	filters := []map[string]any{
		{"term": map[string]any{"job_id": jobID}},
	}

	results, err := b.search(ctx, "croncontrol-attempts", filters, "attempt_number", 100)
	if err != nil {
		return nil, err
	}

	var attempts []logging.JobAttemptLog
	for _, hit := range results {
		var doc attemptDoc
		if err := json.Unmarshal(hit, &doc); err != nil {
			continue
		}
		attempts = append(attempts, logging.JobAttemptLog{
			AttemptNumber: doc.AttemptNumber,
			StartedAt:     doc.StartedAt,
			FinishedAt:    doc.FinishedAt,
			DurationMs:    doc.DurationMs,
			ResponseCode:  doc.ResponseCode,
			ResponseBody:  doc.ResponseBody,
			ErrorMessage:  doc.ErrorMessage,
			WorkerID:      doc.WorkerID,
		})
	}
	return attempts, nil
}

func (b *Backend) Close() error { return nil }

// index writes a document to a date-partitioned index.
func (b *Backend) index(ctx context.Context, indexPrefix string, doc any) error {
	index := fmt.Sprintf("%s-%s", indexPrefix, time.Now().Format("2006.01"))

	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/_doc", b.config.URL, index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.config.Username != "" {
		req.SetBasicAuth(b.config.Username, b.config.Password)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch index: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch index returned %d", resp.StatusCode)
	}
	return nil
}

// search queries documents from OpenSearch.
func (b *Backend) search(ctx context.Context, indexPrefix string, filters []map[string]any, sortField string, size int) ([]json.RawMessage, error) {
	// Search across all monthly indices
	index := fmt.Sprintf("%s-*", indexPrefix)

	query := map[string]any{
		"size": size,
		"sort": []map[string]any{{sortField: "asc"}},
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_search", b.config.URL, index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.config.Username != "" {
		req.SetBasicAuth(b.config.Username, b.config.Password)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensearch search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensearch search returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("opensearch decode: %w", err)
	}

	var docs []json.RawMessage
	for _, hit := range result.Hits.Hits {
		docs = append(docs, hit.Source)
	}
	return docs, nil
}
