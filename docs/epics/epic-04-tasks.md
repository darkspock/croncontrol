# EPIC-04 Tasks: Durable Queue and Replay

> Status: DONE (95%) — audited 2026-03-18

## T04.1 Queue Configuration
- [x] CRUD for queues: workspace-scoped, unique name.
- [x] Queue defines: execution_method, runtime, method_config, concurrency, max_attempts, retry_backoff, job_timeout, max_response_size.
- [x] Queue references reusable credentials (ssh_credential_id, ssm_profile_id, k8s_cluster_id).
- [x] Queue defines default worker routing (worker_id, worker_labels).
- [x] Jobs may override runtime and worker selectors but NOT execution_method.
- [x] Queue tags inherited by jobs at creation time (snapshot).
- [x] Tests: CRUD, validation.

## T04.2 Job Intake
- [x] `POST /api/v1/jobs` — single job enqueue.
- [x] Job fields: queue_id, payload, priority, max_attempts (override), scheduled_at (delayed), expires_at, reference, idempotency_key.
- [x] Runtime overrides: runtime_override, worker_id_override, worker_labels_override.
- [x] Actor tracking: actor_type, actor_id stored on job.
- [x] Tags snapshot from queue at creation time.
- [x] `POST /api/v1/jobs/batch` — atomic batch enqueue with transaction. Single conflict/validation failure fails entire batch.
- [x] Tests: single enqueue, delayed, with overrides.

## T04.3 Idempotency
- [x] `idempotency_key` is workspace-scoped.
- [x] Duplicate key returns HTTP 409 Conflict with `existing_job_id` in error response.
- [x] Key exists as long as job exists in DB (released on retention cleanup).
- [x] Batch: single duplicate fails entire batch (tx rollback on idempotency conflict at any index).
- [x] Tests: duplicate rejection, 409 response.

## T04.4 Queue Processor
- [x] `internal/queue/processor.go`:
  - Background goroutine, tick every 5s.
  - `ClaimPendingJobs` using `SELECT FOR UPDATE SKIP LOCKED`.
  - For each claimed job:
    1. Check workspace state (must be `active`).
    2. Check queue concurrency limit (`CountRunningByQueue`).
    3. Check global and per-method concurrency.
    4. If worker needed and unavailable: `waiting_for_worker`.
    5. Snapshot config on first attempt.
    6. Dispatch to execution method.
    7. Record attempt in `job_attempts`.
    8. On success: state `completed`.
    9. On failure: if attempts remain → `retrying`, calculate `next_attempt_at`. Else → `failed`.
- [x] Response truncation: if response > `max_response_size`, truncate, set `truncated=true`, store `original_size`.
- [x] Tests: processing, concurrency, retry, truncation.

## T04.5 Job Retry Model
- [x] `max_attempts` includes first attempt.
- [x] `retry_backoff`: list of durations from queue (job can override).
- [x] `retrying` is a job-level state while retries remain.
- [x] `next_attempt_at` calculated from backoff list.
- [x] Retries count toward monthly usage.
- [x] `killed` cancels retries.
- [x] Tests: retry progression, backoff, killed stops retries.

## T04.6 Job Expiration
- [x] Jobs with `expires_at` that are still `pending` after expiration → `cancelled` with reason "expired".
- [x] Periodic scan: `ListExpiredPendingJobs`.
- [x] Tests: expiration cancellation.

## T04.7 Job State Machine
- [x] States: pending, waiting_for_worker, running, retrying, kill_requested, completed, failed, killed, cancelled.
- [x] Transitions:
  ```
  pending → waiting_for_worker → running → completed/failed/killed
  pending → running (direct)
  pending → cancelled (manual or expired)
  running → retrying (if attempts remain)
  running → kill_requested → killed
  retrying → running (next attempt)
  retrying → failed (max attempts)
  retrying → cancelled
  retrying → killed
  ```
- [x] State transition validation.
- [x] Tests: all transitions.

## T04.8 Job Replay
- [x] Replay allowed only for terminal jobs (completed, failed, killed, cancelled).
- [x] Replay creates NEW job: copies queue_id, payload, priority, max_attempts, retry_backoff, reference, tags.
- [x] NOT copied: scheduled_at (now), idempotency_key, state (pending), attempt (0).
- [x] User can override: payload, priority.
- [x] Stores `replayed_from_job_id`.
- [x] Replay uses original effective snapshot, not current queue config.
- [x] Actor tracked on replay job.
- [x] Tests: replay, overrides, lineage.

## T04.9 Attempt History
- [x] Each attempt stored as `job_attempt` record.
- [x] Fields: attempt_number, started_at, finished_at, duration_ms, request (JSONB), response_code, response_headers, response_body, truncated, original_size, error_message, worker_id.
- [x] Request JSONB stores redacted version (no raw secrets in historical attempts).
- [x] Attempts are nested in job detail response, not separate public endpoints.
- [x] Tests: attempt recording, redaction.

## T04.10 Queue Stats
- [x] `GetQueueStats`: pending, running, retrying, failed (24h), completed (24h).
- [x] Dashboard shows: queue health indicator, per-queue counts.
- [x] Tests: stats accuracy.

## Acceptance Checklist
- [x] Jobs move through clear durable lifecycle.
- [x] Retry behavior is deterministic and configurable.
- [x] Failed jobs can be replayed with lineage.
- [x] Attempt history is queryable and safe to expose.
- [x] Queue processing respects concurrency and plan constraints.
- [x] Idempotency model is explicit and consistent with workspace isolation.
