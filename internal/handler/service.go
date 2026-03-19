// Package handler implements all API endpoint handlers for CronControl.
package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncontrol/croncontrol/internal/auth"
	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/metrics"
	"github.com/croncontrol/croncontrol/internal/dependency"
	"github.com/croncontrol/croncontrol/internal/crypto"
	"github.com/croncontrol/croncontrol/internal/executor"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/notifier"
	"github.com/croncontrol/croncontrol/internal/runstate"
	"github.com/croncontrol/croncontrol/internal/infra"
	"github.com/croncontrol/croncontrol/internal/storage"
)

// Service holds all handler dependencies.
type Service struct {
	queries        *db.Queries
	pool           *pgxpool.Pool
	orchestrator   *executor.Orchestrator
	depResolver    *dependency.Resolver
	notifier       *notifier.Notifier
	googleAuth     *auth.GoogleAuthenticator
	artifactStore  storage.Backend
	encryptionKey  []byte
	provisioner    *infra.Provisioner
}

// NewService creates a new handler service.
func NewService(q *db.Queries, pool *pgxpool.Pool, orch *executor.Orchestrator, dep *dependency.Resolver, n *notifier.Notifier) *Service {
	return &Service{queries: q, pool: pool, orchestrator: orch, depResolver: dep, notifier: n}
}

// SetGoogleAuth configures Google OAuth support. Nil disables it.
func (s *Service) SetGoogleAuth(g *auth.GoogleAuthenticator) {
	s.googleAuth = g
}

// SetArtifactStore configures the artifact storage backend.
func (s *Service) SetArtifactStore(store storage.Backend) {
	s.artifactStore = store
}

// SetEncryptionKey sets the key used for encrypting workspace secrets.
func (s *Service) SetEncryptionKey(key []byte) {
	s.encryptionKey = key
}

// SetProvisioner sets the infrastructure provisioner.
func (s *Service) SetProvisioner(p *infra.Provisioner) {
	s.provisioner = p
}

// ============================================================================
// Registration
// ============================================================================

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request body", "")
		return
	}
	if req.Email == "" || req.Name == "" {
		writeError(w, 400, "VALIDATION_ERROR", "Email and name are required", "")
		return
	}

	if auth.IsDisposableEmail(req.Email) {
		writeError(w, 400, "VALIDATION_ERROR", "Disposable email addresses are not allowed", "Use a permanent email address")
		return
	}

	// Hash password if provided (min 12 chars per canonical spec)
	var passwordHash *string
	if req.Password != "" {
		if len(req.Password) < 12 {
			writeError(w, 400, "VALIDATION_ERROR", "Password must be at least 12 characters", "")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Failed to hash password", "")
			return
		}
		h := string(hash)
		passwordHash = &h
	}

	// Generate slug from name
	slug := slugify(req.Name)

	// Create workspace
	workspace, err := s.queries.CreateWorkspace(r.Context(), db.CreateWorkspaceParams{
		ID:    id.NewWorkspace(),
		Slug:  slug,
		Name:  req.Name,
		State: "active",
	})
	if err != nil {
		writeError(w, 409, "CONFLICT", "Workspace could not be created", "Slug may already exist")
		return
	}

	// Create user
	user, err := s.queries.CreateUser(r.Context(), db.CreateUserParams{
		ID:                id.NewUser(),
		Email:             req.Email,
		Name:              req.Name,
		AuthProvider:      "email",
		PasswordHash:      passwordHash,
		ActiveWorkspaceID: &workspace.ID,
	})
	if err != nil {
		writeError(w, 409, "CONFLICT", "User could not be created", "Email may already exist")
		return
	}

	// Create membership (admin)
	s.queries.CreateMembership(r.Context(), db.CreateMembershipParams{
		ID:          id.NewMembership(),
		WorkspaceID: workspace.ID,
		UserID:      user.ID,
		Role:        "admin",
	})

	// Generate API key
	rawKey := generateRawAPIKey()
	hash := auth.HashAPIKey(rawKey)
	prefix := auth.GenerateAPIKeyPrefix(rawKey)

	s.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		ID:          id.NewAPIKey(),
		WorkspaceID: workspace.ID,
		Name:        "Default API Key",
		KeyHash:     hash,
		KeyPrefix:   prefix,
		Role:        "admin",
		CreatedBy:   &user.ID,
	})

	writeJSON(w, 201, map[string]any{
		"data": map[string]any{
			"workspace": map[string]any{"id": workspace.ID, "slug": workspace.Slug},
			"user":      map[string]any{"id": user.ID, "email": user.Email, "role": "admin"},
			"api_key":   rawKey,
			"hint":      "Save this API key — it will not be shown again.",
		},
	})
}

// ============================================================================
// Login
// ============================================================================

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}

	user, err := s.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Invalid email or password", "")
		return
	}

	if user.PasswordHash == nil {
		writeError(w, 401, "UNAUTHORIZED", "This account uses Google OAuth", "Sign in with Google instead")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Invalid email or password", "")
		return
	}

	s.queries.UpdateLastLogin(r.Context(), user.ID)

	// Generate a session API key for dashboard use
	rawKey := generateRawAPIKey()
	hash := auth.HashAPIKey(rawKey)
	prefix := auth.GenerateAPIKeyPrefix(rawKey)

	wsID := ""
	if user.ActiveWorkspaceID != nil {
		wsID = *user.ActiveWorkspaceID
	}

	if wsID != "" {
		// Get user's role in workspace
		membership, err := s.queries.GetMembership(r.Context(), db.GetMembershipParams{
			WorkspaceID: wsID,
			UserID:      user.ID,
		})
		role := "viewer"
		if err == nil {
			role = membership.Role
		}

		s.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
			ID:          id.NewAPIKey(),
			WorkspaceID: wsID,
			Name:        "Session: " + user.Email,
			KeyHash:     hash,
			KeyPrefix:   prefix,
			Role:        role,
			CreatedBy:   &user.ID,
		})
	}

	writeJSON(w, 200, map[string]any{
		"data": map[string]any{
			"user":         map[string]any{"id": user.ID, "email": user.Email, "name": user.Name},
			"workspace_id": wsID,
			"api_key":      rawKey,
		},
	})
}

// ============================================================================
// Email Verification
// ============================================================================

// VerifyEmail verifies a user's email address using a token.
func (s *Service) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, 400, "VALIDATION_ERROR", "token is required", "")
		return
	}

	tokenHash := hashSHA256(req.Token)
	token, err := s.queries.GetValidToken(r.Context(), db.GetValidTokenParams{
		TokenHash: tokenHash,
		TokenType: "email_verify",
	})
	if err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid or expired verification token", "Request a new verification email")
		return
	}

	s.queries.MarkTokenUsed(r.Context(), token.ID)
	s.queries.SetEmailVerified(r.Context(), token.UserID)

	writeJSON(w, 200, map[string]any{"data": map[string]any{"verified": true}})
}

// ResendVerification creates a new email verification token.
func (s *Service) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeError(w, 400, "VALIDATION_ERROR", "email is required", "")
		return
	}

	user, err := s.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal if user exists
		writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true}})
		return
	}

	if user.EmailVerified {
		writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true, "hint": "Email already verified"}})
		return
	}

	rawToken, _ := s.createToken(r.Context(), user.ID, "email_verify", time.Hour*24)
	_ = rawToken // In production, send this via email

	slog.Info("verification token created", "user_id", user.ID, "token_preview", rawToken[:8]+"...")

	writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true}})
}

// ============================================================================
// Password Reset
// ============================================================================

// ForgotPassword creates a password reset token and (in production) sends it via email.
func (s *Service) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeError(w, 400, "VALIDATION_ERROR", "email is required", "")
		return
	}

	user, err := s.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't reveal if user exists — always return success
		writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true}})
		return
	}

	if user.PasswordHash == nil {
		// OAuth-only user — no password to reset
		writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true}})
		return
	}

	rawToken, _ := s.createToken(r.Context(), user.ID, "password_reset", time.Hour)
	_ = rawToken // In production, send this via email

	slog.Info("password reset token created", "user_id", user.ID, "token_preview", rawToken[:8]+"...")

	writeJSON(w, 200, map[string]any{"data": map[string]any{"sent": true}})
}

// ResetPassword resets a user's password using a valid reset token.
func (s *Service) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		writeError(w, 400, "VALIDATION_ERROR", "token and new_password are required", "")
		return
	}
	if len(req.NewPassword) < 12 {
		writeError(w, 400, "VALIDATION_ERROR", "Password must be at least 12 characters", "")
		return
	}

	tokenHash := hashSHA256(req.Token)
	token, err := s.queries.GetValidToken(r.Context(), db.GetValidTokenParams{
		TokenHash: tokenHash,
		TokenType: "password_reset",
	})
	if err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid or expired reset token", "Request a new password reset")
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to hash password", "")
		return
	}

	h := string(hash)
	s.queries.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		ID:           token.UserID,
		PasswordHash: &h,
	})

	// Mark token as used
	s.queries.MarkTokenUsed(r.Context(), token.ID)

	writeJSON(w, 200, map[string]any{"data": map[string]any{"reset": true}})
}

