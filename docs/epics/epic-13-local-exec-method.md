# EPIC-13: Local Exec Method — Subprocess Execution

## Outcome

A new execution method `exec` that runs commands as local subprocesses on the worker host. The worker spawns the command, captures stdout/stderr incrementally, injects environment variables, and reports results back to the control plane. This enables running scripts (PHP, Python, bash, etc.) directly on worker infrastructure with progress reporting via the SDK.

## Why This Epic Exists

CronControl defines four canonical execution methods: `http`, `ssh`, `ssm`, `k8s`. All execute remotely — none execute a local command on the worker host itself. The most common use case for a worker deployed in customer infrastructure is to run local scripts with progress reporting via the SDK.

`exec` is the fifth execution method. Unlike the other four, it is **worker-only** — the control plane dispatches exec tasks to workers but never executes them directly. This is an intentional exception to the rule that all methods support both runtimes.

## Auth Model

The subprocess does **not** receive a workspace API key. SDK calls from the subprocess go through the worker relay (EPIC-12):

```
Subprocess                    Worker Relay (localhost)         Control Plane
    |                              |                              |
    |-- heartbeat (no auth) ------>|-- forward (no auth) -------->|
    |-- set_result --------------->|-- /workers/{id}/relay ------>|
    |                              |   (worker credential)        |
```

Environment variables injected into the subprocess:

```
CRONCONTROL_URL=https://croncontrol.io          # for read operations (direct)
CRONCONTROL_RELAY_URL=http://localhost:9091      # for write operations (via relay)
CRONCONTROL_RUN_ID=run_01ABC...
CRONCONTROL_JOB_ID=job_01DEF...                 # for queue jobs
CRONCONTROL_JOB_PAYLOAD={"to":"user@test.com"}  # job payload as JSON
CRONCONTROL_WORKSPACE_ID=ws_01GHI...
```

No `CRONCONTROL_API_KEY` is injected. Write operations go through the relay, which authenticates with the worker credential. Read operations use the unauthenticated or public endpoints on the control plane.

**If the relay is not enabled**, the subprocess can only send heartbeats (unauthenticated) directly to the control plane. Results, chat, and artifacts require the relay.

## Process Configuration

```json
{
  "name": "nightly-cleanup",
  "execution_method": "exec",
  "runtime": "worker",
  "method_config": {
    "command": "php /opt/scripts/cleanup.php",
    "working_dir": "/opt/scripts",
    "timeout": "5m",
    "shell": true
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | required | Command to execute |
| `working_dir` | string | `.` | Working directory (absolute path) |
| `timeout` | string | `5m` | Max execution time |
| `shell` | bool | `true` | Wrap in `sh -c` (allows pipes, redirects) |
| `user` | string | current | Run as this OS user (requires privileges) |

## Two Progress Models

### Active Progress (with SDK)

The subprocess uses the CronControl SDK to report structured progress:

```php
$cc = new CronControl();
$cc->heartbeat(1000, 42, "Processing record 42...");
```

Requires EPIC-12 relay for result/chat/artifact writes. Heartbeats are unauthenticated and work without relay.

### Passive Progress (without SDK, stdout only)

For processes where the SDK cannot be embedded (legacy scripts, third-party binaries, bash one-liners), the worker captures stdout in real time and transmits it to the control plane as incremental output chunks.

```bash
#!/bin/bash
# No SDK needed — stdout is captured automatically
for i in $(seq 1 100); do
    process_record $i
    echo "[${i}/100] Processed record ${i}"
