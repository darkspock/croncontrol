package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient is a simple HTTP client for the CronControl API.
type APIClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewAPIClient creates a new API client.
func NewAPIClient(baseURL, apiKey string) *APIClient {
	return &APIClient{
		baseURL: baseURL + "/api/v1",
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *APIClient) Get(ctx context.Context, path string) (any, error) {
	return c.do(ctx, "GET", path, nil)
}

func (c *APIClient) GetRaw(ctx context.Context, path string) (any, error) {
	return c.do(ctx, "GET", path, nil)
}

func (c *APIClient) Post(ctx context.Context, path string, body any) (any, error) {
	return c.do(ctx, "POST", path, body)
}

func (c *APIClient) do(ctx context.Context, method, path string, body any) (any, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return string(data), nil
	}
	return result, nil
}