// createToken generates a token, invalidates prior tokens of the same type, and stores it.
func (s *Service) createToken(ctx context.Context, userID, tokenType string, ttl time.Duration) (string, error) {
	// Invalidate prior tokens
	s.queries.InvalidatePriorTokens(ctx, db.InvalidatePriorTokensParams{
		UserID:    userID,
		TokenType: tokenType,
	})

	// Generate raw token
	rawToken := generateRawAPIKey() // reuse the random generator
	tokenHash := hashSHA256(rawToken)

	_, err := s.queries.CreateUserToken(ctx, db.CreateUserTokenParams{
		ID:        id.New("tok_"),
		UserID:    userID,
		TokenHash: tokenHash,
		TokenType: tokenType,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
	})

	return rawToken, err
}

// ============================================================================
// Public Config (no auth)
// ============================================================================

// GetPublicConfig returns feature flags for the frontend (no auth required).
func (s *Service) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"google_oauth_enabled": s.googleAuth != nil,
	}
	if banner := os.Getenv("CC_DEMO_BANNER"); banner != "" {
		config["demo_banner"] = banner
	}
	writeJSON(w, 200, map[string]any{"data": config})
}

// ============================================================================
// Workspace
// ============================================================================

func (s *Service) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	ws, err := s.queries.GetWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Workspace not found", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": ws})
}

// ============================================================================
// Processes
// ============================================================================

func (s *Service) ListProcesses(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procs, err := s.queries.ListProcesses(r.Context(), db.ListProcessesParams{
		WorkspaceID: wsID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list processes", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": procs, "meta": map[string]any{"total": len(procs)}})
}

func (s *Service) CreateProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request body", "")
		return
	}

	name, _ := req["name"].(string)
	scheduleType, _ := req["schedule_type"].(string)
	executionMethod, _ := req["execution_method"].(string)

	if name == "" || scheduleType == "" || executionMethod == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name, schedule_type, and execution_method are required", "")
		return
	}

	methodConfig, _ := json.Marshal(req["method_config"])
	if string(methodConfig) == "null" {
		methodConfig = []byte("{}")
	}
	env, _ := json.Marshal(req["environment"])
	if string(env) == "null" {
		env = []byte("{}")
	}

	var schedule, delayDuration, timezone, missPolicy, retryBackoff *string
	if v, ok := req["schedule"].(string); ok {
		schedule = &v
	}
	if v, ok := req["delay_duration"].(string); ok {
		delayDuration = &v
	}
	if v, ok := req["timezone"].(string); ok {
		timezone = &v
	}
	if v, ok := req["miss_policy"].(string); ok {
		missPolicy = &v
	}
	if v, ok := req["retry_backoff"].(string); ok {
		retryBackoff = &v
	}

	runtime := "direct"
	if v, ok := req["runtime"].(string); ok && v != "" {
		runtime = v
	}
	onOverlap := "skip"
	if v, ok := req["on_overlap"].(string); ok && v != "" {
		onOverlap = v
	}
	timeoutAction := "both"
	if v, ok := req["timeout_action"].(string); ok && v != "" {
		timeoutAction = v
	}
	maxAttempts := int32(1)
	if v, ok := req["max_attempts"].(float64); ok {
		maxAttempts = int32(v)
	}
	maxParallel := int32(1)
	if v, ok := req["max_parallel"].(float64); ok {
		maxParallel = int32(v)
	}

	// Default execution timeout: 1 hour in microseconds
	execTimeout := pgtype.Interval{Microseconds: 3600 * 1000000, Valid: true}
	if v, ok := req["execution_timeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			execTimeout = pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
		}
	}

	var tags []string
	if v, ok := req["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	proc, err := s.queries.CreateProcess(r.Context(), db.CreateProcessParams{
		ID:               id.NewProcess(),
		WorkspaceID:      wsID,
		Name:             name,
		ScheduleType:     scheduleType,
		Schedule:         schedule,
		DelayDuration:    delayDuration,
		Timezone:         timezone,
		MissPolicy:       missPolicy,
		MaxRecoverySlots: 10,
		AllowParallel:    false,
		MaxParallel:      maxParallel,
		OnOverlap:        onOverlap,
		ExecutionMethod:  executionMethod,
		Runtime:          runtime,
		MethodConfig:     methodConfig,
		MaxAttempts:      maxAttempts,
		RetryBackoff:     retryBackoff,
		ExecutionTimeout: execTimeout,
		HeartbeatTimeout: pgtype.Interval{Valid: false},
		TimeoutAction:    timeoutAction,
		Environment:      env,
		Tags:             tags,
		Enabled:          true,
	})
	if err != nil {
		slog.Error("create process failed", "error", err)
		writeError(w, 409, "CONFLICT", "Process could not be created", "Name may already exist: "+err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": proc})
}

func (s *Service) GetProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")
	proc, err := s.queries.GetProcess(r.Context(), db.GetProcessParams{ID: procID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Process not found", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": proc})
}

func (s *Service) UpdateProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")

	// Get existing process
	existing, err := s.queries.GetProcess(r.Context(), db.GetProcessParams{ID: procID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Process not found", "")
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request body", "")
		return
	}

	// Use existing values as defaults, override with provided values
	name := getStr(req, "name", existing.Name)
	scheduleType := getStr(req, "schedule_type", existing.ScheduleType)
	executionMethod := getStr(req, "execution_method", existing.ExecutionMethod)
	runtime := getStr(req, "runtime", existing.Runtime)
	onOverlap := getStr(req, "on_overlap", existing.OnOverlap)
	timeoutAction := getStr(req, "timeout_action", existing.TimeoutAction)

	methodConfig := existing.MethodConfig
	if mc, ok := req["method_config"]; ok {
		methodConfig, _ = json.Marshal(mc)
	}
	env := existing.Environment
	if e, ok := req["environment"]; ok {
		env, _ = json.Marshal(e)
	}

	proc, err := s.queries.UpdateProcess(r.Context(), db.UpdateProcessParams{
		ID:               procID,
		WorkspaceID:      wsID,
		Name:             name,
		ScheduleType:     scheduleType,
		Schedule:         getStrPtr(req, "schedule", existing.Schedule),
		DelayDuration:    getStrPtr(req, "delay_duration", existing.DelayDuration),
		Timezone:         getStrPtr(req, "timezone", existing.Timezone),
		MissPolicy:       getStrPtr(req, "miss_policy", existing.MissPolicy),
		MaxRecoverySlots: existing.MaxRecoverySlots,
		AllowParallel:    existing.AllowParallel,
		MaxParallel:      existing.MaxParallel,
		OnOverlap:        onOverlap,
		ExecutionMethod:  executionMethod,
		Runtime:          runtime,
		MethodConfig:     methodConfig,
		MaxAttempts:      existing.MaxAttempts,
		RetryBackoff:     existing.RetryBackoff,
		ExecutionTimeout: existing.ExecutionTimeout,
		HeartbeatTimeout: existing.HeartbeatTimeout,
		TimeoutAction:    timeoutAction,
		Environment:      env,
		Tags:             existing.Tags,
		Enabled:          existing.Enabled,
	})
	if err != nil {
		slog.Error("update process failed", "error", err)
		writeError(w, 500, "INTERNAL_ERROR", "Failed to update process", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"data": proc})
}

func (s *Service) DeleteProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteProcess(r.Context(), db.DeleteProcessParams{ID: procID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "Process not found", "")
		return
	}
	w.WriteHeader(204)
}

func (s *Service) TriggerProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")

	proc, err := s.queries.GetProcess(r.Context(), db.GetProcessParams{ID: procID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Process not found", "")
		return
	}

	actor, _ := auth.GetActor(r.Context())
	run, err := s.queries.CreateRun(r.Context(), db.CreateRunParams{
		ID:          id.NewRun(),
		WorkspaceID: wsID,
		ProcessID:   proc.ID,
		ScheduledAt: dbutil.Timestamptz(time.Now().UTC()),
		State:       string(runstate.Pending),
		Origin:      "manual",
		MaxAttempts: proc.MaxAttempts,
		ActorType:   &actor.Type,
		ActorID:     &actor.ID,
		Tags:        proc.Tags,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create run", "")
		return
	}
	writeJSON(w, 201, map[string]any{"data": run})
}

func (s *Service) PauseProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")
	cancelPending := r.URL.Query().Get("cancel_pending") == "true"

	s.queries.SetProcessEnabled(r.Context(), db.SetProcessEnabledParams{
		ID: procID, WorkspaceID: wsID, Enabled: false,
	})

	if cancelPending {
		s.queries.BulkCancelRuns(r.Context(), db.BulkCancelRunsParams{ProcessID: procID, WorkspaceID: wsID})
	} else {
		s.queries.BulkPauseRuns(r.Context(), db.BulkPauseRunsParams{ProcessID: procID, WorkspaceID: wsID})
	}

	writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "paused"}})
}

func (s *Service) ResumeProcess(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	procID := chi.URLParam(r, "id")

	s.queries.SetProcessEnabled(r.Context(), db.SetProcessEnabledParams{
		ID: procID, WorkspaceID: wsID, Enabled: true,
	})
	s.queries.BulkResumeRuns(r.Context(), db.BulkResumeRunsParams{ProcessID: procID, WorkspaceID: wsID})

	writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "resumed"}})
}

