// Package container implements the Docker Swarm container execution method.
//
// Musicians run as ephemeral Swarm services with resource limits.
// Swarm handles bin-packing, scheduling, and node selection.
//
// Canonical rules:
// - Containers are ephemeral (restart-condition: none)
// - Resource limits (CPU, memory) from method_config
// - Environment variables injected (secrets + orchestra vars)
// - Logs captured via docker service logs
// - Kill via docker service rm
package container

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"

	"github.com/croncontrol/croncontrol/internal/executor"
)

// Compile-time contract.
var _ executor.Method = (*Method)(nil)

// ServerPool manages workspace server capacity for container execution.
// Implemented by infra.Provisioner; nil if auto-provisioning is disabled.
type ServerPool interface {
	EnsureCapacity(ctx context.Context, workspaceID string, needed int) (string, error)
	IncrementContainer(ctx context.Context, workspaceID string) (serverID string, err error)
	DecrementContainer(ctx context.Context, serverID string) error
}

// Config holds container executor settings.
type Config struct {
	SwarmEndpoint  string  // unix:///var/run/docker.sock or tcp://manager:2375
	DefaultCPU     float64 // default CPU limit per container (e.g., 0.5)
	DefaultMemory  int64   // default memory limit in bytes (e.g., 512MB)
	Network        string  // overlay network name
	PullTimeout    time.Duration
	ExecTimeout    time.Duration
}

// Method implements container execution via Docker Swarm.
type Method struct {
	client  *client.Client
	config  Config
	active  sync.Map // map[runID]string (service ID)
	pool    ServerPool
}

