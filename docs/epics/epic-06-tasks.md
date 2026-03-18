# EPIC-06 Tasks: Observability, Alerts, and Data Lifecycle

> Status: DONE (95%) — updated 2026-03-18

## T06.1 Heartbeat API
- [x] `POST /api/v1/heartbeat` (no workspace auth, implicit via run ID):
  - Accept: `run_id` (or `execution_id`), total, current, message.
  - Calculate progress: `(current / total) * 100`.
  - Update `runs` table: progress_total, progress_current, progress, progress_message, last_heartbeat_at.
  - Insert heartbeat record into `heartbeats` table.
  - Heartbeat does NOT apply to HTTP execution method.
- [x] Prometheus counter: `heartbeats_received_total`.
- [x] Tests: progress update, heartbeat recording.

## T06.2 Heartbeat Timeout Detection
- [x] Monitor component checks running runs:
  - If `heartbeat_timeout > 0` and `now - last_heartbeat_at > heartbeat_timeout` → `hung`.
  - For initial heartbeat: if `now - started_at > heartbeat_timeout` and no heartbeat received → `hung`.
  - `heartbeat_timeout = null` → heartbeat monitoring disabled.
- [x] Apply `timeout_action`: kill, alert, or both.
- [x] Tests: timeout detection, initial heartbeat, disabled monitoring.

## T06.3 Execution Timeout Detection
- [x] Monitor checks: if `now - started_at > execution_timeout` → `hung`.
- [x] Apply timeout_action.
- [x] Tests: timeout detection, action dispatch.

## T06.4 Run Output Capture
- [x] For non-HTTP methods: capture stdout/stderr in `run_output` table.
- [x] Truncation: respect size limits. Set `truncated = true` and store original size estimate.
- [x] For HTTP: response body stored in run attempt, not run_output.
- [x] Tests: output capture, truncation.

## T06.5 Webhook Event System
- [x] `internal/notifier/webhook.go`:
  - Unified event system. One model for all events.
  - Multiple subscriptions per workspace.
  - Each subscription has: URL, secret, event_types filter.
  - Delivery:
    - HTTP POST with JSON payload.
    - HMAC-SHA256 signature in `X-CronControl-Signature` header.
    - Timestamp in `X-CronControl-Timestamp`.
    - Unique delivery ID in `X-CronControl-Delivery-Id`.
    - At-least-once delivery guarantee.
  - Retry: 3 attempts with exponential backoff.
  - Auto-disable after 20 consecutive failures. Reactivation manual only.
- [x] Event families:
  - `run.completed`, `run.failed`, `run.hung`, `run.killed`
  - `job.completed`, `job.failed`, `job.killed`
- [x] All event types defined as constants in `internal/notifier/events.go`: run.*, job.*, usage.warning, webhook.disabled, worker.offline, worker.unhealthy, workspace.restricted, workspace.archived.
- [x] Test delivery endpoint: `POST /api/v1/webhook-subscriptions/{id}/test` — sends webhook.test event with HMAC signature, returns delivery status.
- [x] Tests: delivery, signature verification, filtering, auto-disable.

## T06.6 Health Endpoint
- [x] `GET /health` (unauthenticated):
  - Basic: `{ "status": "ok|degraded|unhealthy", "version": "...", "time": "..." }`
  - Minimal. No workspace details.
- [x] `GET /api/v1/workspace/health` — workspace-scoped health: process count, running/pending/failed runs.
- [x] No global admin health view as public endpoint.
- [x] Tests: health response.

## T06.7 Logging Backend Abstraction
- [x] `internal/logging/backend.go`: Backend interface with OutputChunk, Heartbeat, JobAttemptLog types. Methods: WriteRunOutput, WriteHeartbeat, WriteJobAttempt, QueryRunOutput, QueryHeartbeats, QueryJobAttempts, Close.
- [x] Database backend (`internal/logging/database/database.go`): Uses pgxpool to read/write run_output, heartbeats, and job_attempts tables directly. Default backend.
- [x] File backend (`internal/logging/file/file.go`): JSON-lines files organized by date and entity ID. Directories: output/, heartbeats/, attempts/. Thread-safe with mutex. 1MB line buffer for large output.
- [x] OpenSearch backend (`internal/logging/opensearch/opensearch.go`): HTTP-based client, monthly index rotation (croncontrol-output-YYYY.MM, etc.), basic auth, bool filter queries. Supports all read/write operations.
- [x] Metadata (state, progress) always in PostgreSQL regardless of backend.

## T06.8 Data Retention and Cleanup
- [x] `internal/cleanup/cleanup.go`:
  - Scheduled task (configurable, default daily).
  - Retention configurable per resource type (runs, jobs, audit, output, heartbeats).
  - Only terminal-state records eligible for deletion.
  - Batch processing (configurable batch size).
  - Log progress: records deleted per table per run.
- [x] Idempotency key released when job deleted by retention.
- [x] Manual cleanup API: `POST /api/v1/system/cleanup` — admin-only, triggers cleanup in background.
- [x] Tests: retention filtering, cascade, batch processing.

## T06.9 Prometheus Metrics
- [x] `internal/metrics/metrics.go`:
  - `processes_total` gauge
  - `runs_active` gauge (by state)
  - `runs_completed_total` counter
  - `runs_failed_total` counter
  - `run_duration_seconds` histogram
  - `jobs_active` gauge
  - `job_attempts` counter
  - `workers_online` gauge
  - `heartbeats_received_total` counter
  - `api_requests_total` counter (by method, path, status)
  - `api_request_duration_seconds` histogram
- [x] `internal/metrics/middleware.go`: HTTP middleware for request instrumentation.
- [x] Collector goroutine that updates gauges every 30 seconds.
- [x] `GET /metrics` endpoint.

## T06.10 Redaction and Masking
- [x] Secrets never in: API responses, logs, audit details, historical attempt snapshots.
- [x] API responses for credentials: return name, fingerprint, metadata — never raw key.
- [x] Attempt snapshots: redact sensitive fields from `effective_config` and `request` JSONB.
- [x] Log output: `MaskSecrets()` in `internal/logging/mask.go` — masks API keys (cc_live_*), worker creds (wrk_cred_*), Bearer tokens, password/secret key=value patterns, AWS keys.
- [x] Tests: redaction in all surfaces.

## Acceptance Checklist
- [x] Running work can report liveness and progress.
- [x] Alerts have stable payload contract and delivery policy.
- [x] Health endpoints expose actionable indicators.
- [x] Sensitive data masked according to policy.
- [x] Retention and cleanup visible, configurable, safe.
- [x] Product can explain recent failures and historical trends (3 logging backends implemented).