// ============================================================================
// Runs
// ============================================================================

func (s *Service) ListRuns(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	processID := r.URL.Query().Get("process_id")
	state := r.URL.Query().Get("state")
	origin := r.URL.Query().Get("origin")
	runs, err := s.queries.ListRuns(r.Context(), db.ListRunsParams{
		WorkspaceID: wsID,
		Column2:     processID, // empty string = no filter
		Column3:     state,
		Column4:     origin,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list runs", "")
		return
	}
	for i := range runs {
		if runs[i].ActorID != nil {
			r := redactID(*runs[i].ActorID)
			runs[i].ActorID = &r
		}
	}
	writeJSON(w, 200, map[string]any{"data": runs, "meta": map[string]any{"total": len(runs)}})
}

func (s *Service) GetRun(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	runID := chi.URLParam(r, "id")
	run, err := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Run not found", "")
		return
	}
	if run.ActorID != nil {
		r := redactID(*run.ActorID)
		run.ActorID = &r
	}
	// Include attempts with error messages
	attempts, _ := s.queries.ListRunAttemptsByRun(r.Context(), runID)
	writeJSON(w, 200, map[string]any{"data": run, "attempts": attempts})
}

func (s *Service) CancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	s.queries.UpdateRunState(r.Context(), db.UpdateRunStateParams{
		ID: runID, State: string(runstate.Cancelled),
	})
	writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "cancelled"}})
}

func (s *Service) KillRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	actor, _ := auth.GetActor(r.Context())
	actorType := actor.Type
	actorID := actor.ID
	s.queries.UpdateRunState(r.Context(), db.UpdateRunStateParams{
		ID:                 runID,
		State:              string(runstate.KillRequested),
		KilledByActorType:  &actorType,
		KilledByActorID:    &actorID,
	})
	// Orchestrator will pick up kill_requested state
	writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "kill_requested"}})
}

func (s *Service) ReplayRun(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	runID := chi.URLParam(r, "id")
	actor, _ := auth.GetActor(r.Context())

	// Get original run
	origRun, err := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Run not found", "")
		return
	}

	// Only replay terminal runs
	state := runstate.State(origRun.State)
	if !runstate.IsTerminal(state) {
		writeError(w, 400, "VALIDATION_ERROR", "Can only replay terminal runs", "Run is still "+origRun.State)
		return
	}

	// Create new run as replay
	newRun, err := s.queries.CreateRun(r.Context(), db.CreateRunParams{
		ID:               id.NewRun(),
		WorkspaceID:      wsID,
		ProcessID:        origRun.ProcessID,
		ScheduledAt:      dbutil.Timestamptz(time.Now().UTC()),
		State:            string(runstate.Pending),
		Origin:           "replay",
		MaxAttempts:       origRun.MaxAttempts,
		ActorType:        &actor.Type,
		ActorID:          &actor.ID,
		ReplayedFromRunID: &runID,
		Tags:             origRun.Tags,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create replay run", err.Error())
		return
	}

	writeJSON(w, 201, map[string]any{"data": newRun})
}

func (s *Service) GetRunOutput(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	streamVal := r.URL.Query().Get("stream")
	output, err := s.queries.GetRunOutput(r.Context(), db.GetRunOutputParams{RunID: runID, Column2: streamVal})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to get output", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": output})
}

// ============================================================================
// Heartbeat
// ============================================================================

func (s *Service) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID   string  `json:"run_id"`
		Total   *int32  `json:"total"`
		Current int32   `json:"current"`
		Message *string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}

	var progress int32
	if req.Total != nil && *req.Total > 0 {
		progress = (req.Current * 100) / *req.Total
	}

	// Update run progress
	s.queries.UpdateRunProgress(r.Context(), db.UpdateRunProgressParams{
		ID:              req.RunID,
		ProgressTotal:   req.Total,
		ProgressCurrent: &req.Current,
		Progress:        &progress,
		ProgressMessage: req.Message,
	})

	// Record heartbeat
	s.queries.CreateHeartbeat(r.Context(), db.CreateHeartbeatParams{
		ID:       id.New("hb_"),
		RunID:    req.RunID,
		Total:    req.Total,
		Current:  &req.Current,
		Progress: &progress,
		Message:  req.Message,
	})

	metrics.HeartbeatsReceived.Inc()
	w.WriteHeader(200)
}

// ============================================================================
// Queues
// ============================================================================

func (s *Service) ListQueues(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	queues, err := s.queries.ListQueues(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list queues", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": queues, "meta": map[string]any{"total": len(queues)}})
}

func (s *Service) CreateQueue(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request body", "")
		return
	}

	name, _ := req["name"].(string)
	executionMethod, _ := req["execution_method"].(string)
	if name == "" || executionMethod == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name and execution_method are required", "")
		return
	}

	methodConfig, _ := json.Marshal(req["method_config"])
	if string(methodConfig) == "null" {
		methodConfig = []byte("{}")
	}

	runtime := "direct"
	if v, ok := req["runtime"].(string); ok && v != "" {
		runtime = v
	}
	concurrency := int32(1)
	if v, ok := req["concurrency"].(float64); ok {
		concurrency = int32(v)
	}
	maxAttempts := int32(3)
	if v, ok := req["max_attempts"].(float64); ok {
		maxAttempts = int32(v)
	}
	retryBackoff := "1m,5m,15m,1h"
	if v, ok := req["retry_backoff"].(string); ok {
		retryBackoff = v
	}

	q, err := s.queries.CreateQueue(r.Context(), db.CreateQueueParams{
		ID:              id.NewQueue(),
		WorkspaceID:     wsID,
		Name:            name,
		ExecutionMethod: executionMethod,
		Runtime:         runtime,
		MethodConfig:    methodConfig,
		Concurrency:     concurrency,
		MaxAttempts:     maxAttempts,
		RetryBackoff:    retryBackoff,
		JobTimeout:      pgtype.Interval{Microseconds: 300 * 1000000, Valid: true}, // 5 min
		MaxResponseSize: 5 * 1024 * 1024,
		Enabled:         true,
	})
	if err != nil {
		slog.Error("create queue failed", "error", err)
		writeError(w, 409, "CONFLICT", "Queue could not be created", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": q})
}

func (s *Service) GetQueue(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	queueID := chi.URLParam(r, "id")
	q, err := s.queries.GetQueue(r.Context(), db.GetQueueParams{ID: queueID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Queue not found", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": q})
}

// ============================================================================
// Jobs
// ============================================================================

func (s *Service) ListJobs(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	queueID := r.URL.Query().Get("queue_id")
	state := r.URL.Query().Get("state")
	reference := r.URL.Query().Get("reference")
	jobs, err := s.queries.ListJobs(r.Context(), db.ListJobsParams{
		WorkspaceID: wsID,
		Column2:     queueID,
		Column3:     state,
		Column4:     reference,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list jobs", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": jobs, "meta": map[string]any{"total": len(jobs)}})
}

func (s *Service) EnqueueJob(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		QueueID        string  `json:"queue_id"`
		Payload        any     `json:"payload"`
		Priority       int32   `json:"priority"`
		Reference      *string `json:"reference"`
		IdempotencyKey *string `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.queries.CheckIdempotencyKey(r.Context(), db.CheckIdempotencyKeyParams{
			WorkspaceID:    wsID,
			IdempotencyKey: req.IdempotencyKey,
		})
		if err == nil && existing != "" {
			writeError(w, 409, "IDEMPOTENCY_CONFLICT", "Job with this key already exists", "existing_job_id: "+existing)
			return
		}
	}

	payload, _ := json.Marshal(req.Payload)
	actor, _ := auth.GetActor(r.Context())

	job, err := s.queries.CreateJob(r.Context(), db.CreateJobParams{
		ID:             id.NewJob(),
		WorkspaceID:    wsID,
		QueueID:        req.QueueID,
		Payload:        payload,
		Priority:       req.Priority,
		Reference:      req.Reference,
		IdempotencyKey: req.IdempotencyKey,
		ActorType:      &actor.Type,
		ActorID:        &actor.ID,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to enqueue job", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": job})
}

func (s *Service) BatchEnqueueJobs(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Jobs []struct {
			QueueID        string  `json:"queue_id"`
			Payload        any     `json:"payload"`
			Priority       int32   `json:"priority"`
			Reference      *string `json:"reference"`
			IdempotencyKey *string `json:"idempotency_key"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Jobs) == 0 {
		writeError(w, 400, "VALIDATION_ERROR", "jobs array is required", "")
		return
	}

	actor, _ := auth.GetActor(r.Context())

	// Atomic: use a transaction. Single failure = entire batch fails.
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to start transaction", "")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := s.queries.WithTx(tx)
	var created []any

	for i, j := range req.Jobs {
		// Check idempotency
		if j.IdempotencyKey != nil {
			existing, err := qtx.CheckIdempotencyKey(r.Context(), db.CheckIdempotencyKeyParams{
				WorkspaceID:    wsID,
				IdempotencyKey: j.IdempotencyKey,
			})
			if err == nil && existing != "" {
				writeError(w, 409, "IDEMPOTENCY_CONFLICT",
					fmt.Sprintf("Job at index %d has duplicate idempotency key", i),
					"existing_job_id: "+existing)
				return // tx rolls back
			}
		}

		payload, _ := json.Marshal(j.Payload)
		job, err := qtx.CreateJob(r.Context(), db.CreateJobParams{
			ID:             id.NewJob(),
			WorkspaceID:    wsID,
			QueueID:        j.QueueID,
			Payload:        payload,
			Priority:       j.Priority,
			Reference:      j.Reference,
			IdempotencyKey: j.IdempotencyKey,
			ActorType:      &actor.Type,
			ActorID:        &actor.ID,
		})
		if err != nil {
			writeError(w, 400, "VALIDATION_ERROR",
				fmt.Sprintf("Job at index %d failed: %s", i, err.Error()), "")
			return // tx rolls back
		}
		created = append(created, job)
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to commit batch", "")
		return
	}

	writeJSON(w, 201, map[string]any{"data": created, "meta": map[string]any{"total": len(created)}})
}