done
echo "DONE: 100 records processed"
```

The dashboard shows the stdout stream in the run output viewer. The task appears alive as long as new stdout arrives. No SDK, no heartbeat calls, no code changes required.

Additionally, the control plane can optionally parse progress patterns from stdout:

| Pattern | Example | Parsed As |
|---------|---------|-----------|
| `[N/M]` | `[42/100]` | progress: 42/100 (42%) |
| `N%` | `67%` | progress: 67% |
| `progress:N/M` | `progress:42/100` | progress: 42/100 |

This is opt-in via `method_config.parse_progress: true` (default: false). When enabled, the last matching pattern in each stdout chunk updates the run's heartbeat progress automatically — no SDK needed.

## Start/Poll/Kill Contract (Durable Handles)

The exec method follows the EPIC-11 durable handle model. The handle must contain enough information to **recover polling after a worker restart**, not just an in-memory reference.

### Handle Data

```json
{
  "pid": 12345,
  "work_dir": "/tmp/croncontrol/run_01ABC",
  "stdout_path": "/tmp/croncontrol/run_01ABC/stdout.log",
  "stderr_path": "/tmp/croncontrol/run_01ABC/stderr.log",
  "exit_code_path": "/tmp/croncontrol/run_01ABC/exit_code",
  "started_at": "2026-03-24T10:00:00Z"
}
```

Each subprocess writes to a dedicated work directory. Stdout/stderr are captured to files (not pipes) so that:
- Polling reads from file offsets (survives restarts)
- Multiple poll cycles read incrementally
- The handle is fully durable — no in-memory state required for polling

### Start

```go
func (m *ExecMethod) Start(ctx context.Context, params StartParams) (StartResult, error) {
    cfg := parseConfig(params.MethodConfig)

    // Create work directory for this run
    workDir := filepath.Join(m.baseDir, params.RunID)
    os.MkdirAll(workDir, 0755)

    stdoutPath := filepath.Join(workDir, "stdout.log")
    stderrPath := filepath.Join(workDir, "stderr.log")
    exitCodePath := filepath.Join(workDir, "exit_code")

    stdoutFile, _ := os.Create(stdoutPath)
    stderrFile, _ := os.Create(stderrPath)

    cmd := buildCommand(cfg)
    cmd.Stdout = stdoutFile
    cmd.Stderr = stderrFile
    cmd.Env = buildEnv(params.Environment)
    cmd.Dir = cfg.WorkingDir

    if err := cmd.Start(); err != nil {
        return StartResult{Result: &Result{Error: err}}, nil
    }

    // Write exit code on completion (background goroutine)
    go func() {
        err := cmd.Wait()
        exitCode := 0
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        }
        os.WriteFile(exitCodePath, []byte(strconv.Itoa(exitCode)), 0644)
        stdoutFile.Close()
        stderrFile.Close()
    }()

    handle := Handle{
        MethodName: "exec",
        RunID:      params.RunID,
        Data: map[string]any{
            "pid":            cmd.Process.Pid,
            "work_dir":       workDir,
            "stdout_path":    stdoutPath,
            "stderr_path":    stderrPath,
            "exit_code_path": exitCodePath,
            "started_at":     time.Now().UTC().Format(time.RFC3339),
        },
    }

    return StartResult{Handle: handle}, nil
}
```

### Poll (File-Based, No In-Memory State)

```go
func (m *ExecMethod) Poll(ctx context.Context, handle Handle, cursor PollCursor) (PollResult, error) {
    stdoutPath := getString(handle.Data, "stdout_path")
    stderrPath := getString(handle.Data, "stderr_path")
    exitCodePath := getString(handle.Data, "exit_code_path")

    // Read incremental stdout from file offset
    stdout := readFromOffset(stdoutPath, cursor.StdoutOffset)
    stderr := readFromOffset(stderrPath, cursor.StderrOffset)

    // Check if process exited (exit_code file exists)
    exitCodeBytes, err := os.ReadFile(exitCodePath)
    if err != nil {
        // Process still running
        return PollResult{
            State:        RemoteRunning,
            StdoutChunk:  stdout,
            StderrChunk:  stderr,
        }, nil
    }

    // Process exited
    exitCode, _ := strconv.Atoi(strings.TrimSpace(string(exitCodeBytes)))
    state := RemoteCompleted
    if exitCode != 0 {
        state = RemoteFailed
    }

    return PollResult{
        State:       state,
        StdoutChunk: stdout,
        StderrChunk: stderr,
        Result:      &Result{ExitCode: &exitCode},
    }, nil
}
```

### Kill

```go
func (m *ExecMethod) Kill(ctx context.Context, handle Handle) error {
    pid, _ := handle.Data["pid"].(float64)
    if pid == 0 {
        return fmt.Errorf("exec: no pid in handle")
    }

    proc, err := os.FindProcess(int(pid))
    if err != nil {
        return nil // process already gone
    }

    // SIGTERM first
    proc.Signal(syscall.SIGTERM)

    // After 10s grace, SIGKILL
    go func() {
        select {
        case <-ctx.Done():
            return
        case <-time.After(10 * time.Second):
            proc.Kill()
        }
    }()

    return nil
}
```

### Recovery After Worker Restart

When the worker restarts:
1. The control plane's async polling loop calls `ListActiveAsyncRuns` — finds runs with `exec` handles
2. It dispatches a Poll to the worker with the persisted handle
3. The worker's `Poll()` reads stdout/stderr from files and checks for exit_code file
4. If the process finished while the worker was down, exit_code file exists → finalize
5. If the process is still running (orphaned), the PID can be checked with `os.FindProcess` + signal 0

No in-memory state needed. Everything is recoverable from the filesystem.

## Scope: Worker Async Execution

The current worker binary returns `"async worker executions are not implemented yet"` for any method whose `Start()` returns a durable handle (no immediate `Result`). This must be fixed as part of this epic.

### Worker Async Loop (In Scope)

The worker needs its own async polling loop, similar to the control plane's:

```go
// In the worker binary:
// 1. Start() returns handle (no Result) → store handle locally
// 2. Every 5s, Poll() each active handle
// 3. On terminal state → report result to control plane
// 4. On kill request (from control-poll) → call Kill()
```

This replaces the current error stub at `cmd/croncontrol-worker/main.go` line 283.

## Canonical Model Update

This epic adds `exec` to the product specification as the fifth execution method with a worker-only constraint:

| Method | Runtimes | Description |
|--------|----------|-------------|
| `http` | direct, worker | HTTP request to URL |
| `ssh` | direct, worker | SSH command on remote host |
| `ssm` | direct, worker | AWS SSM command on EC2 |
| `k8s` | direct, worker | Kubernetes Job |
| `exec` | **worker only** | Local subprocess on worker host |

The product-specification.md must be updated to reflect this.

## Implementation

### New Files

- `internal/executor/exec/exec.go` — ExecMethod with Start/Poll/Kill (file-based)
- `internal/executor/exec/exec_test.go` — Tests with real subprocesses

### Modified Files

- `cmd/croncontrol-worker/main.go` — Register `exec` method + implement async execution loop
- `docs/product-specification.md` — Add `exec` as fifth canonical method (worker-only)
- `internal/handler/openapi.yaml` — Document `exec` method config
- `frontend/src/pages/ProcessCreate.tsx` — Add exec config form
- `frontend/src/pages/ProcessDetail.tsx` — Show exec config

## Security

- Subprocess runs as the worker process user (or specified `user`)
- No API key injected — writes go through relay (EPIC-12) with worker credential
- Command comes from process/queue `method_config` — set by workspace admins
- Working directory validated as absolute path
- Work directory cleaned up after task completion (configurable retention)

## Dependencies

- **EPIC-11** (Start/Poll/Kill contract, durable handles)
- **EPIC-12** (Worker Relay — required for result/chat/artifact writes from subprocess)

## Acceptance Criteria

- [ ] `exec` execution method with file-based durable handles
- [ ] Start: spawns subprocess, writes stdout/stderr to files, returns handle with paths
- [ ] Poll: reads from file offsets, checks exit_code file (no in-memory state)
- [ ] Kill: SIGTERM → 10s grace → SIGKILL
- [ ] Recovery: polling works after worker restart (file-based)
- [ ] Worker async execution loop (replaces "not implemented" stub)
- [ ] Environment injection: CRONCONTROL_URL, RELAY_URL, RUN_ID, JOB_ID, JOB_PAYLOAD
- [ ] No CRONCONTROL_API_KEY injected (writes via relay only)
- [ ] method_config: command, working_dir, timeout, shell, user
- [ ] Work directory cleanup after completion
- [ ] Passive progress: stdout chunks transmitted to control plane as run output
- [ ] Optional `parse_progress: true` extracts progress from stdout patterns ([N/M], N%)
- [ ] Liveness detection: run considered alive while stdout grows
- [ ] Tests with real subprocesses (with SDK and without SDK)
- [ ] product-specification.md updated with exec as fifth method
- [ ] Frontend: exec config form in ProcessCreate
- [ ] OpenAPI: exec method documented
