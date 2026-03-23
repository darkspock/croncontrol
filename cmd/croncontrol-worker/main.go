// CronControl Worker binary.
//
// A lightweight runtime deployed on customer infrastructure that:
// - Authenticates to exactly one workspace via dedicated credential.
// - Long-polls the control plane for tasks.
// - Executes tasks using local execution methods (http, ssh, ssm, k8s).
// - Reports results, heartbeats, and status back to the control plane.
//
// Usage:
//
//	croncontrol-worker --url https://croncontrol.io --credential wrk_cred_abc123
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/croncontrol/croncontrol/internal/executor"
	exechttp "github.com/croncontrol/croncontrol/internal/executor/http"
	execk8s "github.com/croncontrol/croncontrol/internal/executor/k8s"
	execssh "github.com/croncontrol/croncontrol/internal/executor/ssh"
	execssm "github.com/croncontrol/croncontrol/internal/executor/ssm"
)

var (
	version = "dev"
)

type Config struct {
	ControlPlaneURL  string
	Credential       string
	PollTimeout      time.Duration
	HeartbeatEvery   time.Duration
	ControlPollEvery time.Duration
}

type Task struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	AttemptID       string         `json:"attempt_id,omitempty"`
	WorkspaceID     string         `json:"workspace_id"`
	ExecutionMethod string         `json:"execution_method"`
	MethodConfig    map[string]any `json:"method_config"`
	Environment     map[string]any `json:"environment,omitempty"`
	APIBaseURL      string         `json:"api_base_url"`
}