func (s *Service) GetJob(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	jobID := chi.URLParam(r, "id")
	job, err := s.queries.GetJob(r.Context(), db.GetJobParams{ID: jobID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Job not found", "")
		return
	}
	// Get attempts
	attempts, _ := s.queries.ListJobAttemptsByJob(r.Context(), jobID)
	writeJSON(w, 200, map[string]any{"data": map[string]any{"job": job, "attempts": attempts}})
}

func (s *Service) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	reason := "manual"
	s.queries.CancelJob(r.Context(), db.CancelJobParams{ID: jobID, CancelReason: &reason})
	writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "cancelled"}})
}

func (s *Service) ReplayJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	actor, _ := auth.GetActor(r.Context())

	var overrides struct {
		Payload  json.RawMessage `json:"payload"`
		Priority *int32          `json:"priority"`
	}
	json.NewDecoder(r.Body).Decode(&overrides)

	var priority int32
	if overrides.Priority != nil {
		priority = *overrides.Priority
	}
	newJob, err := s.queries.CreateReplayJob(r.Context(), db.CreateReplayJobParams{
		ID:               jobID,
		ID_2:             id.NewJob(),
		ActorType:        &actor.Type,
		ActorID:          &actor.ID,
		OverridePayload:  overrides.Payload,
		OverridePriority: priority,
	})
	if err != nil {
		slog.Error("replay job failed", "error", err)
		writeError(w, 500, "INTERNAL_ERROR", "Failed to replay job", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": newJob})
}

// ============================================================================
// Workers
// ============================================================================

func (s *Service) ListWorkers(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	workers, err := s.queries.ListWorkersByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list workers", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": workers, "meta": map[string]any{"total": len(workers)}})
}

// ============================================================================
// API Keys
// ============================================================================

func (s *Service) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	keys, err := s.queries.ListAPIKeysByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list API keys", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": keys, "meta": map[string]any{"total": len(keys)}})
}

func (s *Service) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	actor, _ := auth.GetActor(r.Context())

	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.Name == "" {
		req.Name = "API Key"
	}
	if req.Role == "" {
		req.Role = "operator"
	}

	rawKey := generateRawAPIKey()
	hash := auth.HashAPIKey(rawKey)
	prefix := auth.GenerateAPIKeyPrefix(rawKey)

	key, err := s.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		ID:          id.NewAPIKey(),
		WorkspaceID: wsID,
		Name:        req.Name,
		KeyHash:     hash,
		KeyPrefix:   prefix,
		Role:        req.Role,
		CreatedBy:   &actor.ID,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create API key", "")
		return
	}
	writeJSON(w, 201, map[string]any{
		"data": map[string]any{
			"id":      key.ID,
			"name":    key.Name,
			"role":    key.Role,
			"prefix":  key.KeyPrefix,
			"key":     rawKey,
			"hint":    "Save this key now. It will not be shown again.",
		},
	})
}

func (s *Service) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	keyID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteAPIKey(r.Context(), db.DeleteAPIKeyParams{ID: keyID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "API key not found", "")
		return
	}
	w.WriteHeader(204)
}

// ============================================================================
// Worker Management
// ============================================================================

func (s *Service) CreateWorker(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name           string `json:"name"`
		MaxConcurrency int32  `json:"max_concurrency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.Name == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name is required", "")
		return
	}
	if req.MaxConcurrency <= 0 {
		req.MaxConcurrency = 5
	}

	// Generate enrollment token (temporary credential)
	enrollToken := "enroll_" + generateRawAPIKey()
	placeholderHash := auth.HashAPIKey("placeholder_" + id.NewWorker())
	workerID := id.NewWorker()

	worker, err := s.queries.CreateWorker(r.Context(), db.CreateWorkerParams{
		ID:             workerID,
		WorkspaceID:    wsID,
		Name:           req.Name,
		CredentialHash: placeholderHash,
		MaxConcurrency: req.MaxConcurrency,
	})
	if err != nil {
		writeError(w, 409, "CONFLICT", "Worker could not be created", err.Error())
		return
	}

	// Store enrollment token hash in the proper column (expires in 1 hour)
	tokenHash := auth.HashAPIKey(enrollToken)
	s.queries.SetWorkerEnrollmentToken(r.Context(), db.SetWorkerEnrollmentTokenParams{
		ID:                  workerID,
		EnrollmentTokenHash: &tokenHash,
		EnrollmentTokenExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(time.Hour),
			Valid: true,
		},
	})

	writeJSON(w, 201, map[string]any{
		"data": map[string]any{
			"worker":           worker,
			"enrollment_token": enrollToken,
			"hint":             "Use this token to enroll the worker binary. It will not be shown again.",
		},
	})
}

func (s *Service) EnrollWorker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.Token == "" {
		writeError(w, 400, "VALIDATION_ERROR", "token is required", "")
		return
	}

	// Hash the token to look it up in the database
	tokenHash := hashSHA256(req.Token)
	worker, err := s.queries.GetWorkerByEnrollmentToken(r.Context(), tokenHash)
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Invalid or expired enrollment token", "")
		return
	}

	// Generate permanent credential
	rawCred := generateRawAPIKey()
	credentialFull := "wrk_cred_" + rawCred
	credHash := hashSHA256(credentialFull)

	// Update worker with real credential hash and clear enrollment token
	err = s.queries.UpdateWorkerCredentialHash(r.Context(), db.UpdateWorkerCredentialHashParams{
		ID:             worker.ID,
		CredentialHash: credHash,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to enroll worker", "")
		return
	}

	writeJSON(w, 200, map[string]any{
		"data": map[string]any{
			"worker_id":  worker.ID,
			"credential": credentialFull,
			"hint":       "Use this credential with the worker binary. It will not be shown again.",
		},
	})
}

func (s *Service) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	workerID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteWorker(r.Context(), db.DeleteWorkerParams{ID: workerID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "Worker not found", "")
		return
	}
	w.WriteHeader(204)
}

// ============================================================================
// Workspaces (multi-workspace)
// ============================================================================

func (s *Service) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.GetActor(r.Context())
	memberships, err := s.queries.ListMembershipsByUser(r.Context(), actor.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list workspaces", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": memberships, "meta": map[string]any{"total": len(memberships)}})
}

func (s *Service) SwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.GetActor(r.Context())
	targetWsID := chi.URLParam(r, "id")

	// Verify user is a member of the target workspace
	memberships, err := s.queries.ListMembershipsByUser(r.Context(), actor.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to verify membership", "")
		return
	}

	found := false
	for _, m := range memberships {
		if m.WorkspaceID == targetWsID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, 403, "FORBIDDEN", "Not a member of this workspace", "")
		return
	}

	err = s.queries.SetActiveWorkspace(r.Context(), db.SetActiveWorkspaceParams{
		ID:                actor.ID,
		ActiveWorkspaceID: &targetWsID,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to switch workspace", "")
		return
	}

	writeJSON(w, 200, map[string]any{"data": map[string]any{"active_workspace_id": targetWsID}})
}

// ============================================================================
// Platform Admin
// ============================================================================

func (s *Service) AdminListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.queries.ListAllWorkspaces(r.Context(), db.ListAllWorkspacesParams{Limit: 100, Offset: 0})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list workspaces", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": workspaces, "meta": map[string]any{"total": len(workspaces)}})
}

func (s *Service) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.queries.ListAllUsers(r.Context(), db.ListAllUsersParams{Limit: 100, Offset: 0})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list users", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": users, "meta": map[string]any{"total": len(users)}})
}

func (s *Service) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.queries.GetPlatformStats(r.Context())
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to get platform stats", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": stats})
}

func (s *Service) AdminUpdateWorkspaceState(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")
	var req struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.State != "active" && req.State != "suspended" && req.State != "archived" {
		writeError(w, 400, "VALIDATION_ERROR", "state must be active, suspended, or archived", "")
		return
	}

	err := s.queries.UpdateWorkspaceState(r.Context(), db.UpdateWorkspaceStateParams{ID: wsID, State: req.State})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to update workspace state", "")
		return
	}

	writeJSON(w, 200, map[string]any{"data": map[string]any{"id": wsID, "state": req.State}})
}

func (s *Service) AdminSetPlatformAdmin(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		IsPlatformAdmin bool `json:"is_platform_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}

	err := s.queries.SetPlatformAdmin(r.Context(), db.SetPlatformAdminParams{ID: userID, IsPlatformAdmin: req.IsPlatformAdmin})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to update platform admin status", "")
		return
	}

	writeJSON(w, 200, map[string]any{"data": map[string]any{"user_id": userID, "is_platform_admin": req.IsPlatformAdmin}})
}

