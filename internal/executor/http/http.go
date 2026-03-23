// Package http implements the HTTP execution method.
//
// Canonical rules:
// - Success is 2xx only for sync.
// - No automatic redirects by default.
// - Body allowed only on POST, PUT, PATCH.
// - Simple template variable substitution: {{run.id}}, {{now}}, {{workspace.id}}.
// - Heartbeat/progress does NOT apply to HTTP.
// - HTTP supports explicit dispatch modes: sync, async_blind, async_tracked.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/croncontrol/croncontrol/internal/executor"
	"github.com/croncontrol/croncontrol/internal/ssrf"
)

var _ executor.Method = (*Method)(nil)
var _ executor.BlockingMethod = (*Method)(nil)

// Method implements HTTP execution.
type Method struct {
	maxResponseSize int
	skipSSRF        bool // for testing only
}

type trackedHandle struct {
	StatusURL          string         `json:"status_url,omitempty"`
	CancelURL          string         `json:"cancel_url,omitempty"`
	JobID              string         `json:"job_id,omitempty"`
	Headers            map[string]any `json:"headers,omitempty"`
	PollHeaders        map[string]any `json:"poll_headers,omitempty"`
	CancelHeaders      map[string]any `json:"cancel_headers,omitempty"`
	PollMethod         string         `json:"poll_method,omitempty"`
	CancelMethod       string         `json:"cancel_method,omitempty"`
	AcceptedStatusCode []int          `json:"accepted_status_codes,omitempty"`
	RunningStatusCode  []int          `json:"running_status_codes,omitempty"`
	SuccessStatusCode  []int          `json:"success_status_codes,omitempty"`
	StatusField        string         `json:"status_field,omitempty"`
	RunningValues      []string       `json:"running_values,omitempty"`
	SuccessValues      []string       `json:"success_values,omitempty"`
	FailedValues       []string       `json:"failed_values,omitempty"`
}

// New creates a new HTTP execution method.
func New(maxResponseSize int) *Method {
	if maxResponseSize <= 0 {
		maxResponseSize = 5 * 1024 * 1024
	}
	return &Method{maxResponseSize: maxResponseSize}
}

func (m *Method) Start(ctx context.Context, params executor.StartParams) (executor.StartResult, error) {
	mode := getString(params.MethodConfig, "dispatch_mode")
	if mode == "" {
		mode = "sync"
	}

	switch mode {
	case "sync":
		result, err := m.Execute(ctx, executor.ExecuteParams{
			RunID:        params.RunID,
			WorkspaceID:  params.WorkspaceID,
			MethodConfig: params.MethodConfig,
			Environment:  params.Environment,
			APIBaseURL:   params.APIBaseURL,
		})
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "http", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, err
	case "async_blind":
		result, err := m.dispatchAsync(ctx, params, false)
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "http", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, err
	case "async_tracked":
		result, handle, err := m.dispatchTracked(ctx, params)
		if result != nil {
			return executor.StartResult{
				Handle: executor.Handle{MethodName: "http", RunID: params.RunID, Data: map[string]any{}},
				Result: result,
			}, err
		}
		return executor.StartResult{
			Handle:     handle,
			AcceptedAt: time.Now().UTC(),
			Result:     nil,
		}, err
	default:
		result := executor.Result{Error: fmt.Errorf("http: unsupported dispatch_mode %q", mode)}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "http", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}
}

