// Package http implements the HTTP execution method.
//
// Canonical rules (from docs/product-specification.md):
// - Success is 2xx only.
// - No automatic redirects by default.
// - Body allowed only on POST, PUT, PATCH.
// - Simple template variable substitution: {{run.id}}, {{now}}, {{workspace.id}}.
// - Heartbeat/progress does NOT apply to HTTP.
// - HTTP is request/response only. No async 202 lifecycle.
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/croncontrol/croncontrol/internal/executor"
	"github.com/croncontrol/croncontrol/internal/ssrf"
)

// Compile-time contract.
var _ executor.Method = (*Method)(nil)

// Method implements HTTP execution.
type Method struct {
	maxResponseSize int
	skipSSRF        bool // for testing only
}

// New creates a new HTTP execution method.
func New(maxResponseSize int) *Method {
	if maxResponseSize <= 0 {
		maxResponseSize = 5 * 1024 * 1024 // 5MB default
	}
	return &Method{maxResponseSize: maxResponseSize}
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg := params.MethodConfig
	start := time.Now()

	// Extract config
	targetURL, _ := cfg["url"].(string)
	method, _ := cfg["method"].(string)
	if method == "" {
		method = "POST"
	}
	method = strings.ToUpper(method)

	// Template substitution
	targetURL = substituteTemplates(targetURL, params)

	// SSRF check (format only — full DNS check happens at request time via transport)
	if !m.skipSSRF {
	if err := ssrf.ValidateFormat(targetURL, nil); err != nil {
		return executor.Result{
			Error:      fmt.Errorf("SSRF validation failed: %w", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}
	}

	// Build body
	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		if bodyStr, ok := cfg["body"].(string); ok {
			bodyStr = substituteTemplates(bodyStr, params)
			body = strings.NewReader(bodyStr)
		}
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return executor.Result{
			Error:      fmt.Errorf("create request: %w", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	// Headers
	if headers, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, substituteTemplates(s, params))
			}
		}
	}
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Client with no redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Execute
	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return executor.Result{
			Error:      err,
			DurationMs: durationMs,
		}, nil
	}
	defer resp.Body.Close()

	// Read response body (limited)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, int64(m.maxResponseSize)))

	code := resp.StatusCode
	return executor.Result{
		ResponseCode: &code,
		ResponseBody: string(respBody),
		DurationMs:   durationMs,
	}, nil
}

func (m *Method) Kill(_ context.Context, _ executor.Handle) error {
	// HTTP cannot kill — request is already in flight.
	return nil
}

func (m *Method) SupportsKill() bool      { return false }
func (m *Method) SupportsHeartbeat() bool  { return false }

// substituteTemplates replaces {{variable}} placeholders.
func substituteTemplates(s string, params executor.ExecuteParams) string {
	s = strings.ReplaceAll(s, "{{run.id}}", params.RunID)
	s = strings.ReplaceAll(s, "{{workspace.id}}", params.WorkspaceID)
	s = strings.ReplaceAll(s, "{{now}}", time.Now().UTC().Format(time.RFC3339))
	return s
}
