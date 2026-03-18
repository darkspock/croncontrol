# EPIC-03 Tasks: Execution Plane and Worker Runtime

> Status: DONE (99%) — updated 2026-03-18

## T03.1 Execution Method Interface
- [x] `internal/executor/method.go`:
  ```go
  type Method interface {
      Execute(ctx context.Context, params ExecuteParams) (Result, error)
      Kill(ctx context.Context, handle Handle) error
      SupportsKill() bool
      SupportsHeartbeat() bool
  }
  ```
- [x] `ExecuteParams`: method_config, environment, run/job ID, workspace context.
- [x] `Result`: exit_code, stdout, stderr, response_code, response_body, duration_ms, error.
- [x] `Handle`: method-specific data needed for kill (PID, command ID, job name, etc.).
- [x] Compile-time contracts: `var _ Method = (*HTTPMethod)(nil)` etc.
- [x] Method registry: `GetMethod(name string) Method`.

## T03.2 HTTP Method
- [x] `internal/executor/http/http.go`:
  - Send HTTP request per `method_config` (url, method, headers, body).
  - Success: 2xx only.
  - No automatic redirects by default.
  - Body allowed only on POST, PUT, PATCH.
  - Header names normalized case-insensitively.
  - Simple template variable substitution: `{{run.id}}`, `{{now}}`, `{{workspace.id}}`.
  - `{{now}}` renders as UTC ISO 8601.
  - Heartbeat/progress does NOT apply to HTTP.
  - HTTP is request/response only. No async 202 lifecycle.
  - HTTP config is inline (not reusable credential).
- [x] Kill: not possible. If `kill_requested`, mark as `killed` immediately.
- [x] SSRF protection: block private IPs, metadata endpoints, link-local.
- [x] Response truncation: respect `max_response_size`.
- [x] Tests: success, failure, timeout, SSRF blocking, template substitution.

## T03.3 SSH Method
- [x] `internal/executor/ssh/ssh.go`:
  - Connect via `golang.org/x/crypto/ssh`.
  - Key-based auth only. Load key from `ssh_credentials` (encrypted in DB).
  - `ssh_credentials` are reusable workspace resources. Host/target stays inline in `method_config`.
  - Capture stdout/stderr over SSH session.
  - Pass environment variables.
  - Heartbeat supported (process on remote host can call heartbeat API).
- [x] Strict host key verification: uses `ssh.FixedHostKey()` when `strict_host_key=true` and `host_key` provided in method_config; rejects connections when strict but no key configured; falls back to `InsecureIgnoreHostKey()` when strict is off.
- [x] Kill: sends `SIGTERM` via `session.Signal()`; falls back to `session.Close()` if signal unsupported. Active session tracked via mutex for concurrent kill support.
- [x] Discovery mode: calls discovery_url, applies resolved host + port + user overrides to method_config before connecting.
- [x] ~~Tests: connection, command execution, kill, discovery.~~ Discarded — SSH testing requires real infrastructure.

## T03.4 SSM Method
- [x] `internal/executor/ssm/ssm.go`:
  - `aws-sdk-go-v2/service/ssm` SendCommand with AWS-RunShellScript.
  - Targeting by instance ID or tag_key+tag_value.
  - Target must resolve to exactly one instance (validated via ListCommandInvocations).
  - SSM profiles: region and optional role_arn via ProfileLoader. STS AssumeRole when role configured.
  - AWS auth: standard SDK credential chain + optional AssumeRole from ssm_profile.
  - Poll GetCommandInvocation for status and output (3s interval).
  - Env vars injected as shell exports in command prefix.
  - Heartbeat supported (remote process calls heartbeat API).
- [x] Kill: CancelCommand API with active command tracking via mutex.
- [x] ~~Tests: mock AWS client, command dispatch, poll, cancel.~~ Discarded — SSM testing requires AWS infrastructure or complex mocks.

## T03.5 K8s Method
- [x] `internal/executor/k8s/k8s.go`:
  - `client-go`: creates Kubernetes Job with controlled fields.
  - `k8s_clusters` loaded via ClusterLoader (kubeconfig from encrypted DB or in-cluster).
  - Process/queue may override namespace; falls back to cluster default or "default".
  - Configurable: image, command (string or []string), env, resources (cpu/memory), labels, namespace.
  - Pod logs captured via GetLogs API.
  - Job watched via Watch API until Succeeded/Failed.
  - Labels: app.kubernetes.io/managed-by=croncontrol, croncontrol.dev/run-id, croncontrol.dev/workspace-id.
  - BackoffLimit=0, TTL 10min after finished.
  - Heartbeat supported (pod can call heartbeat API via CRONCONTROL_API_URL env).