type TaskResult struct {
	TaskID       string `json:"task_id"`
	AttemptID    string `json:"attempt_id,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	ResponseCode *int   `json:"response_code,omitempty"`
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

type controlCommand struct {
	TaskID string `json:"task_id"`
	Action string `json:"action"`
}

type activeTask struct {
	cancel context.CancelFunc
	method executor.Method
	handle executor.Handle
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := Config{
		PollTimeout:      30 * time.Second,
		HeartbeatEvery:   15 * time.Second,
		ControlPollEvery: 5 * time.Second,
	}

	flag.StringVar(&cfg.ControlPlaneURL, "url", os.Getenv("CRONCONTROL_URL"), "Control plane URL")
	flag.StringVar(&cfg.Credential, "credential", os.Getenv("CRONCONTROL_CREDENTIAL"), "Worker credential")
	flag.Parse()

	if cfg.ControlPlaneURL == "" {
		return fmt.Errorf("--url or CRONCONTROL_URL is required")
	}
	if cfg.Credential == "" {
		return fmt.Errorf("--credential or CRONCONTROL_CREDENTIAL is required")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("croncontrol-worker starting",
		"version", version,
		"control_plane", cfg.ControlPlaneURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: cfg.PollTimeout + 5*time.Second}

	// Build execution method registry
	registry := buildMethodRegistry()
	slog.Info("execution methods registered", "methods", []string{"http", "ssh", "ssm", "k8s"})

	var activeTasks sync.Map

	// Start heartbeat loop
	go heartbeatLoop(ctx, client, cfg)
	go controlPollLoop(ctx, client, cfg, &activeTasks)

	// Main poll loop
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down worker")
			return nil
		default:
			task, err := pollTask(ctx, client, cfg)
			if err != nil {
				slog.Error("poll failed", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}
			if task == nil {
				continue // no task available, poll again
			}

			slog.Info("received task", "id", task.ID, "method", task.ExecutionMethod)

			go func(task *Task) {
				result := executeTask(ctx, task, registry, &activeTasks)
				if err := reportResult(ctx, client, cfg, result); err != nil {
					slog.Error("report result failed", "error", err)
				}
			}(task)
		}
	}
}

func pollTask(ctx context.Context, client *http.Client, cfg Config) (*Task, error) {
	url := cfg.ControlPlaneURL + "/api/v1/workers/poll"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusRequestTimeout {
		return nil, nil // no task, poll again
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("poll returned %d: %s", resp.StatusCode, string(body))
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("decode task: %w", err)
	}

	return &task, nil
}

// buildMethodRegistry creates the executor method registry with available methods.
// The worker uses the same execution methods as the control plane but runs them
// from the customer's local network.
func buildMethodRegistry() *executor.Registry {
	reg := executor.NewRegistry()

	// HTTP method — always available
	reg.Register("http", exechttp.New(5*1024*1024))

	// SSH method — available when credentials are provided via method_config.
	// On the worker side, SSH credentials come inline in the task's method_config
	// (private_key, username, port) rather than via DB credential loader.
	reg.RegisterBlocking("ssh", execssh.New(workerCredentialNotAvailable))

	// SSM method — credentials (region, role_arn) come inline in method_config.
	// The worker uses local AWS credentials/role from its environment.
	reg.Register("ssm", execssm.New(workerSSMProfileLoader))

	// K8s method — kubeconfig comes inline in method_config or from local ~/.kube/config.
	reg.Register("k8s", execk8s.New(workerK8sClusterLoader))

	return reg
}

// workerCredentialNotAvailable is a fallback loader — actual credentials are injected by the dispatcher.
func workerCredentialNotAvailable(_ context.Context, _ string) ([]byte, string, int, bool, error) {
	return nil, "", 0, true, fmt.Errorf("credentials must be provided in method_config for worker execution")
}

// workerSSMProfileLoader loads SSM profile from method_config (injected by dispatcher).
func workerSSMProfileLoader(_ context.Context, _ string) (string, string, error) {
	// Worker-side SSM uses the local AWS credentials and region from environment.
	// Profile data is injected by the control plane into method_config.
	return "", "", fmt.Errorf("ssm profile must be provided in method_config for worker execution")
}

// workerK8sClusterLoader loads K8s config from method_config (injected by dispatcher).
func workerK8sClusterLoader(_ context.Context, _ string) ([]byte, string, error) {
	// Worker-side K8s uses local kubeconfig or in-cluster config.
	// Cluster data is injected by the control plane into method_config.
	return nil, "", fmt.Errorf("k8s cluster config must be provided in method_config for worker execution")
}

func executeTask(ctx context.Context, task *Task, registry *executor.Registry, activeTasks *sync.Map) TaskResult {
	start := time.Now()

	slog.Info("executing task", "id", task.ID, "method", task.ExecutionMethod)

	method, ok := registry.Get(task.ExecutionMethod)
	if !ok {
		return TaskResult{
			TaskID:     task.ID,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("unsupported execution method: %s", task.ExecutionMethod),
		}
	}

	// Convert task environment from map[string]any to map[string]string
	env := make(map[string]string, len(task.Environment))
	for k, v := range task.Environment {
		if s, ok := v.(string); ok {
			env[k] = s
		}
	}

	// Inject CronControl variables so remote processes can report heartbeats
	env["CRONCONTROL_RUN_ID"] = task.ID
	if task.APIBaseURL != "" {
		env["CRONCONTROL_API_URL"] = task.APIBaseURL
	}

	params := executor.StartParams{
		RunID:        task.ID,
		WorkspaceID:  task.WorkspaceID,
		MethodConfig: task.MethodConfig,
		Environment:  env,
		APIBaseURL:   task.APIBaseURL,
	}

	execCtx, cancel := context.WithCancel(ctx)
	handle := executor.Handle{MethodName: task.ExecutionMethod, RunID: task.ID, Data: make(map[string]any)}
	activeTasks.Store(task.ID, &activeTask{cancel: cancel, method: method, handle: handle})
	defer func() {
		cancel()
		activeTasks.Delete(task.ID)
	}()

	startResult, err := method.Start(execCtx, params)
	if startResult.Handle.MethodName != "" {
		handle = startResult.Handle
		activeTasks.Store(task.ID, &activeTask{cancel: cancel, method: method, handle: handle})
	}
	if startResult.Result == nil {
		durationMs := time.Since(start).Milliseconds()
		return TaskResult{
			TaskID:     task.ID,
			AttemptID:  task.AttemptID,
			DurationMs: durationMs,
			Error:      "async worker executions are not implemented yet",
		}
	}
	result := *startResult.Result
	durationMs := time.Since(start).Milliseconds()

	tr := TaskResult{
		TaskID:       task.ID,
		AttemptID:    task.AttemptID,
		ExitCode:     result.ExitCode,
		ResponseCode: result.ResponseCode,
		Stdout:       result.Stdout,
		Stderr:       result.Stderr,
		ResponseBody: result.ResponseBody,
		DurationMs:   durationMs,
	}

	if err != nil {
		tr.Error = err.Error()
	} else if result.Error != nil {
		tr.Error = result.Error.Error()
	}

	slog.Info("task completed",
		"id", task.ID,
		"method", task.ExecutionMethod,
		"duration_ms", durationMs,
		"success", result.IsSuccess(),
	)

	return tr
}

func controlPollLoop(ctx context.Context, client *http.Client, cfg Config, activeTasks *sync.Map) {
	ticker := time.NewTicker(cfg.ControlPollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sendControlPoll(ctx, client, cfg, activeTasks)
		case <-ctx.Done():
			return
		}
	}
}

func sendControlPoll(ctx context.Context, client *http.Client, cfg Config, activeTasks *sync.Map) {
	activeIDs := make([]string, 0)
	activeTasks.Range(func(key, value any) bool {
		if id, ok := key.(string); ok {
			activeIDs = append(activeIDs, id)
		}
		return true
	})

	body, _ := json.Marshal(map[string]any{"active_tasks": activeIDs})
	req, err := http.NewRequestWithContext(ctx, "POST", cfg.ControlPlaneURL+"/api/v1/workers/control-poll", bytes.NewReader(body))
	if err != nil {
		slog.Error("control poll: create request", "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("control poll failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var payload struct {
		Data struct {
			Commands []controlCommand `json:"commands"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		slog.Error("control poll decode failed", "error", err)
		return
	}

	for _, cmd := range payload.Data.Commands {
		if cmd.Action != "kill" {
			continue
		}
		val, ok := activeTasks.Load(cmd.TaskID)
		if !ok {
			continue
		}
		task := val.(*activeTask)
		task.cancel()
		if task.method.SupportsKill() {
			killCtx, killCancel := context.WithTimeout(ctx, 10*time.Second)
			if err := task.method.Kill(killCtx, task.handle); err != nil {
				slog.Warn("worker kill failed", "task", cmd.TaskID, "error", err)
			}
			killCancel()
		}
	}
}

func reportResult(ctx context.Context, client *http.Client, cfg Config, result TaskResult) error {
	url := cfg.ControlPlaneURL + "/api/v1/workers/result"

	body, err := json.Marshal(result)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("report returned %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("result reported", "task", result.TaskID)
	return nil
}

func heartbeatLoop(ctx context.Context, client *http.Client, cfg Config) {
	ticker := time.NewTicker(cfg.HeartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sendHeartbeat(ctx, client, cfg)
		case <-ctx.Done():
			return
		}
	}
}

func sendHeartbeat(ctx context.Context, client *http.Client, cfg Config) {
	url := cfg.ControlPlaneURL + "/api/v1/workers/heartbeat"

	payload, _ := json.Marshal(map[string]any{
		"version": version,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		slog.Error("heartbeat: create request", "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("heartbeat: send failed", "error", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("heartbeat: unexpected status", "status", resp.StatusCode)
	}
}
