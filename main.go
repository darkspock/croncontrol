package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncontrol/croncontrol/config"
	"github.com/croncontrol/croncontrol/internal/auth"
	"github.com/croncontrol/croncontrol/internal/cleanup"
	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dependency"
	"github.com/croncontrol/croncontrol/internal/executor"
	exechttp "github.com/croncontrol/croncontrol/internal/executor/http"
	execk8s "github.com/croncontrol/croncontrol/internal/executor/k8s"
	execssh "github.com/croncontrol/croncontrol/internal/executor/ssh"
	execssm "github.com/croncontrol/croncontrol/internal/executor/ssm"
	"github.com/croncontrol/croncontrol/internal/frontend"
	"github.com/croncontrol/croncontrol/internal/handler"
	"github.com/croncontrol/croncontrol/internal/logging"
	logdb "github.com/croncontrol/croncontrol/internal/logging/database"
	logfile "github.com/croncontrol/croncontrol/internal/logging/file"
	logos "github.com/croncontrol/croncontrol/internal/logging/opensearch"
	"github.com/croncontrol/croncontrol/internal/metrics"
	"github.com/croncontrol/croncontrol/internal/monitor"
	"github.com/croncontrol/croncontrol/internal/notifier"
	orchestramonitor "github.com/croncontrol/croncontrol/internal/orchestra"
	"github.com/croncontrol/croncontrol/internal/planner"
	"github.com/croncontrol/croncontrol/internal/queue"
	"github.com/croncontrol/croncontrol/internal/recovery"
	"github.com/croncontrol/croncontrol/internal/storage"
	"github.com/croncontrol/croncontrol/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	slog.Info("starting CronControl",
		"version", version,
		"port", cfg.Server.Port,
	)

	// Database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.URL())
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	slog.Info("database connected")

	queries := db.New(pool)

	// Bootstrap platform admin from config/env (CC_SAAS_PLATFORM_ADMIN_EMAIL)
	if adminEmail := cfg.SaaS.PlatformAdminEmail; adminEmail != "" {
		user, err := queries.GetUserByEmail(ctx, adminEmail)
		if err == nil && !user.IsPlatformAdmin {
			queries.SetPlatformAdmin(ctx, db.SetPlatformAdminParams{ID: user.ID, IsPlatformAdmin: true})
			slog.Info("platform admin bootstrapped", "email", adminEmail)
		} else if err == nil {
			slog.Info("platform admin already set", "email", adminEmail)
		} else {
			slog.Warn("platform admin email not found — register first, then restart", "email", adminEmail)
		}
	}

	// Initialize logging backend based on config (CC_LOGGING_BACKEND: database, file, opensearch)
	var logBackend logging.Backend
	switch cfg.Logging.Backend {
	case "file":
		path := cfg.Logging.FilePath
		if path == "" {
			path = "./data/logs"
		}
		fb, err := logfile.New(path)
		if err != nil {
			return fmt.Errorf("init file logging backend: %w", err)
		}
		logBackend = fb
		slog.Info("logging backend: file", "path", path)
	case "opensearch":
		osURL := cfg.Logging.OpenSearchURL
		if osURL == "" {
			osURL = "http://localhost:9200"
		}
		var user, pass string
		if cfg.Logging.OpenSearchAuth != "" {
			parts := splitOnce(cfg.Logging.OpenSearchAuth, ":")
			user, pass = parts[0], parts[1]
		}
		logBackend = logos.New(logos.Config{URL: osURL, Username: user, Password: pass})
		slog.Info("logging backend: opensearch", "url", osURL)
	default:
		logBackend = logdb.New(pool)
		slog.Info("logging backend: database")
	}

	// Execution method registry
	registry := executor.NewRegistry()
	registry.Register("http", exechttp.New(5*1024*1024)) // 5MB max response
	registry.Register("ssh", execssh.New(func(ctx context.Context, credentialID string) ([]byte, string, int, bool, error) {
		var privateKey []byte
		var username *string
		var port *int32
		var strict bool
		err := pool.QueryRow(ctx, `
			SELECT private_key_enc, username, port, strict_host_key
			FROM ssh_credentials
			WHERE id = $1
		`, credentialID).Scan(&privateKey, &username, &port, &strict)
		if err != nil {
			return nil, "", 0, false, err
		}
		user := ""
		if username != nil {
			user = *username
		}
		p := 22
		if port != nil && *port > 0 {
			p = int(*port)
		}
		return privateKey, user, p, strict, nil
	}))
	registry.Register("ssm", execssm.New(func(ctx context.Context, profileID string) (string, string, error) {
		var region string
		var roleARN *string
		err := pool.QueryRow(ctx, `
			SELECT region, role_arn
			FROM ssm_profiles
			WHERE id = $1
		`, profileID).Scan(&region, &roleARN)
		if err != nil {
			return "", "", err
		}
		role := ""
		if roleARN != nil {
			role = *roleARN
		}
		return region, role, nil
	}))
	registry.Register("k8s", execk8s.New(func(ctx context.Context, clusterID string) ([]byte, string, error) {
		var kubeconfig []byte
		var defaultNamespace *string
		err := pool.QueryRow(ctx, `
			SELECT kubeconfig_enc, default_namespace
			FROM k8s_clusters
			WHERE id = $1
		`, clusterID).Scan(&kubeconfig, &defaultNamespace)
		if err != nil {
			return nil, "", err
		}
		ns := ""
		if defaultNamespace != nil {
			ns = *defaultNamespace
		}
		return kubeconfig, ns, nil
	}))

	// Components
	depResolver := dependency.New(queries)
	webhookNotifier := notifier.New(queries)

	plannerInterval := parseDurationOrDefault(cfg.Planner.Interval, time.Hour)
	plannerHorizon := parseDurationOrDefault(cfg.Planner.Horizon, 24*time.Hour)
	executorInterval := parseDurationOrDefault(cfg.Executor.Interval, 30*time.Second)
	monitorInterval := parseDurationOrDefault(cfg.Monitor.Interval, 30*time.Second)

	plannerComp := planner.New(pool, plannerInterval, plannerHorizon)
	orchestrator := executor.NewOrchestrator(pool, registry, executorInterval)
	orchestrator.SetLogBackend(logBackend)
	queueProc := queue.NewProcessor(pool, registry, 5*time.Second)
	queueProc.SetLogBackend(logBackend)
	workerDisp := worker.NewDispatcher(pool)

	monitorComp := monitor.New(pool, monitorInterval,
		func(runID string) error { return orchestrator.Kill(runID) },
		func(ctx context.Context, run db.ListRunningRunsRow, reason string) {
			webhookNotifier.Emit(ctx, run.WorkspaceID, notifier.Event{
				Type: "run.hung",
				Data: map[string]any{"run_id": run.ID, "reason": reason},
			})
		},
	)

	cleanupComp := cleanup.New(pool, cleanup.Config{
		RunRetention:   parseDurationOrDefault(cfg.Retention.Slots, 30*24*time.Hour),
		JobRetention:   parseDurationOrDefault(cfg.Retention.Jobs, 30*24*time.Hour),
		AuditRetention: parseDurationOrDefault(cfg.Retention.Audit, 90*24*time.Hour),
		BatchSize:      int32(cfg.Retention.CleanupBatchSize),
		Interval:       24 * time.Hour,
	})

	// Startup recovery
	slog.Info("running startup recovery...")
	if err := recovery.Run(ctx, queries); err != nil {
		slog.Error("recovery failed", "error", err)
		// non-fatal: continue starting
	}

	baseURL := cfg.SaaS.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	}

	// Wire dependency resolver into executor
	orchestrator.SetOnRunTerminal(func(ctx context.Context, run db.Run, proc db.Process) {
		depResolver.Evaluate(ctx, run)
	})
	orchestrator.SetWorkerDispatch(func(ctx context.Context, run db.Run, proc db.Process) error {
		var methodConfig map[string]any
		if len(proc.MethodConfig) > 0 {
			_ = json.Unmarshal(proc.MethodConfig, &methodConfig)
		}
		if methodConfig == nil {
			methodConfig = make(map[string]any)
		}

		// Resolve credential references into inline config for worker dispatch.
		// Workers cannot access the control plane database, so credentials must be
		// injected into the method_config before sending.
		if err := resolveCredentialsForWorker(ctx, pool, proc.ExecutionMethod, methodConfig); err != nil {
			slog.Error("worker dispatch: failed to resolve credentials", "method", proc.ExecutionMethod, "error", err)
			return err
		}

		var environment map[string]any
		if len(proc.Environment) > 0 {
			_ = json.Unmarshal(proc.Environment, &environment)
		}

		workerID, err := workerDisp.Dispatch(ctx, proc.WorkspaceID, worker.Task{
			ID:              run.ID,
			Type:            "run",
			WorkspaceID:     proc.WorkspaceID,
			ExecutionMethod: proc.ExecutionMethod,
			MethodConfig:    methodConfig,
			Environment:     environment,
			APIBaseURL:      baseURL,
		}, proc.WorkerID, proc.WorkerLabels)
		if err != nil {
			return err
		}

		if workerID == "" {
			return queries.UpdateRunState(ctx, db.UpdateRunStateParams{
				ID:            run.ID,
				State:         "waiting_for_worker",
				WaitingReason: strPtr("Waiting for available worker"),
			})
		}

		return queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:            run.ID,
			State:         "waiting_for_worker",
			WaitingReason: strPtr("Assigned to worker " + workerID),
			WorkerID:      &workerID,
		})
	})
	queueProc.SetWorkerDispatch(func(ctx context.Context, job db.ClaimPendingJobsRow) error {
		methodConfig := queue.BuildJobMethodConfig(job.QueueMethodConfig, job.Payload)

		// Resolve credential references for worker dispatch
		if err := resolveCredentialsForWorker(ctx, pool, job.ExecutionMethod, methodConfig); err != nil {
			slog.Error("worker dispatch: failed to resolve job credentials", "method", job.ExecutionMethod, "error", err)
			return err
		}

		selectedWorkerID, err := workerDisp.Dispatch(ctx, job.WorkspaceID, worker.Task{
			ID:              job.ID,
			Type:            "job",
			WorkspaceID:     job.WorkspaceID,
			ExecutionMethod: job.ExecutionMethod,
			MethodConfig:    methodConfig,
			APIBaseURL:      baseURL,
		}, firstNonEmptyPtr(job.WorkerIDOverride, job.QueueWorkerID), job.WorkerLabelsOverride)
		if err != nil {
			return err
		}

		if selectedWorkerID == "" {
			return queries.UpdateJobState(ctx, db.UpdateJobStateParams{
				ID:            job.ID,
				State:         "waiting_for_worker",
				WaitingReason: strPtr("Waiting for available worker"),
			})
		}

		attempt := job.Attempt + 1
		if attempt == 1 {
			snapshot, _ := json.Marshal(map[string]any{
				"method_config":    methodConfig,
				"execution_method": job.ExecutionMethod,
			})
			_ = queries.SnapshotJobConfig(ctx, db.SnapshotJobConfigParams{
				ID:              job.ID,
				EffectiveConfig: snapshot,
			})
		}

		return queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:            job.ID,
			State:         "waiting_for_worker",
			Attempt:       &attempt,
			WaitingReason: strPtr("Assigned to worker " + selectedWorkerID),
			WorkerID:      &selectedWorkerID,
		})
	})

	// Metrics collector
	metricsCollector := metrics.NewCollector(queries, 30*time.Second)

	// Start background components
	plannerComp.Start(ctx)
	orchestrator.Start(ctx)
	queueProc.Start(ctx)
	monitorComp.Start(ctx)
	metricsCollector.Start(ctx)
	cleanupComp.Start(ctx)
	workerDisp.Start(ctx)
	orchMonitor := orchestramonitor.NewMonitor(pool, 30*time.Second)
	orchMonitor.Start(ctx)

	// Handler service
	svc := handler.NewService(queries, pool, orchestrator, depResolver, webhookNotifier, workerDisp)

	// Configure Google OAuth if credentials are set
	googleAuth := auth.NewGoogleAuth(auth.GoogleOAuthConfig{
		ClientID:     cfg.Auth.GoogleClientID,
		ClientSecret: cfg.Auth.GoogleClientSecret,
		RedirectURL:  baseURL + "/api/v1/auth/google/callback",
	})
	svc.SetGoogleAuth(googleAuth)
	if googleAuth != nil {
		slog.Info("google oauth enabled")
	}

	// Encryption key for secrets
	svc.SetEncryptionKey([]byte(cfg.Auth.EncryptionKey))

	// Artifact storage (local by default)
	localStore, err := storage.NewLocalBackend("./data/artifacts")
	if err != nil {
		slog.Warn("artifact storage not available", "error", err)
	} else {
		svc.SetArtifactStore(localStore)
		slog.Info("artifact storage: local", "path", "./data/artifacts")
	}

	// Router
	r := buildRouter(cfg, pool, queries, svc)

	// Server
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	shutdownCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	<-shutdownCtx.Done()
	slog.Info("shutting down...")

	// Stop background components
	plannerComp.Stop()
	orchestrator.Stop()
	queueProc.Stop()
	monitorComp.Stop()
	cleanupComp.Stop()
	workerDisp.Stop()
	metricsCollector.Stop()

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	slog.Info("shutdown complete")
	return nil
}