// New creates a new container execution method. pool may be nil if auto-provisioning is disabled.
func New(cfg Config, pool ServerPool) (*Method, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.SwarmEndpoint != "" {
		opts = append(opts, client.WithHost(cfg.SwarmEndpoint))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("container: create docker client: %w", err)
	}

	// Verify Swarm is active
	info, err := cli.Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("container: docker info: %w", err)
	}
	if info.Swarm.LocalNodeState != swarm.LocalNodeStateActive {
		slog.Warn("container: Docker Swarm is not active, container executor will not work until swarm init")
	}

	if cfg.DefaultCPU == 0 {
		cfg.DefaultCPU = 0.5
	}
	if cfg.DefaultMemory == 0 {
		cfg.DefaultMemory = 512 * 1024 * 1024 // 512MB
	}
	if cfg.ExecTimeout == 0 {
		cfg.ExecTimeout = 5 * time.Minute
	}

	return &Method{client: cli, config: cfg, pool: pool}, nil
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg := params.MethodConfig
	start := time.Now()

	image, _ := cfg["image"].(string)
	if image == "" {
		return executor.Result{Error: fmt.Errorf("container: image is required")}, nil
	}

	// Ensure workspace has server capacity (auto-provision if needed)
	var allocatedServerID string
	if m.pool != nil && params.WorkspaceID != "" {
		waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		if _, err := m.pool.EnsureCapacity(waitCtx, params.WorkspaceID, 1); err != nil {
			return executor.Result{Error: fmt.Errorf("container: no capacity: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
		sid, err := m.pool.IncrementContainer(ctx, params.WorkspaceID)
		if err != nil {
			slog.Warn("container: failed to increment container count", "error", err)
		}
		allocatedServerID = sid
	}

	// Parse command
	var command []string
	switch v := cfg["command"].(type) {
	case string:
		command = []string{"/bin/sh", "-c", v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				command = append(command, s)
			}
		}
	}

	// Resource limits
	cpuLimit := m.config.DefaultCPU
	memLimit := m.config.DefaultMemory
	if v, ok := cfg["cpu"].(string); ok {
		fmt.Sscanf(v, "%f", &cpuLimit)
	} else if v, ok := cfg["cpu"].(float64); ok {
		cpuLimit = v
	}
	if v, ok := cfg["memory"].(string); ok {
		// Parse "1G", "512M", etc.
		memLimit = parseMemory(v)
	}

	// Environment variables
	var envVars []string
	if params.Environment != nil {
		for k, v := range params.Environment {
			envVars = append(envVars, k+"="+v)
		}
	}
	envVars = append(envVars, "CRONCONTROL_RUN_ID="+params.RunID)
	if params.APIBaseURL != "" {
		envVars = append(envVars, "CRONCONTROL_API_URL="+params.APIBaseURL)
	}

	// Service name (must be unique, max 63 chars)
	serviceName := fmt.Sprintf("cc-%s", params.RunID)
	if len(serviceName) > 63 {
		serviceName = serviceName[:63]
	}

	// Create Swarm service
	replicas := uint64(1)
	maxAttempts := uint64(0) // no restart

	spec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: serviceName,
			Labels: map[string]string{
				"croncontrol.dev/run-id":       params.RunID,
				"croncontrol.dev/workspace-id": params.WorkspaceID,
				"managed-by":                   "croncontrol",
			},
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:   image,
				Command: command,
				Env:     envVars,
			},
			Resources: &swarm.ResourceRequirements{
				Limits: &swarm.Limit{
					NanoCPUs:    int64(cpuLimit * 1e9),
					MemoryBytes: memLimit,
				},
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition:   swarm.RestartPolicyConditionNone,
				MaxAttempts: &maxAttempts,
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{Replicas: &replicas},
		},
	}

	if params.WorkspaceID != "" {
		spec.TaskTemplate.Placement = &swarm.Placement{
			Constraints: []string{
				fmt.Sprintf("node.labels.workspace == %s", params.WorkspaceID),
			},
		}
	}

	svc, err := m.client.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("container: create service: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	serviceID := svc.ID
	m.active.Store(params.RunID, serviceID)
	defer func() {
		m.active.Delete(params.RunID)
		m.client.ServiceRemove(context.Background(), serviceID)
		// Decrement container count on the allocated server
		if m.pool != nil && allocatedServerID != "" {
			m.pool.DecrementContainer(context.Background(), allocatedServerID)
		}
	}()

	slog.Info("container: service created", "service", serviceName, "image", image)

	// Poll for completion
	result, err := m.waitForCompletion(ctx, serviceID, m.config.ExecTimeout)
	if err != nil {
		return executor.Result{Error: fmt.Errorf("container: wait: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Collect logs
	stdout, stderr := m.collectLogs(ctx, serviceID)

	durationMs := time.Since(start).Milliseconds()
	result.Stdout = stdout
	result.Stderr = stderr
	result.DurationMs = durationMs

	return result, nil
}

func (m *Method) Kill(_ context.Context, handle executor.Handle) error {
	val, ok := m.active.Load(handle.RunID)
	if !ok {
		return fmt.Errorf("container: no active service for run %s", handle.RunID)
	}
	serviceID := val.(string)
	return m.client.ServiceRemove(context.Background(), serviceID)
}

func (m *Method) SupportsKill() bool     { return true }
func (m *Method) SupportsHeartbeat() bool { return true }

func (m *Method) waitForCompletion(ctx context.Context, serviceID string, timeout time.Duration) (executor.Result, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return executor.Result{}, ctx.Err()
		case <-deadline:
			return executor.Result{Error: fmt.Errorf("container: execution timeout after %s", timeout)}, nil
		case <-ticker.C:
			f := filters.NewArgs(filters.Arg("service", serviceID))
			tasks, err := m.client.TaskList(ctx, types.TaskListOptions{Filters: f})
			if err != nil || len(tasks) == 0 {
				continue
			}

			task := tasks[len(tasks)-1] // most recent task
			switch task.Status.State {
			case swarm.TaskStateComplete:
				exitCode := 0
				if task.Status.ContainerStatus != nil {
					exitCode = task.Status.ContainerStatus.ExitCode
				}
				return executor.Result{ExitCode: &exitCode}, nil
			case swarm.TaskStateFailed, swarm.TaskStateRejected:
				exitCode := 1
				if task.Status.ContainerStatus != nil {
					exitCode = task.Status.ContainerStatus.ExitCode
				}
				return executor.Result{
					ExitCode: &exitCode,
					Error:    fmt.Errorf("container: %s: %s", task.Status.State, task.Status.Err),
				}, nil
			default:
				continue // still running
			}
		}
	}
}

func (m *Method) collectLogs(ctx context.Context, serviceID string) (string, string) {
	reader, err := m.client.ServiceLogs(ctx, serviceID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "all",
	})
	if err != nil {
		return "", ""
	}
	defer reader.Close()

	data, _ := io.ReadAll(io.LimitReader(reader, 5*1024*1024)) // 5MB max
	// Docker muxed stream: first 8 bytes per frame are header
	// For simplicity, return all as stdout
	return string(data), ""
}

func parseMemory(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	switch unit {
	case "G", "GI", "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "M", "MI", "MB":
		return int64(val * 1024 * 1024)
	case "K", "KI", "KB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}
