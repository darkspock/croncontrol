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
	"github.com/croncontrol/croncontrol/internal/frontend"
	"github.com/croncontrol/croncontrol/internal/handler"
	"github.com/croncontrol/croncontrol/internal/logging"
	logdb "github.com/croncontrol/croncontrol/internal/logging/database"
	logfile "github.com/croncontrol/croncontrol/internal/logging/file"
	logos "github.com/croncontrol/croncontrol/internal/logging/opensearch"
	"github.com/croncontrol/croncontrol/internal/metrics"
	"github.com/croncontrol/croncontrol/internal/monitor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/croncontrol/croncontrol/internal/notifier"
	"github.com/croncontrol/croncontrol/internal/planner"
	"github.com/croncontrol/croncontrol/internal/queue"
	"github.com/croncontrol/croncontrol/internal/recovery"
	orchestramonitor "github.com/croncontrol/croncontrol/internal/orchestra"
	"github.com/croncontrol/croncontrol/internal/storage"
	"github.com/croncontrol/croncontrol/internal/worker"
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
	registry.Register("http", exechttp.New(5 * 1024 * 1024)) // 5MB max response

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

	// Wire dependency resolver into executor
	orchestrator.SetOnRunTerminal(func(ctx context.Context, run db.Run, proc db.Process) {
		depResolver.Evaluate(ctx, run)
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
	svc := handler.NewService(queries, pool, orchestrator, depResolver, webhookNotifier)

	// Configure Google OAuth if credentials are set
	baseURL := cfg.SaaS.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	}
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
		AllowedOrigins:   []string{"*"},
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
		"/health":              true,
		"/api/v1/heartbeat":    true,
		"/api/v1/register":     true,
		"/api/v1/register/verify": true,
		"/api/v1/login":        true,
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
		r.Post("/infra/servers/{id}/ready", svc.ServerReadyCallback)
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

func splitOnce(s, sep string) [2]string {
	if i := strings.Index(s, sep); i >= 0 {
		return [2]string{s[:i], s[i+len(sep):]}
	}
	return [2]string{s, ""}
}