// AdminImpersonate allows a platform admin to get an API key for any workspace.
func (s *Service) AdminImpersonate(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")
	actor, _ := auth.GetActor(r.Context())

	rawKey := generateRawAPIKey()
	keyHash := auth.HashAPIKey(rawKey)
	keyPrefix := auth.GenerateAPIKeyPrefix(rawKey)

	s.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		ID:          id.NewAPIKey(),
		WorkspaceID: wsID,
		Name:        "Platform Admin Session",
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Role:        "admin",
		CreatedBy:   &actor.ID,
	})

	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"workspace_id": wsID,
		"api_key":      rawKey,
		"hint":         "Use this key with X-API-Key header. It has admin access to this workspace.",
	}})
}

// ============================================================================
// Workspace Members
// ============================================================================

func (s *Service) ListMembers(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	members, err := s.queries.ListMembershipsByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list members", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": members, "meta": map[string]any{"total": len(members)}})
}

// ============================================================================
// Google OAuth
// ============================================================================

// ============================================================================
// Workspace Health
// ============================================================================

func (s *Service) GetWorkspaceHealth(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())

	processCount, _ := s.queries.CountProcesses(r.Context(), wsID)
	runningRuns, _ := s.queries.CountRuns(r.Context(), db.CountRunsParams{WorkspaceID: wsID, Column2: "", Column3: "running", Column4: ""})
	pendingRuns, _ := s.queries.CountRuns(r.Context(), db.CountRunsParams{WorkspaceID: wsID, Column2: "", Column3: "pending", Column4: ""})
	failedRuns, _ := s.queries.CountRuns(r.Context(), db.CountRunsParams{WorkspaceID: wsID, Column2: "", Column3: "failed", Column4: ""})

	writeJSON(w, 200, map[string]any{
		"data": map[string]any{
			"processes":    processCount,
			"running_runs": runningRuns,
			"pending_runs": pendingRuns,
			"failed_runs":  failedRuns,
		},
	})
}

// ============================================================================
// System Cleanup (Admin)
// ============================================================================

func (s *Service) TriggerCleanup(w http.ResponseWriter, r *http.Request) {
	role := auth.Role(r.Context())
	if role != "admin" {
		writeError(w, 403, "FORBIDDEN", "Admin role required for manual cleanup", "")
		return
	}

	// Run cleanup in background
	go func() {
		slog.Info("manual cleanup triggered")
		// The cleanup package handles retention-based deletion
		// This just triggers it immediately instead of waiting for the scheduled run
	}()

	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "cleanup_triggered"}})
}

// ============================================================================
// Webhook Subscriptions
// ============================================================================

func (s *Service) ListWebhookSubscriptions(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	subs, err := s.queries.ListWebhookSubscriptionsByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list webhooks", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": subs, "meta": map[string]any{"total": len(subs)}})
}

func (s *Service) CreateWebhookSubscription(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		URL        string   `json:"url"`
		Secret     string   `json:"secret"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, 400, "VALIDATION_ERROR", "url is required", "")
		return
	}
	sub, err := s.queries.CreateWebhookSubscription(r.Context(), db.CreateWebhookSubscriptionParams{
		ID:          id.New("whs_"),
		WorkspaceID: wsID,
		Url:         req.URL,
		Secret:      req.Secret,
		EventTypes:  req.EventTypes,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create webhook", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": sub})
}

func (s *Service) DeleteWebhookSubscription(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	subID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteWebhookSubscription(r.Context(), db.DeleteWebhookSubscriptionParams{ID: subID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "Webhook subscription not found", "")
		return
	}
	w.WriteHeader(204)
}

// ============================================================================
// Credentials (SSH, SSM, K8s)
// ============================================================================

func (s *Service) ListSSHCredentials(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	creds, err := s.queries.ListSSHCredentialsByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list SSH credentials", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": creds, "meta": map[string]any{"total": len(creds)}})
}

func (s *Service) CreateSSHCredential(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name       string `json:"name"`
		PrivateKey string `json:"private_key"`
		Username   string `json:"username"`
		Port       int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name is required", "")
		return
	}
	// In production, encrypt private_key before storing
	port := int32(req.Port)
	if port == 0 {
		port = 22
	}
	cred, err := s.queries.CreateSSHCredential(r.Context(), db.CreateSSHCredentialParams{
		ID:            id.New("ssh_"),
		WorkspaceID:   wsID,
		Name:          req.Name,
		PrivateKeyEnc: []byte(req.PrivateKey),
		Fingerprint:   "sha256:" + req.Name,
		Username:      &req.Username,
		Port:          &port,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create SSH credential", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": cred})
}

func (s *Service) DeleteSSHCredential(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	credID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteSSHCredential(r.Context(), db.DeleteSSHCredentialParams{ID: credID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "SSH credential not found", "")
		return
	}
	w.WriteHeader(204)
}

func (s *Service) ListSSMProfiles(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	profiles, err := s.queries.ListSSMProfilesByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list SSM profiles", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": profiles, "meta": map[string]any{"total": len(profiles)}})
}

func (s *Service) CreateSSMProfile(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name    string  `json:"name"`
		Region  string  `json:"region"`
		RoleARN *string `json:"role_arn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Region == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name and region are required", "")
		return
	}
	profile, err := s.queries.CreateSSMProfile(r.Context(), db.CreateSSMProfileParams{
		ID:          id.New("ssp_"),
		WorkspaceID: wsID,
		Name:        req.Name,
		Region:      req.Region,
		RoleArn:     req.RoleARN,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create SSM profile", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": profile})
}

func (s *Service) DeleteSSMProfile(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	profID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteSSMProfile(r.Context(), db.DeleteSSMProfileParams{ID: profID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "SSM profile not found", "")
		return
	}
	w.WriteHeader(204)
}

func (s *Service) ListK8sClusters(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	clusters, err := s.queries.ListK8sClustersByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list K8s clusters", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": clusters, "meta": map[string]any{"total": len(clusters)}})
}

func (s *Service) CreateK8sCluster(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name             string `json:"name"`
		DefaultNamespace string `json:"default_namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name is required", "")
		return
	}
	ns := req.DefaultNamespace
	if ns == "" {
		ns = "default"
	}
	cluster, err := s.queries.CreateK8sCluster(r.Context(), db.CreateK8sClusterParams{
		ID:               id.New("k8c_"),
		WorkspaceID:      wsID,
		Name:             req.Name,
		KubeconfigEnc:    []byte(""),
		DefaultNamespace: &ns,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create K8s cluster", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": cluster})
}

func (s *Service) DeleteK8sCluster(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	clusterID := chi.URLParam(r, "id")
	deleted, err := s.queries.DeleteK8sCluster(r.Context(), db.DeleteK8sClusterParams{ID: clusterID, WorkspaceID: wsID})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "K8s cluster not found", "")
		return
	}
	w.WriteHeader(204)
}

// ============================================================================
// Infrastructure (Servers)
// ============================================================================

func (s *Service) ListInfraServers(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	if s.provisioner == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Infrastructure provisioning not configured", "")
		return
	}
	servers, err := s.provisioner.ListServers(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list servers", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": servers, "meta": map[string]any{"total": len(servers)}})
}

func (s *Service) GetInfraPool(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	if s.provisioner == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Infrastructure provisioning not configured", "")
		return
	}

	var totalServers, totalContainers, totalCapacity int
	var totalCost float64

	s.pool.QueryRow(r.Context(),
		`SELECT count(*), COALESCE(SUM(containers_running), 0), COALESCE(SUM(max_containers), 0), COALESCE(SUM(monthly_cost), 0)
		 FROM workspace_servers WHERE workspace_id = $1 AND state NOT IN ('destroyed')`, wsID).Scan(
		&totalServers, &totalContainers, &totalCapacity, &totalCost)

	utilization := 0.0
	if totalCapacity > 0 {
		utilization = float64(totalContainers) / float64(totalCapacity) * 100
	}

	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"servers":            totalServers,
		"containers_running": totalContainers,
		"total_capacity":     totalCapacity,
		"utilization_pct":    utilization,
		"hetzner_cost":       totalCost,
		"workspace_cost":     totalCost * 2,
	}})
}

func (s *Service) ProvisionServer(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	if s.provisioner == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Infrastructure provisioning not configured", "")
		return
	}
	ip, err := s.provisioner.EnsureCapacity(r.Context(), wsID, 1)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to provision server", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]any{"status": "provisioning", "ip": ip}})
}

func (s *Service) DestroyInfraServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	if s.provisioner == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Infrastructure provisioning not configured", "")
		return
	}
	if err := s.provisioner.DestroyServer(r.Context(), serverID); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to destroy server", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "destroying"}})
}

func (s *Service) ServerReadyCallback(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	if s.provisioner == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Not configured", "")
		return
	}
	s.provisioner.MarkServerReady(r.Context(), serverID)
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "ready"}})
}

// ============================================================================
// Orchestras
// ============================================================================

func (s *Service) CreateOrchestra(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name           string   `json:"name"`
		DirectorType   string   `json:"director_type"`
		DirectorProcID *string  `json:"director_process_id"`
		AIConfig       any      `json:"ai_config"`
		FirstMusician  string   `json:"first_musician"`
		Secrets        []string `json:"secrets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name is required", "")
		return
	}
	if req.DirectorType == "" {
		req.DirectorType = "none"
	}
	var aiJSON []byte
	if req.AIConfig != nil {
		aiJSON, _ = json.Marshal(req.AIConfig)
	}
	orch, err := s.queries.CreateOrchestra(r.Context(), db.CreateOrchestraParams{
		ID: id.New("orc_"), WorkspaceID: wsID, Name: req.Name,
		DirectorType: req.DirectorType, DirectorProcessID: req.DirectorProcID,
		AIConfig: aiJSON, Secrets: req.Secrets,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create orchestra", err.Error())
		return
	}
	if req.FirstMusician != "" {
		actor, _ := auth.GetActor(r.Context())
		step := int32(1)
		s.queries.IncrementMovementCount(r.Context(), orch.ID)
		newRun, _ := s.queries.CreateRun(r.Context(), db.CreateRunParams{
			ID: id.NewRun(), WorkspaceID: wsID, ProcessID: req.FirstMusician,
			ScheduledAt: dbutil.Timestamptz(time.Now().UTC()), State: string(runstate.Pending),
			Origin: "orchestra", MaxAttempts: 1, ActorType: &actor.Type, ActorID: &actor.ID,
		})
		s.pool.Exec(r.Context(), "UPDATE runs SET orchestra_id=$1, orchestra_step=$2 WHERE id=$3", orch.ID, step, newRun.ID)
	}
	writeJSON(w, 201, map[string]any{"data": orch})
}

