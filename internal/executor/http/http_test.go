package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/croncontrol/croncontrol/internal/executor"
)

func newTestMethod() *Method {
	m := New(0)
	m.skipSSRF = true
	return m
}

func TestHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	m := newTestMethod()
	result, err := m.Execute(context.Background(), executor.ExecuteParams{
		RunID:       "run_test123",
		WorkspaceID: "wsp_test",
		MethodConfig: map[string]any{
			"url":    srv.URL + "/webhook",
			"method": "POST",
			"body":   `{"run_id":"{{run.id}}"}`,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if !result.IsSuccess() {
		t.Errorf("expected success, got code %d", *result.ResponseCode)
	}
	if result.ResponseBody != `{"ok":true}` {
		t.Errorf("unexpected body: %s", result.ResponseBody)
	}
}

func TestHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	m := newTestMethod()
	result, _ := m.Execute(context.Background(), executor.ExecuteParams{
		MethodConfig: map[string]any{"url": srv.URL, "method": "GET"},
	})

	if result.IsSuccess() {
		t.Error("expected failure for 500 response")
	}
	if *result.ResponseCode != 500 {
		t.Errorf("expected 500, got %d", *result.ResponseCode)
	}
}

func TestHTTPNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer srv.Close()

	m := newTestMethod()
	result, _ := m.Execute(context.Background(), executor.ExecuteParams{
		MethodConfig: map[string]any{"url": srv.URL, "method": "GET"},
	})

	// Should NOT follow redirect — return the 302 directly
	if result.ResponseCode == nil || *result.ResponseCode != 302 {
		t.Errorf("expected 302 (no redirect follow), got %v", result.ResponseCode)
	}
}

func TestHTTPTemplateSubstitution(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		receivedBody = string(b[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	m := newTestMethod()
	m.Execute(context.Background(), executor.ExecuteParams{
		RunID:       "run_ABC123",
		WorkspaceID: "wsp_XYZ",
		MethodConfig: map[string]any{
			"url":    srv.URL,
			"method": "POST",
			"body":   `{"run":"{{run.id}}","ws":"{{workspace.id}}"}`,
		},
	})

	if receivedBody == "" {
		t.Fatal("no body received")
	}
	if receivedBody != `{"run":"run_ABC123","ws":"wsp_XYZ"}` {
		t.Errorf("unexpected body: %s", receivedBody)
	}
}

func TestHTTPSSRFBlocked(t *testing.T) {
	m := New(0) // NOT skipSSRF — test real SSRF blocking
	result, _ := m.Execute(context.Background(), executor.ExecuteParams{
		MethodConfig: map[string]any{"url": "http://127.0.0.1/secret", "method": "GET"},
	})

	if result.Error == nil {
		t.Error("expected SSRF error for localhost")
	}
}

func TestSupports(t *testing.T) {
	m := New(0)
	if m.SupportsKill() {
		t.Error("HTTP should not support kill")
	}
	if m.SupportsHeartbeat() {
		t.Error("HTTP should not support heartbeat")
	}
}
