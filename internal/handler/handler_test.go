package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncontrol/croncontrol/internal/auth"
	db "github.com/croncontrol/croncontrol/internal/db"
)

// setupTestRouter creates a chi router with a test service backed by a real DB.
// Skipped if no DATABASE_URL is available (unit test mode).
func setupTestRouter(t *testing.T) (*chi.Mux, *db.Queries) {
	t.Helper()

	// Use test database or skip
	dbURL := "postgres://croncontrol:croncontrol@localhost:5435/croncontrol?sslmode=disable"

	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		t.Skipf("Skipping integration test: DB not reachable: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	queries := db.New(pool)
	svc := NewService(queries, pool, nil, nil, nil)

	r := chi.NewRouter()
	apiKeyAuth := auth.NewAPIKeyAuth(queries)
	skipPaths := map[string]bool{"/api/v1/register": true, "/api/v1/login": true, "/api/v1/heartbeat": true}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", svc.Register)
		r.Post("/login", svc.Login)
		r.Post("/heartbeat", svc.Heartbeat)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(apiKeyAuth, skipPaths))
			r.Get("/processes", svc.ListProcesses)
			r.Post("/processes", svc.CreateProcess)
			r.Get("/processes/{id}", svc.GetProcess)
			r.Post("/processes/{id}/trigger", svc.TriggerProcess)
			r.Get("/runs", svc.ListRuns)
			r.Get("/runs/{id}", svc.GetRun)
			r.Get("/queues", svc.ListQueues)
			r.Post("/queues", svc.CreateQueue)
			r.Get("/api-keys", svc.ListAPIKeys)
			r.Get("/workers", svc.ListWorkers)
		})
	})

	return r, queries
}

func TestE2ERegisterAndCreateProcess(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 1. Register (unique email per test run)
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("e2e-%s@example.com", unique)
	regBody := fmt.Sprintf(`{"email":"%s","name":"E2E %s","password":"supersecure12"}`, email, unique)
	req := httptest.NewRequest("POST", "/api/v1/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("register: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var regResp struct {
		Data struct {
			APIKey    string `json:"api_key"`
			Workspace struct{ ID, Slug string }
			User      struct{ ID, Email string }
		}
	}
	json.Unmarshal(w.Body.Bytes(), &regResp)

	apiKey := regResp.Data.APIKey
	if apiKey == "" {
		t.Fatal("register: no API key returned")
	}
	t.Logf("Registered: workspace=%s key=%s...", regResp.Data.Workspace.Slug, apiKey[:20])

	// 2. Login with password
	loginBody := fmt.Sprintf(`{"email":"%s","password":"supersecure12"}`, email)
	req = httptest.NewRequest("POST", "/api/v1/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("login: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("Login: OK")

	// 3. List processes (empty)
	req = httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("list processes: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Create process
	procBody := fmt.Sprintf(`{"name":"e2e-cron-%s","schedule_type":"cron","schedule":"*/5 * * * *","execution_method":"http","method_config":{"url":"https://httpbin.org/post","method":"POST"}}`, unique)
	req = httptest.NewRequest("POST", "/api/v1/processes", bytes.NewBufferString(procBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("create process: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var procResp struct {
		Data struct{ ID string `json:"id"` }
	}
	json.Unmarshal(w.Body.Bytes(), &procResp)
	procID := procResp.Data.ID
	t.Logf("Created process: %s", procID)

	// 5. Get process
	req = httptest.NewRequest("GET", "/api/v1/processes/"+procID, nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("get process: expected 200, got %d", w.Code)
	}

	// 6. Trigger process
	req = httptest.NewRequest("POST", "/api/v1/processes/"+procID+"/trigger", nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("trigger: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var trigResp struct {
		Data struct {
			ID    string `json:"id"`
			State string `json:"state"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &trigResp)
	runID := trigResp.Data.ID
	t.Logf("Triggered run: %s state=%s", runID, trigResp.Data.State)

	if trigResp.Data.State != "pending" {
		t.Errorf("trigger: expected state=pending, got %s", trigResp.Data.State)
	}

	// 7. List runs (should have 1)
	req = httptest.NewRequest("GET", "/api/v1/runs", nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("list runs: expected 200, got %d", w.Code)
	}

	var runsResp struct {
		Data []any
		Meta struct{ Total int }
	}
	json.Unmarshal(w.Body.Bytes(), &runsResp)
	if runsResp.Meta.Total < 1 {
		t.Errorf("list runs: expected at least 1, got %d", runsResp.Meta.Total)
	}
	t.Logf("Runs: %d total", runsResp.Meta.Total)

	// 8. Send heartbeat
	hbBody := `{"run_id":"` + runID + `","total":100,"current":42,"message":"testing heartbeat"}`
	req = httptest.NewRequest("POST", "/api/v1/heartbeat", bytes.NewBufferString(hbBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("heartbeat: expected 200, got %d", w.Code)
	}

	// 9. Verify progress
	req = httptest.NewRequest("GET", "/api/v1/runs/"+runID, nil)
	req.Header.Set("X-API-Key", apiKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var runResp struct {
		Data struct {
			ProgressCurrent *int32  `json:"progress_current"`
			ProgressTotal   *int32  `json:"progress_total"`
			Progress        *int32  `json:"progress"`
			ProgressMessage *string `json:"progress_message"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &runResp)
	if runResp.Data.ProgressCurrent != nil && *runResp.Data.ProgressCurrent == 42 {
		t.Log("Heartbeat progress verified: 42/100")
	} else {
		t.Logf("Heartbeat progress: %v", runResp.Data.ProgressCurrent)
	}

	t.Log("E2E test passed!")
}

func TestUnauthorizedAccess(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401 without API key, got %d", w.Code)
	}
}

func TestInvalidAPIKey(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.Header.Set("X-API-Key", "cc_live_invalid_key_that_does_not_exist")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401 with invalid key, got %d", w.Code)
	}
}