func buildRouter(cfg *config.Config, pool *pgxpool.Pool, queries *db.Queries, svc *handler.Service) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	httpLogger := httplog.NewLogger("croncontrol", httplog.Options{
		JSON:            true,
		Concise:         true,
		RequestHeaders:  true,
		TimeFieldFormat: time.RFC3339,
	})

	r.Use(middleware.RequestID)
	r.Use(httplog.RequestLogger(httpLogger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://croncontrol.dev", "https://app.croncontrol.dev", "http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Prometheus metrics
	r.Use(metrics.Middleware)
	r.Handle("/metrics", promhttp.Handler())

	// Health (unauthenticated)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		if err := pool.Ping(r.Context()); err != nil {
			status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"status":  status,
			"version": version,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Auth middleware
	apiKeyAuth := auth.NewAPIKeyAuth(queries)
	skipPaths := map[string]bool{
		"/health":                      true,
		"/api/v1/heartbeat":            true,
		"/api/v1/workers/poll":         true,
		"/api/v1/workers/result":       true,
		"/api/v1/workers/heartbeat":    true,
		"/api/v1/workers/control-poll": true,
		"/api/v1/register":             true,
		"/api/v1/register/verify":      true,
		"/api/v1/login":                true,
	}

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// OpenAPI spec (unauthenticated)
		r.Get("/openapi.yaml", handler.ServeOpenAPI)

		// Unauthenticated routes
		r.Get("/config", svc.GetPublicConfig)
		r.Post("/register", svc.Register)
		r.Post("/login", svc.Login)
		r.Post("/heartbeat", svc.Heartbeat)
		r.Post("/workers/enroll", svc.EnrollWorker)
		r.Get("/workers/poll", svc.WorkerPoll)
		r.Post("/workers/result", svc.WorkerResult)
		r.Post("/workers/heartbeat", svc.WorkerHeartbeat)
		r.Post("/workers/control-poll", svc.WorkerControlPoll)
		r.Post("/infra/servers/{id}/ready", svc.ServerReadyCallback)
		r.Post("/chat/simulate", svc.ChatSimulate)
		r.Post("/register/verify", svc.VerifyEmail)
		r.Post("/register/resend", svc.ResendVerification)
		r.Post("/auth/forgot-password", svc.ForgotPassword)
		r.Post("/auth/reset-password", svc.ResetPassword)
		r.Get("/auth/google/login", svc.GoogleLogin)
		r.Get("/auth/google/callback", svc.GoogleCallback)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(apiKeyAuth, skipPaths))

			// Workspace
			r.Get("/workspace", svc.GetWorkspace)
			r.Get("/workspace/health", svc.GetWorkspaceHealth)
			r.Get("/workspaces", svc.ListWorkspaces)
			r.Post("/workspaces/{id}/switch", svc.SwitchWorkspace)

			// Processes
			r.Get("/processes", svc.ListProcesses)
			r.Post("/processes", svc.CreateProcess)
			r.Get("/processes/{id}", svc.GetProcess)
			r.Put("/processes/{id}", svc.UpdateProcess)
			r.Delete("/processes/{id}", svc.DeleteProcess)
			r.Post("/processes/{id}/trigger", svc.TriggerProcess)
			r.Post("/processes/{id}/pause", svc.PauseProcess)
			r.Post("/processes/{id}/resume", svc.ResumeProcess)

			// Runs
			r.Get("/runs", svc.ListRuns)
			r.Get("/runs/{id}", svc.GetRun)
			r.Post("/runs/{id}/cancel", svc.CancelRun)
			r.Post("/runs/{id}/kill", svc.KillRun)
			r.Post("/runs/{id}/replay", svc.ReplayRun)
			r.Get("/runs/{id}/output", svc.GetRunOutput)
			r.Patch("/runs/{id}/result", svc.SetRunResult)
			r.Get("/runs/{id}/result", svc.GetRunResult)
			r.Post("/runs/{id}/artifacts", svc.UploadArtifact)
			r.Get("/runs/{id}/artifacts", svc.ListArtifacts)
			r.Get("/runs/{id}/artifacts/{name}", svc.DownloadArtifact)

			// Orchestras
			r.Post("/orchestras", svc.CreateOrchestra)
			r.Get("/orchestras", svc.ListOrchestras)
			r.Get("/orchestras/{id}", svc.GetOrchestraHandler)
			r.Get("/orchestras/{id}/score", svc.GetOrchestraScore)
			r.Post("/orchestras/{id}/cancel", svc.CancelOrchestra)
			r.Post("/orchestras/{id}/pause", svc.PauseOrchestra)
			r.Post("/orchestras/{id}/resume", svc.ResumeOrchestra)
			r.Post("/orchestras/{id}/finish", svc.FinishOrchestraHandler)
			r.Post("/orchestras/{id}/chat", svc.PostChatMessage)
			r.Get("/orchestras/{id}/chat", svc.ListChatMessages)
			r.Get("/orchestras/{id}/stream", svc.StreamChat)
			r.Post("/runs/{id}/next", svc.NextMovement)
			r.Post("/runs/{id}/choice", svc.SetChoiceConfig)
			r.Post("/runs/{id}/choose", svc.Choose)

			// Secrets
			r.Get("/secrets", svc.ListSecrets)
			r.Post("/secrets", svc.CreateSecretHandler)
			r.Put("/secrets/{name}", svc.UpdateSecretHandler)
			r.Delete("/secrets/{name}", svc.DeleteSecretHandler)

			// Queues
			r.Get("/queues", svc.ListQueues)
			r.Post("/queues", svc.CreateQueue)
			r.Get("/queues/{id}", svc.GetQueue)
			r.Put("/queues/{id}", svc.UpdateQueue)

			// Jobs
			r.Get("/jobs", svc.ListJobs)
			r.Post("/jobs", svc.EnqueueJob)
			r.Post("/jobs/batch", svc.BatchEnqueueJobs)
			r.Get("/jobs/{id}", svc.GetJob)
			r.Post("/jobs/{id}/cancel", svc.CancelJob)
			r.Post("/jobs/{id}/replay", svc.ReplayJob)

			// Workers
			r.Get("/workers", svc.ListWorkers)
			r.Post("/workers", svc.CreateWorker)
			r.Get("/workers/{id}", svc.GetWorker)
			r.Put("/workers/{id}", svc.UpdateWorker)
			r.Delete("/workers/{id}", svc.DeleteWorker)

			// API Keys
			r.Get("/api-keys", svc.ListAPIKeys)
			r.Post("/api-keys", svc.CreateAPIKey)
			r.Delete("/api-keys/{id}", svc.DeleteAPIKey)

			// Members
			r.Get("/users", svc.ListMembers)
			r.Post("/users/invite", svc.InviteMember)

			// Webhooks
			r.Get("/webhook-subscriptions", svc.ListWebhookSubscriptions)
			r.Post("/webhook-subscriptions", svc.CreateWebhookSubscription)
			r.Delete("/webhook-subscriptions/{id}", svc.DeleteWebhookSubscription)
			r.Post("/webhook-subscriptions/{id}/test", svc.TestWebhookDelivery)

			// Credentials
			r.Get("/ssh-credentials", svc.ListSSHCredentials)
			r.Post("/ssh-credentials", svc.CreateSSHCredential)
			r.Delete("/ssh-credentials/{id}", svc.DeleteSSHCredential)
			r.Get("/ssm-profiles", svc.ListSSMProfiles)
			r.Post("/ssm-profiles", svc.CreateSSMProfile)
			r.Delete("/ssm-profiles/{id}", svc.DeleteSSMProfile)
			r.Get("/k8s-clusters", svc.ListK8sClusters)
			r.Post("/k8s-clusters", svc.CreateK8sCluster)
			r.Delete("/k8s-clusters/{id}", svc.DeleteK8sCluster)

			// System (workspace admin)
			r.Post("/system/cleanup", svc.TriggerCleanup)

			// Infrastructure (servers)
			r.Get("/infra/servers", svc.ListInfraServers)
			r.Get("/infra/pool", svc.GetInfraPool)
			r.Post("/infra/servers", svc.ProvisionServer)
			r.Delete("/infra/servers/{id}", svc.DestroyInfraServer)

			// Platform Admin (cross-workspace)
			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequirePlatformAdmin())
				r.Get("/stats", svc.AdminGetStats)
				r.Get("/workspaces", svc.AdminListWorkspaces)
				r.Post("/workspaces/{id}/state", svc.AdminUpdateWorkspaceState)
				r.Post("/workspaces/{id}/impersonate", svc.AdminImpersonate)
				r.Get("/users", svc.AdminListUsers)
				r.Post("/users/{id}/platform-admin", svc.AdminSetPlatformAdmin)
				r.Get("/infra/servers", svc.ListAllInfraServers)
			})
		})
	})

	// Serve embedded frontend (SPA catch-all)
	r.NotFound(frontend.Handler().ServeHTTP)

	return r
}