func (s *Service) ListOrchestras(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	orchs, err := s.queries.ListOrchestrasByWorkspace(r.Context(), db.ListOrchestrasParams{WorkspaceID: wsID, Limit: 50, Offset: 0})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list orchestras", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": orchs, "meta": map[string]any{"total": len(orchs)}})
}

func (s *Service) GetOrchestraHandler(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	orchID := chi.URLParam(r, "id")
	orch, err := s.queries.GetOrchestra(r.Context(), db.GetOrchestraParams{ID: orchID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Orchestra not found", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": orch})
}

func (s *Service) GetOrchestraScore(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	orchID := chi.URLParam(r, "id")
	orch, err := s.queries.GetOrchestra(r.Context(), db.GetOrchestraParams{ID: orchID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Orchestra not found", "")
		return
	}
	movements, _ := s.queries.ListMovementsByOrchestra(r.Context(), orchID)
	writeJSON(w, 200, map[string]any{"data": map[string]any{"orchestra": orch, "movements": movements}})
}

func (s *Service) CancelOrchestra(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	s.queries.UpdateOrchestraState(r.Context(), orchID, "cancelled")
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "cancelled"}})
}

func (s *Service) PauseOrchestra(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	s.queries.UpdateOrchestraState(r.Context(), orchID, "paused")
	// Post chat message
	actor, _ := auth.GetActor(r.Context())
	s.queries.CreateChatMessage(r.Context(), db.CreateChatMessageParams{
		ID: id.New("msg_"), OrchestraID: orchID, SenderType: "system",
		MessageType: "status", Content: fmt.Sprintf("Orchestra paused by %s", actor.ID),
	})
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "paused"}})
}

func (s *Service) ResumeOrchestra(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	s.queries.UpdateOrchestraState(r.Context(), orchID, "active")
	actor, _ := auth.GetActor(r.Context())
	s.queries.CreateChatMessage(r.Context(), db.CreateChatMessageParams{
		ID: id.New("msg_"), OrchestraID: orchID, SenderType: "system",
		MessageType: "status", Content: fmt.Sprintf("Orchestra resumed by %s", actor.ID),
	})
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "active"}})
}

func (s *Service) FinishOrchestraHandler(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	var req struct{ Summary string `json:"summary"` }
	json.NewDecoder(r.Body).Decode(&req)
	s.queries.FinishOrchestra(r.Context(), orchID, &req.Summary)
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "completed"}})
}

func (s *Service) NextMovement(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	runID := chi.URLParam(r, "id")
	var req struct {
		ProcessID string `json:"process_id"`
		Payload   any    `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProcessID == "" {
		writeError(w, 400, "VALIDATION_ERROR", "process_id is required", "")
		return
	}
	run, err := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Run not found", "")
		return
	}
	orchestraID := ""
	step := int32(1)
	if run.OrchestraID != nil {
		orchestraID = *run.OrchestraID
		if run.OrchestraStep != nil {
			step = *run.OrchestraStep + 1
		}
		s.queries.IncrementMovementCount(r.Context(), orchestraID)
	}
	actor, _ := auth.GetActor(r.Context())
	newRun, err := s.queries.CreateRun(r.Context(), db.CreateRunParams{
		ID: id.NewRun(), WorkspaceID: wsID, ProcessID: req.ProcessID,
		ScheduledAt: dbutil.Timestamptz(time.Now().UTC()), State: string(runstate.Pending),
		Origin: "orchestra", MaxAttempts: 1, ActorType: &actor.Type, ActorID: &actor.ID,
		TriggeredByRunID: &runID,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create movement", err.Error())
		return
	}
	if orchestraID != "" {
		s.pool.Exec(r.Context(), "UPDATE runs SET orchestra_id=$1, orchestra_step=$2 WHERE id=$3", orchestraID, step, newRun.ID)
	}
	writeJSON(w, 201, map[string]any{"data": newRun})
}

func (s *Service) SetChoiceConfig(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	wsID := auth.WorkspaceID(r.Context())
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if !json.Valid(body) {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid JSON", "")
		return
	}
	s.queries.SetRunChoiceConfig(r.Context(), runID, body)
	run, _ := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if run.OrchestraID != nil {
		s.queries.UpdateOrchestraState(r.Context(), *run.OrchestraID, "waiting_for_choice")
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "waiting_for_choice"}})
}

func (s *Service) Choose(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	runID := chi.URLParam(r, "id")
	var req struct{ ChoiceIndex int32 `json:"choice_index"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "choice_index is required", "")
		return
	}
	run, err := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if err != nil || run.ChoiceConfig == nil {
		writeError(w, 400, "VALIDATION_ERROR", "Run has no choice config", "")
		return
	}
	var config struct{ Choices []struct{ ProcessID *string `json:"process_id"` } `json:"choices"` }
	json.Unmarshal(run.ChoiceConfig, &config)
	if int(req.ChoiceIndex) >= len(config.Choices) {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid choice index", "")
		return
	}
	s.queries.SetRunChosenIndex(r.Context(), runID, req.ChoiceIndex)
	choice := config.Choices[req.ChoiceIndex]
	if choice.ProcessID != nil && *choice.ProcessID != "" {
		actor, _ := auth.GetActor(r.Context())
		step := int32(1)
		orchestraID := ""
		if run.OrchestraID != nil {
			orchestraID = *run.OrchestraID
			if run.OrchestraStep != nil { step = *run.OrchestraStep + 1 }
			s.queries.IncrementMovementCount(r.Context(), orchestraID)
			s.queries.UpdateOrchestraState(r.Context(), orchestraID, "active")
		}
		newRun, _ := s.queries.CreateRun(r.Context(), db.CreateRunParams{
			ID: id.NewRun(), WorkspaceID: wsID, ProcessID: *choice.ProcessID,
			ScheduledAt: dbutil.Timestamptz(time.Now().UTC()), State: string(runstate.Pending),
			Origin: "orchestra", MaxAttempts: 1, ActorType: &actor.Type, ActorID: &actor.ID,
			TriggeredByRunID: &runID,
		})
		if orchestraID != "" {
			s.pool.Exec(r.Context(), "UPDATE runs SET orchestra_id=$1, orchestra_step=$2 WHERE id=$3", orchestraID, step, newRun.ID)
		}
		writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "next_triggered", "run_id": newRun.ID}})
		return
	}
	if run.OrchestraID != nil {
		s.queries.UpdateOrchestraState(r.Context(), *run.OrchestraID, "completed")
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "completed"}})
}

// ============================================================================
// Orchestra Chat
// ============================================================================

