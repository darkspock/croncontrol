// Package infra manages auto-provisioned infrastructure for workspace container execution.
// Currently supports Hetzner Cloud as the server provider.
package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HetznerClient interacts with the Hetzner Cloud API.
type HetznerClient struct {
	apiToken string
	client   *http.Client
}

// NewHetznerClient creates a new Hetzner API client.
func NewHetznerClient(apiToken string) *HetznerClient {
	return &HetznerClient{
		apiToken: apiToken,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ServerInfo contains Hetzner server details.
type ServerInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PublicIP  string `json:"public_ip"`
	Created   string `json:"created"`
}

// CreateServer provisions a new server with cloud-init.
func (h *HetznerClient) CreateServer(ctx context.Context, name, serverType, datacenter, sshKeyName, cloudInit string) (*ServerInfo, error) {
	body, _ := json.Marshal(map[string]any{
		"name":        name,
		"server_type": serverType,
		"datacenter":  datacenter,
		"image":       "ubuntu-24.04",
		"ssh_keys":    []string{sshKeyName},
		"user_data":   cloudInit,
		"labels": map[string]string{
			"managed-by": "croncontrol",
		},
	})

	resp, err := h.doRequest(ctx, "POST", "https://api.hetzner.cloud/v1/servers", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Server struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"server"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &ServerInfo{
		ID:       result.Server.ID,
		Name:     result.Server.Name,
		Status:   result.Server.Status,
		PublicIP: result.Server.PublicNet.IPv4.IP,
	}, nil
}

// DeleteServer destroys a server.
func (h *HetznerClient) DeleteServer(ctx context.Context, serverID int64) error {
	_, err := h.doRequest(ctx, "DELETE", fmt.Sprintf("https://api.hetzner.cloud/v1/servers/%d", serverID), nil)
	return err
}

// GetServer returns server status.
func (h *HetznerClient) GetServer(ctx context.Context, serverID int64) (*ServerInfo, error) {
	resp, err := h.doRequest(ctx, "GET", fmt.Sprintf("https://api.hetzner.cloud/v1/servers/%d", serverID), nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Server struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
			Created string `json:"created"`
		} `json:"server"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &ServerInfo{
		ID:       result.Server.ID,
		Name:     result.Server.Name,
		Status:   result.Server.Status,
		PublicIP: result.Server.PublicNet.IPv4.IP,
		Created:  result.Server.Created,
	}, nil
}

func (h *HetznerClient) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hetzner: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hetzner: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