func parseDurationOrDefault(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func strPtr(s string) *string { return &s }

func firstNonEmptyPtr(values ...*string) *string {
	for _, v := range values {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}

// resolveCredentialsForWorker resolves database-stored credential references
// (ssh_credential_id, ssm_profile_id, k8s_cluster_id) into inline configuration
// so the worker can execute without database access.
func resolveCredentialsForWorker(ctx context.Context, pool *pgxpool.Pool, method string, cfg map[string]any) error {
	switch method {
	case "ssh":
		credID, _ := cfg["ssh_credential_id"].(string)
		if credID == "" {
			return nil // inline credentials already present
		}
		var privateKey []byte
		var username string
		var port int32
		var strictHostKey bool
		err := pool.QueryRow(ctx,
			`SELECT private_key_enc, username, port, strict_host_key FROM ssh_credentials WHERE id = $1`, credID,
		).Scan(&privateKey, &username, &port, &strictHostKey)
		if err != nil {
			return fmt.Errorf("resolve ssh credential %s: %w", credID, err)
		}
		cfg["private_key"] = string(privateKey)
		cfg["username"] = username
		cfg["port"] = port
		cfg["strict_host_key"] = strictHostKey
		delete(cfg, "ssh_credential_id")

	case "ssm":
		profileID, _ := cfg["ssm_profile_id"].(string)
		if profileID == "" {
			return nil // inline config already present
		}
		var region string
		var roleARN *string
		err := pool.QueryRow(ctx,
			`SELECT region, role_arn FROM ssm_profiles WHERE id = $1`, profileID,
		).Scan(&region, &roleARN)
		if err != nil {
			return fmt.Errorf("resolve ssm profile %s: %w", profileID, err)
		}
		cfg["region"] = region
		if roleARN != nil {
			cfg["role_arn"] = *roleARN
		}
		delete(cfg, "ssm_profile_id")

	case "k8s":
		clusterID, _ := cfg["k8s_cluster_id"].(string)
		if clusterID == "" {
			return nil // inline config already present
		}
		var kubeconfig []byte
		var defaultNamespace *string
		err := pool.QueryRow(ctx,
			`SELECT kubeconfig_enc, default_namespace FROM k8s_clusters WHERE id = $1`, clusterID,
		).Scan(&kubeconfig, &defaultNamespace)
		if err != nil {
			return fmt.Errorf("resolve k8s cluster %s: %w", clusterID, err)
		}
		cfg["kubeconfig"] = string(kubeconfig)
		if defaultNamespace != nil {
			cfg["namespace"] = *defaultNamespace
		}
		delete(cfg, "k8s_cluster_id")
	}
	return nil
}

func splitOnce(s, sep string) [2]string {
	if i := strings.Index(s, sep); i >= 0 {
		return [2]string{s[:i], s[i+len(sep):]}
	}
	return [2]string{s, ""}
}