func (m *Method) Poll(ctx context.Context, handle executor.Handle, _ executor.PollCursor) (executor.PollResult, error) {
	data := trackedHandle{
		StatusURL:          getString(handle.Data, "status_url"),
		CancelURL:          getString(handle.Data, "cancel_url"),
		JobID:              getString(handle.Data, "job_id"),
		Headers:            getMap(handle.Data, "headers"),
		PollHeaders:        getMap(handle.Data, "poll_headers"),
		CancelHeaders:      getMap(handle.Data, "cancel_headers"),
		PollMethod:         getString(handle.Data, "poll_method"),
		CancelMethod:       getString(handle.Data, "cancel_method"),
		AcceptedStatusCode: getIntSlice(handle.Data, "accepted_status_codes"),
		RunningStatusCode:  getIntSlice(handle.Data, "running_status_codes"),
		SuccessStatusCode:  getIntSlice(handle.Data, "success_status_codes"),
		StatusField:        getString(handle.Data, "status_field"),
		RunningValues:      getStringSlice(handle.Data, "running_values"),
		SuccessValues:      getStringSlice(handle.Data, "success_values"),
		FailedValues:       getStringSlice(handle.Data, "failed_values"),
	}
	if data.StatusURL == "" {
		return executor.PollResult{}, fmt.Errorf("http: tracked handle missing status_url")
	}

	method := data.PollMethod
	if method == "" {
		method = "GET"
	}
	headers := data.Headers
	if len(data.PollHeaders) > 0 {
		headers = data.PollHeaders
	}

	result, bodyMap, err := m.doRequest(ctx, executor.ExecuteParams{
		RunID:       handle.RunID,
		WorkspaceID: "",
	}, requestSpec{
		URL:     data.StatusURL,
		Method:  method,
		Headers: headers,
	})
	if err != nil {
		return executor.PollResult{}, err
	}

	poll := executor.PollResult{
		ResponseBodyChunk: result.ResponseBody,
	}

	statusCode := 0
	if result.ResponseCode != nil {
		statusCode = *result.ResponseCode
	}

	if data.StatusField != "" && bodyMap != nil {
		if state, ok := extractString(bodyMap, data.StatusField); ok {
			switch {
			case containsString(data.RunningValues, state):
				poll.State = executor.RemoteRunning
				return poll, nil
			case containsString(data.SuccessValues, state):
				poll.State = executor.RemoteCompleted
				return poll, nil
			case containsString(data.FailedValues, state):
				poll.State = executor.RemoteFailed
				poll.Error = fmt.Errorf("http remote status %q", state)
				return poll, nil
			}
		}
	}

	if containsInt(defaultedStatusCodes(data.RunningStatusCode, 202), statusCode) {
		poll.State = executor.RemoteRunning
		return poll, nil
	}
	if containsInt(defaultedStatusCodes(data.SuccessStatusCode, 200, 201, 204), statusCode) {
		poll.State = executor.RemoteCompleted
		return poll, nil
	}

	poll.State = executor.RemoteFailed
	poll.Error = fmt.Errorf("http poll returned %d", statusCode)
	return poll, nil
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	result, _, err := m.doSync(ctx, params)
	return result, err
}

func (m *Method) Kill(ctx context.Context, handle executor.Handle) error {
	cancelURL := getString(handle.Data, "cancel_url")
	if cancelURL == "" {
		return fmt.Errorf("http: tracked handle missing cancel_url")
	}
	method := getString(handle.Data, "cancel_method")
	if method == "" {
		method = "POST"
	}
	headers := getMap(handle.Data, "headers")
	cancelHeaders := getMap(handle.Data, "cancel_headers")
	if len(cancelHeaders) > 0 {
		headers = cancelHeaders
	}

	result, _, err := m.doRequest(ctx, executor.ExecuteParams{RunID: handle.RunID}, requestSpec{
		URL:     cancelURL,
		Method:  method,
		Headers: headers,
	})
	if err != nil {
		return err
	}
	if result.ResponseCode != nil && *result.ResponseCode >= 200 && *result.ResponseCode < 300 {
		return nil
	}
	if result.ResponseCode != nil {
		return fmt.Errorf("http cancel returned %d", *result.ResponseCode)
	}
	return nil
}

func (m *Method) SupportsKill() bool      { return true }
func (m *Method) SupportsHeartbeat() bool { return false }

// IsAsync returns true because the HTTP method CAN be async (async_tracked mode).
// The orchestrator uses startResult.Result != nil to determine whether a specific
// invocation was actually async, so returning true here just signals capability.
func (m *Method) IsAsync() bool { return true }

type requestSpec struct {
	URL     string
	Method  string
	Headers map[string]any
	Body    string
	Payload any
}