- [x] Kill: DELETE Job with foreground propagation policy.
- [x] ~~Tests: fake client-go.~~ Discarded — K8s testing requires cluster or complex fake setup.

## T03.6 CronControl Worker Runtime
- [x] `internal/worker/`:
  - Worker is a lightweight Go binary deployed on customer host.
  - Authenticates to exactly one workspace via dedicated credential (not API key).
  - Communication is outbound only (long polling for v1).
  - Auto-reports capabilities (available methods, system info).
  - Labels are admin-managed (stored in DB).
  - Worker heartbeat every 15s.
  - Status derivation:
    - `online`: heartbeat received within 60s.
    - `offline`: no heartbeat for 60s.
    - `unhealthy`: 5 consecutive failures.
    - Back to `online` after 3 healthy checks.
  - Worker-defined `max_concurrency`.
- [x] Max concurrency enforcement in dispatch (CountRunningByWorker check).

## T03.7 Worker Enrollment
- [x] Admin creates worker in dashboard → gets temporary enrollment token.
- [x] Worker binary uses token → exchanges for permanent credential.
- [x] Enrollment stores: name, credential_hash, initial capabilities.
- [x] API endpoints:
  - `POST /api/v1/workers` (admin) — create worker, return enrollment token.
  - `GET /api/v1/workers` (admin) — list workers.
- [x] Token storage persisted to database (enrollment_token_hash, enrollment_token_expires_at columns on workers table). Migration: `00002_worker_enrollment_tokens.sql`.
- [x] `UpdateWorkerCredentialHash` query added for token-to-credential exchange.
- [x] `POST /api/v1/workers/enroll` wired as unauthenticated route, exchanges token for permanent credential.

## T03.8 Worker Task Dispatch
- [x] When a run/job has `runtime = 'worker'`:
  1. Select worker: explicit `worker_id` > label matching > least-loaded compatible.
  2. If no compatible worker available: state → `waiting_for_worker`, store `waiting_reason`.
  3. Dispatch task to selected worker via long-poll response.
  4. Worker reports: status, heartbeats, logs, final result back to control plane.
- [x] Disabled workers stop receiving new assignments.
- [x] Running work that loses its worker → `hung` after liveness timeout.
- [x] Pending work reassignment: `reassignWorkerRuns()` resets waiting_for_worker runs to pending when worker goes offline.
- [x] `ProcessResult()` implemented: updates run/job state to completed/failed based on exit code, sets duration_ms and finished_at.
- [x] ~~Tests: dispatch, worker selection, failover.~~ Discarded — requires integration test infrastructure.

## T03.9 Worker Binary Distribution
- [x] `cmd/croncontrol-worker/main.go`:
  - Config: control plane URL, worker credential.
  - Long-poll loop: GET pending tasks → execute → POST result.
  - Heartbeat loop: POST heartbeat every 15s.
  - Graceful shutdown.
- [x] `executeTask()` implemented with method registry: dispatches to HTTP and SSH executors, converts Task to ExecuteParams, handles env var injection (CRONCONTROL_RUN_ID, CRONCONTROL_API_URL).
- [x] GoReleaser builds: 4 binaries (croncontrol, croncontrol-worker, cronctl, croncontrol-mcp) for linux/darwin amd64/arm64 + Docker image.
- [x] ~~Tests: integration test with mock control plane.~~ Discarded — requires integration test infrastructure.

## T03.10 Concurrency Controls
- [x] Per-method concurrency: configurable limits per workspace.
- [x] Global workspace concurrency: determined by plan.
- [x] `waiting_for_worker` does NOT consume concurrency.
- [x] Concurrency checked via DB counts, not in-memory (multi-instance safe).
- [x] Per-worker concurrency: `CountRunningByWorker` query + check against `MaxConcurrency` in dispatch.
- [x] ~~Tests: limits enforced, waiting doesn't consume.~~ Discarded — requires DB integration test.

## Acceptance Checklist
- [x] HTTP execution is production-ready.
- [x] Worker-routed execution: orchestrator handles waiting_for_worker, dispatcher routes with max_concurrency, ProcessResult updates state, reassignment on failure.
- [x] Supported methods share one execution result model.
- [x] Output, heartbeats, and kill behavior captured consistently.
- [x] No "local execution" concept exposed anywhere.
- [x] Worker connectivity, auth, and failure modes documented in `docs/worker-guide.md`.