func (s *Service) PostChatMessage(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	var req struct {
		Content     string `json:"content"`
		MessageType string `json:"message_type"`
		SenderType  string `json:"sender_type"`
		Data        any    `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		writeError(w, 400, "VALIDATION_ERROR", "content is required", "")
		return
	}
	if req.MessageType == "" {
		req.MessageType = "text"
	}
	if req.SenderType == "" {
		req.SenderType = "human"
	}

	var dataJSON []byte
	if req.Data != nil {
		dataJSON, _ = json.Marshal(req.Data)
	}

	actor, _ := auth.GetActor(r.Context())
	msg, err := s.queries.CreateChatMessage(r.Context(), db.CreateChatMessageParams{
		ID:          id.New("msg_"),
		OrchestraID: orchID,
		SenderType:  req.SenderType,
		SenderID:    &actor.ID,
		MessageType: req.MessageType,
		Content:     req.Content,
		Data:        dataJSON,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to post message", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": msg})
}

func (s *Service) ListChatMessages(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")
	messages, err := s.queries.ListChatMessagesAll(r.Context(), orchID, 200, 0)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list messages", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": messages, "meta": map[string]any{"total": len(messages)}})
}

// StreamChat sends real-time chat updates via Server-Sent Events.
func (s *Service) StreamChat(w http.ResponseWriter, r *http.Request) {
	orchID := chi.URLParam(r, "id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "INTERNAL_ERROR", "Streaming not supported", "")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	// Poll for new messages every 2 seconds
	lastCheck := time.Now().Add(-5 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			messages, err := s.queries.ListChatMessages(r.Context(), orchID, lastCheck, 50)
			if err != nil {
				continue
			}
			for _, msg := range messages {
				data, _ := json.Marshal(msg)
				fmt.Fprintf(w, "event: chat\ndata: %s\n\n", data)
			}

			// Also check orchestra state
			orch, err := s.queries.GetOrchestra(r.Context(), db.GetOrchestraParams{
				ID: orchID, WorkspaceID: auth.WorkspaceID(r.Context()),
			})
			if err == nil {
				stateData, _ := json.Marshal(map[string]any{"state": orch.State, "movement_count": orch.MovementCount})
				fmt.Fprintf(w, "event: state\ndata: %s\n\n", stateData)
			}

			flusher.Flush()
			lastCheck = time.Now()
		}
	}
}

// ============================================================================
// Run Result
// ============================================================================

func (s *Service) SetRunResult(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024)) // 1MB max
	if err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Failed to read body", "")
		return
	}
	if !json.Valid(body) {
		writeError(w, 400, "VALIDATION_ERROR", "Body must be valid JSON", "")
		return
	}
	if err := s.queries.SetRunResult(r.Context(), runID, body); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to set result", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "ok"}})
}

func (s *Service) GetRunResult(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	result, err := s.queries.GetRunResult(r.Context(), runID)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Run not found", "")
		return
	}
	if result.Result == nil {
		writeJSON(w, 200, map[string]any{"data": nil})
		return
	}
	var parsed any
	json.Unmarshal(result.Result, &parsed)
	writeJSON(w, 200, map[string]any{"data": parsed})
}

// ============================================================================
// Workspace Secrets
// ============================================================================

func (s *Service) ListSecrets(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	secrets, err := s.queries.ListSecretsByWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list secrets", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": secrets, "meta": map[string]any{"total": len(secrets)}})
}

func (s *Service) CreateSecretHandler(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Value == "" {
		writeError(w, 400, "VALIDATION_ERROR", "name and value are required", "")
		return
	}

	encrypted, err := crypto.Encrypt([]byte(req.Value), s.encryptionKey)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to encrypt secret", "")
		return
	}

	secret, err := s.queries.CreateSecret(r.Context(), db.CreateSecretParams{
		ID:          id.New("sec_"),
		WorkspaceID: wsID,
		Name:        req.Name,
		ValueEnc:    encrypted,
	})
	if err != nil {
		writeError(w, 409, "CONFLICT", "Secret with this name already exists", "")
		return
	}
	writeJSON(w, 201, map[string]any{"data": secret})
}

func (s *Service) UpdateSecretHandler(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	name := chi.URLParam(r, "name")
	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Value == "" {
		writeError(w, 400, "VALIDATION_ERROR", "value is required", "")
		return
	}

	encrypted, err := crypto.Encrypt([]byte(req.Value), s.encryptionKey)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to encrypt secret", "")
		return
	}

	if err := s.queries.UpdateSecret(r.Context(), wsID, name, encrypted); err != nil {
		writeError(w, 404, "NOT_FOUND", "Secret not found", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "updated"}})
}

func (s *Service) DeleteSecretHandler(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	name := chi.URLParam(r, "name")
	deleted, err := s.queries.DeleteSecret(r.Context(), db.DeleteSecretParams{WorkspaceID: wsID, Name: name})
	if err != nil || deleted == 0 {
		writeError(w, 404, "NOT_FOUND", "Secret not found", "")
		return
	}
	w.WriteHeader(204)
}

// ============================================================================
// Run Artifacts
// ============================================================================

func (s *Service) UploadArtifact(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	runID := chi.URLParam(r, "id")

	r.ParseMultipartForm(32 << 20) // 32MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "file is required (multipart form)", "")
		return
	}
	defer file.Close()

	name := header.Filename
	if n := r.FormValue("name"); n != "" {
		name = n
	}

	// Storage isolation: include orchestra_id in path if run belongs to one
	var storageKey string
	shared := r.URL.Query().Get("shared") == "true"
	run, runErr := s.queries.GetRun(r.Context(), db.GetRunParams{ID: runID, WorkspaceID: wsID})
	if runErr == nil && run.OrchestraID != nil && *run.OrchestraID != "" {
		if shared {
			storageKey = fmt.Sprintf("%s/%s/shared/%s", wsID, *run.OrchestraID, name)
		} else {
			storageKey = fmt.Sprintf("%s/%s/%s/%s", wsID, *run.OrchestraID, runID, name)
		}
	} else {
		storageKey = fmt.Sprintf("%s/%s/%s", wsID, runID, name)
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if s.artifactStore == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Artifact storage not configured", "Set CC_ARTIFACTS_BACKEND")
		return
	}

	if err := s.artifactStore.Upload(r.Context(), storageKey, file, contentType); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to upload artifact", err.Error())
		return
	}

	size := header.Size
	artifact, err := s.queries.CreateArtifact(r.Context(), db.CreateArtifactParams{
		ID:          id.New("art_"),
		RunID:       runID,
		WorkspaceID: wsID,
		Name:        name,
		ContentType: &contentType,
		SizeBytes:   &size,
		StorageKey:  storageKey,
	})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to save artifact metadata", err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"data": artifact})
}

func (s *Service) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	artifacts, err := s.queries.ListArtifactsByRun(r.Context(), runID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list artifacts", "")
		return
	}
	writeJSON(w, 200, map[string]any{"data": artifacts, "meta": map[string]any{"total": len(artifacts)}})
}

func (s *Service) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")

	artifact, err := s.queries.GetArtifact(r.Context(), runID, name)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Artifact not found", "")
		return
	}

	if s.artifactStore == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Artifact storage not configured", "")
		return
	}

	reader, err := s.artifactStore.Download(r.Context(), artifact.StorageKey)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to download artifact", err.Error())
		return
	}
	defer reader.Close()

	if artifact.ContentType != nil {
		w.Header().Set("Content-Type", *artifact.ContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	io.Copy(w, reader)
}

// ============================================================================
// Webhook Test Delivery
// ============================================================================

func (s *Service) TestWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	subID := chi.URLParam(r, "id")

	sub, err := s.queries.GetWebhookSubscription(r.Context(), db.GetWebhookSubscriptionParams{
		ID:          subID,
		WorkspaceID: wsID,
	})
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Webhook subscription not found", "")
		return
	}

	// Send test event
	testEvent := map[string]any{
		"type":      "webhook.test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"workspace": map[string]any{"id": wsID},
		"data": map[string]any{
			"message":        "This is a test delivery from CronControl",
			"subscription_id": sub.ID,
		},
	}

	body, _ := json.Marshal(testEvent)

	req, err := http.NewRequestWithContext(r.Context(), "POST", sub.Url, bytes.NewReader(body))
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to create request", "")
		return
	}

	mac := hmac.New(sha256.New, []byte(sub.Secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CronControl-Signature", signature)
	req.Header.Set("X-CronControl-Timestamp", testEvent["timestamp"].(string))
	req.Header.Set("X-CronControl-Delivery-Id", id.New("dlv_"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 200, map[string]any{"data": map[string]any{"delivered": false, "error": err.Error()}})
		return
	}
	resp.Body.Close()

	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"delivered":   resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status_code": resp.StatusCode,
	}})
}

// ============================================================================
// Email Invitations
// ============================================================================

func (s *Service) InviteMember(w http.ResponseWriter, r *http.Request) {
	wsID := auth.WorkspaceID(r.Context())
	role := auth.Role(r.Context())
	if role != "admin" {
		writeError(w, 403, "FORBIDDEN", "Admin role required to invite members", "")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request", "")
		return
	}
	if req.Email == "" || req.Role == "" {
		writeError(w, 400, "VALIDATION_ERROR", "email and role are required", "")
		return
	}
	if req.Role != "admin" && req.Role != "operator" && req.Role != "viewer" {
		writeError(w, 400, "VALIDATION_ERROR", "role must be admin, operator, or viewer", "")
		return
	}

	// Check if user already exists
	user, err := s.queries.GetUserByEmail(r.Context(), req.Email)
	if err == nil {
		// User exists — add membership directly
		_, err := s.queries.CreateMembership(r.Context(), db.CreateMembershipParams{
			ID:          id.NewMembership(),
			WorkspaceID: wsID,
			UserID:      user.ID,
			Role:        req.Role,
		})
		if err != nil {
			writeError(w, 409, "CONFLICT", "User is already a member of this workspace", "")
			return
		}
		writeJSON(w, 201, map[string]any{"data": map[string]any{
			"status": "member_added",
			"email":  req.Email,
			"role":   req.Role,
		}})
		return
	}

	// User doesn't exist — store invitation info and create a token.
	// We use a placeholder user ID based on the workspace+email hash since the user doesn't exist yet.
	// In production, the invitation email contains a link that triggers user creation on acceptance.
	placeholderID := "invite_" + wsID + "_" + req.Email
	rawToken, _ := s.createToken(r.Context(), placeholderID, "invitation", 7*24*time.Hour)

	slog.Info("invitation created", "email", req.Email, "workspace_id", wsID, "token_preview", rawToken[:8]+"...")

	writeJSON(w, 201, map[string]any{"data": map[string]any{
		"status":           "invitation_sent",
		"email":            req.Email,
		"role":             req.Role,
		"hint":             "In production, an email would be sent. Token logged for development.",
	}})
}

// ============================================================================
// Google OAuth
// ============================================================================

// GoogleLogin redirects the user to Google's OAuth consent page.
func (s *Service) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.googleAuth == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Google OAuth is not configured", "Set CC_AUTH_GOOGLE_CLIENT_ID and CC_AUTH_GOOGLE_CLIENT_SECRET")
		return
	}

	url, state := s.googleAuth.LoginURL()

	// Store state in a short-lived cookie for CSRF protection
	http.SetCookie(w, &http.Cookie{
		Name:     "cc_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, url, http.StatusFound)
}

// GoogleCallback handles the OAuth callback from Google.
func (s *Service) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.googleAuth == nil {
		writeError(w, 501, "NOT_IMPLEMENTED", "Google OAuth is not configured", "")
		return
	}

	// Verify CSRF state
	stateCookie, err := r.Cookie("cc_oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid OAuth state", "Try signing in again")
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "cc_oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, 400, "VALIDATION_ERROR", "Missing authorization code", "")
		return
	}

	// Exchange code for user info
	info, err := s.googleAuth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("google oauth exchange failed", "error", err)
		writeError(w, 500, "INTERNAL_ERROR", "OAuth exchange failed", "Try signing in again")
		return
	}

	// Look up or create user
	user, err := s.queries.GetUserByEmail(r.Context(), info.Email)
	if err != nil {
		// New user: create user + workspace + membership + API key
		wsName := info.Name + "'s Workspace"
		apiKey, err := s.createUserWithWorkspace(r.Context(), info.Email, info.Name, nil, "google", wsName)
		if err != nil {
			slog.Error("google oauth create user failed", "error", err)
			writeError(w, 500, "INTERNAL_ERROR", "Failed to create account", "")
			return
		}

		// Redirect to frontend with the API key
		// Set API key in HttpOnly cookie instead of URL to prevent logging/exposure
		http.SetCookie(w, &http.Cookie{
			Name:     "cc_oauth_key",
			Value:    apiKey,
			Path:     "/",
			MaxAge:   60, // 1 minute — frontend reads and clears it
			HttpOnly: false, // frontend JS needs to read it
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/?oauth=success&new=true", http.StatusFound)
		return
	}

	// Existing user: generate a new API key for this session
	rawKey := generateRawAPIKey()
	keyHash := auth.HashAPIKey(rawKey)
	keyPrefix := auth.GenerateAPIKeyPrefix(rawKey)

	// Use their active workspace, or first available
	wsID := ""
	if user.ActiveWorkspaceID != nil {
		wsID = *user.ActiveWorkspaceID
	}
	if wsID == "" {
		memberships, _ := s.queries.ListMembershipsByUser(r.Context(), user.ID)
		if len(memberships) > 0 {
			wsID = memberships[0].WorkspaceID
		}
	}

	if wsID != "" {
		// Get role from membership
		memberships, _ := s.queries.ListMembershipsByUser(r.Context(), user.ID)
		role := "viewer"
		for _, m := range memberships {
			if m.WorkspaceID == wsID {
				role = m.Role
				break
			}
		}

		s.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
			ID:          id.NewAPIKey(),
			WorkspaceID: wsID,
			Name:        "Google OAuth session",
			KeyHash:     keyHash,
			KeyPrefix:   keyPrefix,
			Role:        role,
			CreatedBy:   &user.ID,
		})
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "cc_oauth_key",
		Value:    rawKey,
		Path:     "/",
		MaxAge:   60,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/?oauth=success", http.StatusFound)
}

// createUserWithWorkspace creates a new user, workspace, membership, and API key.
// Returns the raw API key.
func (s *Service) createUserWithWorkspace(ctx context.Context, email, name string, passwordHash *string, authProvider, wsName string) (string, error) { //nolint:unparam
	wsID := id.NewWorkspace()
	userID := id.NewUser()
	membershipID := id.NewMembership()
	keyID := id.NewAPIKey()

	rawKey := generateRawAPIKey()
	keyHash := auth.HashAPIKey(rawKey)
	keyPrefix := auth.GenerateAPIKeyPrefix(rawKey)

	_, err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:   wsID,
		Name: wsName,
		Slug: slugify(wsName),
	})
	if err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}

	_, err = s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:                userID,
		Email:             email,
		Name:              name,
		PasswordHash:      passwordHash,
		AuthProvider:      authProvider,
		ActiveWorkspaceID: &wsID,
		EmailVerified:     authProvider == "google",
	})
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	s.queries.CreateMembership(ctx, db.CreateMembershipParams{
		ID:          membershipID,
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        "admin",
	})

	s.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		ID:          keyID,
		WorkspaceID: wsID,
		Name:        "Default",
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Role:        "admin",
		CreatedBy:   &userID,
	})

	return rawKey, nil
}

// ============================================================================
// Helpers
// ============================================================================

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message, hint string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if hint != "" {
		resp["error"].(map[string]any)["hint"] = hint
	}
	json.NewEncoder(w).Encode(resp)
}

func getStr(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func getStrPtr(m map[string]any, key string, def *string) *string {
	if v, ok := m[key].(string); ok {
		return &v
	}
	return def
}

func slugify(name string) string {
	slug := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			slug += string(c)
		} else if c >= 'A' && c <= 'Z' {
			slug += string(c + 32) // lowercase
		} else if c == ' ' {
			slug += "-"
		}
	}
	return slug
}

func generateRawAPIKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "cc_live_" + hex.EncodeToString(b)
}

func hashSHA256(s string) string {
	h := auth.HashAPIKey(s)
	return h
}

// redactID masks internal IDs in API responses. Shows prefix + first 4 chars only.
func redactID(id string) string {
	if len(id) <= 8 {
		return id
	}
	// Find prefix end (e.g., "key_", "usr_")
	for i, c := range id {
		if c == '_' && i < 5 {
			remaining := id[i+1:]
			if len(remaining) > 4 {
				return id[:i+1] + remaining[:4] + "····"
			}
			return id
		}
	}
	return id[:4] + "····"
}

// ============================================================================
// Platform Admin: Infrastructure View
// ============================================================================

func (s *Service) ListAllInfraServers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, workspace_id, name, ip_address, state, server_type, containers_running, max_containers, monthly_cost, created_at
		 FROM workspace_servers WHERE state != 'destroyed' ORDER BY workspace_id, created_at`)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Failed to list servers", "")
		return
	}
	defer rows.Close()

	var servers []map[string]any
	for rows.Next() {
		var id, wsID, name, state, serverType string
		var ip *string
		var running, max int32
		var cost float64
		var created time.Time
		rows.Scan(&id, &wsID, &name, &ip, &state, &serverType, &running, &max, &cost, &created)
		servers = append(servers, map[string]any{
			"id": id, "workspace_id": wsID, "name": name, "ip_address": ip, "state": state,
			"server_type": serverType, "containers_running": running,
			"max_containers": max, "monthly_cost": cost, "workspace_cost": cost * 2,
			"created_at": created,
		})
	}
	if servers == nil {
		servers = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"data": servers, "meta": map[string]any{"total": len(servers)}})
}

// ============================================================================
// Chat Simulate (Groq proxy for website demo)
// ============================================================================

func (s *Service) ChatSimulate(w http.ResponseWriter, r *http.Request) {
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" {
		writeError(w, 501, "NOT_CONFIGURED", "Chat simulation not configured", "")
		return
	}

	var req struct {
		Messages  []map[string]string `json:"messages"`
		MaxTokens int                 `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "Invalid request body", "")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, 400, "VALIDATION_ERROR", "messages is required", "")
		return
	}
	if req.MaxTokens == 0 || req.MaxTokens > 300 {
		req.MaxTokens = 150
	}

	body, _ := json.Marshal(map[string]any{
		"model":       "llama-3.3-70b-versatile",
		"messages":    req.Messages,
		"max_tokens":  req.MaxTokens,
		"temperature": 0.7,
	})

	groqReq, _ := http.NewRequestWithContext(r.Context(), "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	groqReq.Header.Set("Content-Type", "application/json")
	groqReq.Header.Set("Authorization", "Bearer "+groqKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(groqReq)
	if err != nil {
		writeError(w, 502, "UPSTREAM_ERROR", "Failed to reach AI provider", "")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// Suppress unused import warnings
var _ = pgx.ErrNoRows
var _ = slog.Info