func (m *Method) doSync(ctx context.Context, params executor.ExecuteParams) (executor.Result, map[string]any, error) {
	spec := requestSpec{
		URL:     substituteTemplates(getString(params.MethodConfig, "url"), params),
		Method:  strings.ToUpper(defaultString(getString(params.MethodConfig, "method"), "POST")),
		Headers: getMap(params.MethodConfig, "headers"),
		Body:    getString(params.MethodConfig, "body"),
	}
	if payload, ok := params.MethodConfig["payload"]; ok {
		spec.Payload = payload
	}
	return m.doRequest(ctx, params, spec)
}

func (m *Method) dispatchAsync(ctx context.Context, params executor.StartParams, tracked bool) (executor.Result, error) {
	execParams := executor.ExecuteParams{
		RunID:        params.RunID,
		WorkspaceID:  params.WorkspaceID,
		MethodConfig: params.MethodConfig,
		Environment:  params.Environment,
		APIBaseURL:   params.APIBaseURL,
	}
	spec := requestSpec{
		URL:     substituteTemplates(getString(params.MethodConfig, "url"), execParams),
		Method:  strings.ToUpper(defaultString(getString(params.MethodConfig, "method"), "POST")),
		Headers: getMap(params.MethodConfig, "headers"),
		Body:    getString(params.MethodConfig, "body"),
	}
	if payload, ok := params.MethodConfig["payload"]; ok {
		spec.Payload = payload
	}
	result, _, err := m.doRequest(ctx, execParams, spec)
	if err != nil {
		return result, err
	}
	if !containsInt(defaultAcceptedCodes(params.MethodConfig), derefInt(result.ResponseCode)) {
		result.Error = fmt.Errorf("http dispatch not accepted: %d", derefInt(result.ResponseCode))
	}
	return result, nil
}

func (m *Method) dispatchTracked(ctx context.Context, params executor.StartParams) (*executor.Result, executor.Handle, error) {
	result, err := m.dispatchAsync(ctx, params, true)
	if err != nil {
		return &result, executor.Handle{}, err
	}
	if result.Error != nil {
		return &result, executor.Handle{}, nil
	}

	var bodyMap map[string]any
	if result.ResponseBody != "" {
		_ = json.Unmarshal([]byte(result.ResponseBody), &bodyMap)
	}

	jobID, _ := extractString(bodyMap, getString(params.MethodConfig, "job_id_field"))
	statusURL, _ := extractString(bodyMap, getString(params.MethodConfig, "status_url_field"))
	cancelURL, _ := extractString(bodyMap, getString(params.MethodConfig, "cancel_url_field"))

	if tpl := getString(params.MethodConfig, "status_url_template"); tpl != "" {
		statusURL = substituteTrackedTemplate(tpl, params.RunID, params.WorkspaceID, jobID)
	}
	if tpl := getString(params.MethodConfig, "cancel_url_template"); tpl != "" {
		cancelURL = substituteTrackedTemplate(tpl, params.RunID, params.WorkspaceID, jobID)
	}

	if statusURL == "" {
		res := result
		res.Error = fmt.Errorf("http tracked dispatch requires status_url_field or status_url_template")
		return &res, executor.Handle{}, nil
	}

	handle := executor.Handle{
		MethodName: "http",
		RunID:      params.RunID,
		Data: map[string]any{
			"status_url":            statusURL,
			"cancel_url":            cancelURL,
			"job_id":                jobID,
			"headers":               getMap(params.MethodConfig, "headers"),
			"poll_headers":          getMap(params.MethodConfig, "poll_headers"),
			"cancel_headers":        getMap(params.MethodConfig, "cancel_headers"),
			"poll_method":           strings.ToUpper(defaultString(getString(params.MethodConfig, "poll_method"), "GET")),
			"cancel_method":         strings.ToUpper(defaultString(getString(params.MethodConfig, "cancel_method"), "POST")),
			"accepted_status_codes": defaultAcceptedCodes(params.MethodConfig),
			"running_status_codes":  defaultedStatusCodes(getIntSlice(params.MethodConfig, "running_status_codes"), 202),
			"success_status_codes":  defaultedStatusCodes(getIntSlice(params.MethodConfig, "success_status_codes"), 200, 201, 204),
			"status_field":          getString(params.MethodConfig, "status_field"),
			"running_values":        getStringSlice(params.MethodConfig, "running_values"),
			"success_values":        getStringSlice(params.MethodConfig, "success_values"),
			"failed_values":         getStringSlice(params.MethodConfig, "failed_values"),
		},
	}
	return nil, handle, nil
}

