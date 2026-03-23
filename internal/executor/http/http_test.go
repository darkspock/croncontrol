package http

import (
	"context"
	"encoding/json"
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
	if !m.SupportsKill() {
		t.Error("HTTP should support kill for tracked async mode")
	}
	if m.SupportsHeartbeat() {
		t.Error("HTTP should not support heartbeat")
	}
}

func TestHTTPStartAsyncBlindAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"accepted":true}`))
	}))
	defer srv.Close()

	m := newTestMethod()
	startResult, err := m.Start(context.Background(), executor.StartParams{
		RunID:       "run_async_blind",
		WorkspaceID: "ws_test",
		MethodConfig: map[string]any{
			"url":                   srv.URL + "/jobs",
			"method":                "POST",
			"dispatch_mode":         "async_blind",
			"accepted_status_codes": []int{202},
		},
	})
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if startResult.Result == nil {
		t.Fatal("expected terminal dispatch result for async_blind")
	}
	if startResult.Result.Error != nil {
		t.Fatalf("unexpected result error: %v", startResult.Result.Error)
	}
	if startResult.Result.ResponseCode == nil || *startResult.Result.ResponseCode != 202 {
		t.Fatalf("expected accepted status 202, got %v", startResult.Result.ResponseCode)
	}
}

func TestHTTPStartAsyncTrackedReturnsHandle(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":     "job_123",
			"status_url": baseURL + "/jobs/job_123/status",
			"cancel_url": baseURL + "/jobs/job_123/cancel",
		})
	}))
	defer srv.Close()
	baseURL = srv.URL

	m := newTestMethod()
	startResult, err := m.Start(context.Background(), executor.StartParams{
		RunID:       "run_async_tracked",
		WorkspaceID: "ws_test",
		MethodConfig: map[string]any{
			"url":                   srv.URL + "/jobs",
			"method":                "POST",
			"dispatch_mode":         "async_tracked",
			"accepted_status_codes": []int{202},
			"job_id_field":          "job_id",
			"status_url_field":      "status_url",
			"cancel_url_field":      "cancel_url",
		},
	})
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if startResult.Result != nil {
		t.Fatalf("expected durable handle for async_tracked, got terminal result: %+v", *startResult.Result)
	}
	if got := startResult.Handle.MethodName; got != "http" {
		t.Fatalf("expected http handle, got %q", got)
	}
	if got := startResult.Handle.Data["job_id"]; got != "job_123" {
		t.Fatalf("expected job_id job_123, got %#v", got)
	}
	if got := startResult.Handle.Data["status_url"]; got != srv.URL+"/jobs/job_123/status" {
		t.Fatalf("unexpected status_url: %#v", got)
	}
	if got := startResult.Handle.Data["cancel_url"]; got != srv.URL+"/jobs/job_123/cancel" {
		t.Fatalf("unexpected cancel_url: %#v", got)
	}
}

func TestHTTPPollTrackedByStatusField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/job_123/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state": "completed",
		})
	}))
	defer srv.Close()

	m := newTestMethod()
	poll, err := m.Poll(context.Background(), executor.Handle{
		MethodName: "http",
		RunID:      "run_async_tracked",
		Data: map[string]any{
			"status_url":            srv.URL + "/jobs/job_123/status",
			"poll_method":           "GET",
			"status_field":          "state",
			"running_values":        []string{"queued", "running"},
			"success_values":        []string{"completed", "succeeded"},
			"accepted_status_codes": []int{202},
		},
	}, executor.PollCursor{})
	if err != nil {
		t.Fatalf("unexpected poll error: %v", err)
	}
	if poll.State != executor.RemoteCompleted {
		t.Fatalf("expected remote completed, got %q", poll.State)
	}
	if poll.ResponseBodyChunk == "" {
		t.Fatal("expected poll response body chunk")
	}
}

func TestHTTPKillTracked(t *testing.T) {
	var cancelled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/job_123/cancel" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		cancelled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	m := newTestMethod()
	if err := m.Kill(context.Background(), executor.Handle{
		MethodName: "http",
		RunID:      "run_async_tracked",
		Data: map[string]any{
			"cancel_url":    srv.URL + "/jobs/job_123/cancel",
			"cancel_method": "POST",
		},
	}); err != nil {
		t.Fatalf("unexpected kill error: %v", err)
	}
	if !cancelled {
		t.Fatal("expected tracked cancel endpoint to be called")
	}
}
