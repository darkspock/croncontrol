package infra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(handler http.HandlerFunc) (*HetznerClient, *httptest.Server) {
	ts := httptest.NewServer(handler)
	c := NewHetznerClient("test-token")
	c.baseURL = ts.URL
	return c, ts
}

func TestCreateServer(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/servers" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-server" {
			t.Errorf("expected name=test-server, got %v", body["name"])
		}
		if body["server_type"] != "cx22" {
			t.Errorf("expected server_type=cx22, got %v", body["server_type"])
		}

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"server": map[string]any{
				"id":     42,
				"name":   "test-server",
				"status": "initializing",
				"public_net": map[string]any{
					"ipv4": map[string]any{"ip": "1.2.3.4"},
				},
			},
		})
	})
	defer ts.Close()

	info, err := client.CreateServer(context.Background(), "test-server", "cx22", "fsn1", "mykey", "#!/bin/bash\necho ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != 42 {
		t.Errorf("expected ID=42, got %d", info.ID)
	}
	if info.Name != "test-server" {
		t.Errorf("expected Name=test-server, got %s", info.Name)
	}
	if info.PublicIP != "1.2.3.4" {
		t.Errorf("expected IP=1.2.3.4, got %s", info.PublicIP)
	}
}

func TestDeleteServer(t *testing.T) {
	var called bool
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "DELETE" || r.URL.Path != "/servers/99" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(200)
	})
	defer ts.Close()

	err := client.DeleteServer(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("delete was not called")
	}
}

func TestGetServer(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/servers/55" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"server": map[string]any{
				"id":     55,
				"name":   "cc-ws-abc123",
				"status": "running",
				"public_net": map[string]any{
					"ipv4": map[string]any{"ip": "5.6.7.8"},
				},
				"created": "2026-03-19T10:00:00+00:00",
			},
		})
	})
	defer ts.Close()

	info, err := client.GetServer(context.Background(), 55)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != 55 {
		t.Errorf("expected ID=55, got %d", info.ID)
	}
	if info.Status != "running" {
		t.Errorf("expected Status=running, got %s", info.Status)
	}
	if info.PublicIP != "5.6.7.8" {
		t.Errorf("expected IP=5.6.7.8, got %s", info.PublicIP)
	}
}

func TestAPIError(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":{"message":"server not found"}}`))
	})
	defer ts.Close()

	_, err := client.GetServer(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "hetzner: HTTP 404: {\"error\":{\"message\":\"server not found\"}}" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRateLimit(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	})
	defer ts.Close()

	_, err := client.GetServer(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "hetzner: rate limited (retry after 30)" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestCreateServerError(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"error":{"message":"uniqueness_error"}}`))
	})
	defer ts.Close()

	_, err := client.CreateServer(context.Background(), "dup", "cx22", "fsn1", "key", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