func (m *Method) doRequest(ctx context.Context, params executor.ExecuteParams, spec requestSpec) (executor.Result, map[string]any, error) {
	start := time.Now()
	if spec.URL == "" {
		return executor.Result{Error: fmt.Errorf("http: url is required")}, nil, nil
	}
	if !m.skipSSRF {
		if err := ssrf.ValidateFormat(spec.URL, nil); err != nil {
			return executor.Result{
				Error:      fmt.Errorf("SSRF validation failed: %w", err),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil, nil
		}
	}

	var body io.Reader
	if spec.Method == "POST" || spec.Method == "PUT" || spec.Method == "PATCH" || spec.Method == "DELETE" {
		if spec.Body != "" {
			spec.Body = substituteTemplates(spec.Body, params)
			body = strings.NewReader(spec.Body)
		} else if spec.Payload != nil {
			if payloadJSON, err := json.Marshal(spec.Payload); err == nil {
				body = bytes.NewReader(payloadJSON)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, spec.Method, spec.URL, body)
	if err != nil {
		return executor.Result{
			Error:      fmt.Errorf("create request: %w", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil, nil
	}
	for k, v := range spec.Headers {
		if s, ok := v.(string); ok {
			req.Header.Set(k, substituteTemplates(s, params))
		}
	}
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return executor.Result{
			Error:      err,
			DurationMs: durationMs,
		}, nil, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, int64(m.maxResponseSize)))
	code := resp.StatusCode

	var bodyMap map[string]any
	_ = json.Unmarshal(respBody, &bodyMap)

	return executor.Result{
		ResponseCode: &code,
		ResponseBody: string(respBody),
		DurationMs:   durationMs,
	}, bodyMap, nil
}

// substituteTemplates replaces {{variable}} placeholders.
func substituteTemplates(s string, params executor.ExecuteParams) string {
	s = strings.ReplaceAll(s, "{{run.id}}", params.RunID)
	s = strings.ReplaceAll(s, "{{workspace.id}}", params.WorkspaceID)
	s = strings.ReplaceAll(s, "{{now}}", time.Now().UTC().Format(time.RFC3339))
	return s
}

func substituteTrackedTemplate(s, runID, workspaceID, jobID string) string {
	s = strings.ReplaceAll(s, "{{run.id}}", runID)
	s = strings.ReplaceAll(s, "{{workspace.id}}", workspaceID)
	s = strings.ReplaceAll(s, "{{job.id}}", jobID)
	return s
}

func extractString(data map[string]any, path string) (string, bool) {
	if path == "" || data == nil {
		return "", false
	}
	var current any = data
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = obj[part]
		if !ok {
			return "", false
		}
	}
	switch v := current.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case float64:
		return strconv.FormatInt(int64(v), 10), true
	default:
		return "", false
	}
}

func getString(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return v
}

func getMap(cfg map[string]any, key string) map[string]any {
	v, _ := cfg[key].(map[string]any)
	return v
}

func getIntSlice(cfg map[string]any, key string) []int {
	raw, ok := cfg[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := make([]int, 0, len(v))
		for _, item := range v {
			switch n := item.(type) {
			case int:
				out = append(out, n)
			case int32:
				out = append(out, int(n))
			case int64:
				out = append(out, int(n))
			case float64:
				out = append(out, int(n))
			}
		}
		return out
	default:
		return nil
	}
}

func getStringSlice(cfg map[string]any, key string) []string {
	raw, ok := cfg[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func containsInt(values []int, v int) bool {
	for _, item := range values {
		if item == v {
			return true
		}
	}
	return false
}

func containsString(values []string, v string) bool {
	for _, item := range values {
		if item == v {
			return true
		}
	}
	return false
}

func defaultAcceptedCodes(cfg map[string]any) []int {
	if v := getIntSlice(cfg, "accepted_status_codes"); len(v) > 0 {
		return v
	}
	return []int{200, 201, 202}
}

func defaultedStatusCodes(values []int, defaults ...int) []int {
	if len(values) > 0 {
		return values
	}
	return defaults
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
