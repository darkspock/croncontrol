// Package croncontrol provides a thin Go client for the CronControl REST API.
//
// Usage:
//
//	cc := croncontrol.New("http://localhost:8080", "cc_live_...")
//	processes, _ := cc.ListProcesses(ctx, nil)
//	cc.TriggerProcess(ctx, "prc_01HYX...")
//	cc.Heartbeat(ctx, "run_01HYX...", 100, 50, "Halfway")
package croncontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client is a CronControl API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Error represents a structured API error.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// ListResponse wraps paginated list responses.
type ListResponse struct {
	Data []json.RawMessage `json:"data"`
	Meta map[string]any    `json:"meta,omitempty"`
}

// SingleResponse wraps single-resource responses.
type SingleResponse struct {
	Data json.RawMessage `json:"data"`
}

// New creates a CronControl client. Falls back to CRONCONTROL_URL and CRONCONTROL_API_KEY env vars.
func New(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("CRONCONTROL_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if apiKey == "" {
		apiKey = os.Getenv("CRONCONTROL_API_KEY")
	}
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, params map[string]string) ([]byte, error) {
	u := c.BaseURL + "/api/v1" + path
	if len(params) > 0 {
		v := url.Values{}
		for k, val := range params {
			if val != "" {
				v.Set(k, val)
			}
		}
		if qs := v.Encode(); qs != "" {
			u += "?" + qs
		}
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))

	if resp.StatusCode == 204 {
		return nil, nil
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Err Error `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		errResp.Err.Status = resp.StatusCode
		if errResp.Err.Code == "" {
			errResp.Err.Code = "UNKNOWN"
			errResp.Err.Message = http.StatusText(resp.StatusCode)
		}
		return nil, &errResp.Err
	}

	return respBody, nil
}

// -- Workspace --

func (c *Client) GetWorkspace(ctx context.Context) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/workspace", nil, nil)
}

// -- Processes --

func (c *Client) ListProcesses(ctx context.Context, params map[string]string) (*ListResponse, error) {
	return c.list(ctx, "/processes", params)
}

func (c *Client) GetProcess(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/processes/"+id, nil, nil)
}

func (c *Client) CreateProcess(ctx context.Context, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/processes", data, nil)
}

func (c *Client) UpdateProcess(ctx context.Context, id string, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "PUT", "/processes/"+id, data, nil)
}

func (c *Client) DeleteProcess(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", "/processes/"+id, nil, nil)
	return err
}

func (c *Client) TriggerProcess(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/processes/"+id+"/trigger", nil, nil)
}

func (c *Client) PauseProcess(ctx context.Context, id string, cancelPending bool) error {
	p := map[string]string{}
	if cancelPending {
		p["cancel_pending"] = "true"
	}
	_, err := c.do(ctx, "POST", "/processes/"+id+"/pause", nil, p)
	return err
}

func (c *Client) ResumeProcess(ctx context.Context, id string) error {
	_, err := c.do(ctx, "POST", "/processes/"+id+"/resume", nil, nil)
	return err
}

// -- Runs --

func (c *Client) ListRuns(ctx context.Context, params map[string]string) (*ListResponse, error) {
	return c.list(ctx, "/runs", params)
}

func (c *Client) GetRun(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/runs/"+id, nil, nil)
}

func (c *Client) CancelRun(ctx context.Context, id string) error {
	_, err := c.do(ctx, "POST", "/runs/"+id+"/cancel", nil, nil)
	return err
}

func (c *Client) KillRun(ctx context.Context, id string) error {
	_, err := c.do(ctx, "POST", "/runs/"+id+"/kill", nil, nil)
	return err
}

func (c *Client) ReplayRun(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/runs/"+id+"/replay", nil, nil)
}

func (c *Client) GetRunOutput(ctx context.Context, id string, stream string) (*ListResponse, error) {
	p := map[string]string{}
	if stream != "" {
		p["stream"] = stream
	}
	return c.list(ctx, "/runs/"+id+"/output", p)
}

// -- Queues --

func (c *Client) ListQueues(ctx context.Context) (*ListResponse, error) {
	return c.list(ctx, "/queues", nil)
}

func (c *Client) GetQueue(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/queues/"+id, nil, nil)
}

func (c *Client) CreateQueue(ctx context.Context, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/queues", data, nil)
}

// -- Jobs --

func (c *Client) ListJobs(ctx context.Context, params map[string]string) (*ListResponse, error) {
	return c.list(ctx, "/jobs", params)
}

func (c *Client) GetJob(ctx context.Context, id string) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/jobs/"+id, nil, nil)
}

func (c *Client) EnqueueJob(ctx context.Context, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/jobs", data, nil)
}

func (c *Client) CancelJob(ctx context.Context, id string) error {
	_, err := c.do(ctx, "POST", "/jobs/"+id+"/cancel", nil, nil)
	return err
}

func (c *Client) ReplayJob(ctx context.Context, id string, overrides map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/jobs/"+id+"/replay", overrides, nil)
}

// -- Workers --

func (c *Client) ListWorkers(ctx context.Context) (*ListResponse, error) {
	return c.list(ctx, "/workers", nil)
}

func (c *Client) CreateWorker(ctx context.Context, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/workers", data, nil)
}

func (c *Client) DeleteWorker(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", "/workers/"+id, nil, nil)
	return err
}

// -- API Keys --

func (c *Client) ListAPIKeys(ctx context.Context) (*ListResponse, error) {
	return c.list(ctx, "/api-keys", nil)
}

func (c *Client) CreateAPIKey(ctx context.Context, data map[string]any) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/api-keys", data, nil)
}

func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", "/api-keys/"+id, nil, nil)
	return err
}

// -- Run Result --

func (c *Client) SetResult(ctx context.Context, runID string, data any) error {
	_, err := c.do(ctx, "PATCH", "/runs/"+runID+"/result", data, nil)
	return err
}

func (c *Client) GetResult(ctx context.Context, runID string) (*SingleResponse, error) {
	return c.single(ctx, "GET", "/runs/"+runID+"/result", nil, nil)
}

// -- Secrets --

func (c *Client) ListSecrets(ctx context.Context) (*ListResponse, error) {
	return c.list(ctx, "/secrets", nil)
}

func (c *Client) CreateSecret(ctx context.Context, name, value string) (*SingleResponse, error) {
	return c.single(ctx, "POST", "/secrets", map[string]any{"name": name, "value": value}, nil)
}

func (c *Client) UpdateSecret(ctx context.Context, name, value string) error {
	_, err := c.do(ctx, "PUT", "/secrets/"+name, map[string]any{"value": value}, nil)
	return err
}

func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	_, err := c.do(ctx, "DELETE", "/secrets/"+name, nil, nil)
	return err
}

// -- Artifacts --

func (c *Client) ListArtifacts(ctx context.Context, runID string) (*ListResponse, error) {
	return c.list(ctx, "/runs/"+runID+"/artifacts", nil)
}

func (c *Client) GetArtifactURL(runID, name string) string {
	return c.BaseURL + "/api/v1/runs/" + runID + "/artifacts/" + name
}

// -- Heartbeat (no auth) --

func (c *Client) Heartbeat(ctx context.Context, runID string, total, current int, message string) error {
	_, err := c.do(ctx, "POST", "/heartbeat", map[string]any{
		"run_id":  runID,
		"total":   total,
		"current": current,
		"message": message,
	}, nil)
	return err
}

// -- Health --

func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	b, err := c.do(ctx, "GET", "/health", nil, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	json.Unmarshal(b, &result)
	return result, nil
}

// helpers

func (c *Client) list(ctx context.Context, path string, params map[string]string) (*ListResponse, error) {
	b, err := c.do(ctx, "GET", path, nil, params)
	if err != nil {
		return nil, err
	}
	var result ListResponse
	json.Unmarshal(b, &result)
	return &result, nil
}

func (c *Client) single(ctx context.Context, method, path string, body any, params map[string]string) (*SingleResponse, error) {
	b, err := c.do(ctx, method, path, body, params)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	var result SingleResponse
	json.Unmarshal(b, &result)
	return &result, nil
}
