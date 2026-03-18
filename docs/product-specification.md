# CronControl Product Specification

## Purpose

This document defines the canonical product model for CronControl. It is the source of truth for public terminology, resource names, execution behavior, and core product contracts.

## Product Scope

CronControl is a hosted multi-tenant SaaS control plane for:

- Scheduled operational work
- Durable background queues
- Infrastructure-oriented execution across multiple methods
- Auditability, replay, health visibility, and event delivery
- Agentic/API-first adoption

There is no self-hosted mode in the canonical product model.

## Public Terminology

Use these terms in product, API, dashboard, and SDKs:

- `workspace`
- `user`
- `process`
- `run`
- `queue`
- `job`
- `worker`
- `webhook subscription`

Internal-only implementation terms:

- `tenant`
- `scheduled_slot`

## Canonical Resources

The public resource map is:

- `workspaces`
- `users`
- `workspace_memberships`
- `workers`
- `processes`
- `runs`
- `queues`
- `jobs`
- `webhook_subscriptions`
- `api_keys`
- `ssh_credentials`
- `ssm_profiles`
- `k8s_clusters`

## Identifiers

All canonical resource IDs use `prefix + ULID` and are also the primary database IDs.

Prefix set:

- `wsp_` workspace
- `usr_` user
- `wmb_` workspace membership
- `wrk_` worker
- `prc_` process
- `run_` run
- `rat_` run attempt
- `que_` queue
- `job_` job
- `jat_` job attempt
- `whs_` webhook subscription
- `key_` api key
- `ssh_` ssh credential
- `ssp_` ssm profile
- `k8c_` k8s cluster

## Workspaces, Users, and Memberships

- Public organizational term: `workspace`
- Public role model: `admin`, `operator`, `viewer`
- One role per `workspace_membership`
- A user may belong to multiple workspaces
- `users.active_workspace_id` is persisted in the database
- `workspace.slug` is globally unique, auto-generated from name, editable only at creation, then immutable
- There must always be at least one workspace admin

Email and identity rules:

- One global user identity per email
- Google OAuth and email/password can be attached to the same user
- Password policy: minimum 12 characters, simple passphrase-friendly rules
- Password reset tokens expire in 1 hour, are single-use, and invalidate prior tokens
- Dashboard sessions are rolling 30-day sessions
- Google OAuth allows any valid Google account by default
- Email verification is required before any real execution or queue dispatch

Workspace states:

- `active`
- `suspended`
- `archived`

Lifecycle rules:

- `suspended` workspaces block all execution
- `archived` is recoverable only by admin
- Runs missed during `suspended` periods are skipped, not recovered

## API Keys

- API keys are workspace-scoped
- Single API key model only; no separate machine key types
- Initial workspace bootstrap API key is `admin`
- API keys have optional expiration and no expiry by default
- API keys are shown only once at creation
- Metadata tracked:
  - `last_used_at`
  - `last_ip`
  - `last_user_agent`

## API Conventions

- Versioning via path: `/api/v1/...`
- Resource endpoints use plural nouns
- Workspace scoping is implicit
- The public API uses `runs`, not `slots`
- Consistent response envelope:
  - success list: `{ data, meta }`
  - success single: `{ data }`
  - error: `{ error }`
- Error payload fields:
  - `code`
  - `message`
  - `hint`
  - `details` optional
- Validation details use field-level structured errors
- Rate limiting returns `429` with a structured error

## Scheduling Model

Public schedule types:

- `cron`
- `fixed_delay`
- `on_demand`

Rules:

- `cron` uses standard 5-field cron syntax
- `fixed_delay` uses explicit durations only, such as `30s`, `5m`, `1h`
- If a process does not define timezone, it inherits `workspace.default_timezone`
- Default workspace timezone is `UTC`

DST rules:

- Non-existent local time: skip
- Duplicated local time: execute once on the first occurrence

## Processes and Runs

`process.name` is unique per workspace.

Public run origins:

- `cron`
- `fixed_delay`
- `manual`
- `one_time`
- `recovery`
- `dependency`
- `replay`

Important distinction:

- `schedule_type` describes how a process creates work
- `run.origin` describes why a specific run exists

Manual and replay rules:

- Manual trigger creates an independent run and does not alter future schedule planning
- Manual trigger allows controlled overrides
- Replay exists for terminal runs
- Run replay uses the original effective snapshot, not the current process config
- Run replay supports controlled overrides
- Replays store `replayed_from_run_id`
- Manual and replay actions store actor identity
- Automatic actions use actor `system`

Configuration snapshot rules:

- Latest process configuration applies until the first attempt starts
- On first attempt, the effective configuration is snapshotted
- All retries for that run use the same snapshot

## Run Attempts and Retry Model

Runs support automatic retry.

Canonical rules:

- `max_attempts` includes the first attempt
- Default `max_attempts = 1`
- `retry_backoff` is a list of explicit durations
- Retries count toward monthly usage
- `retrying` is a run-level state while retries remain
- `scheduled_at` stays unchanged
- `next_attempt_at` stores the next retry time
- `hung` is retryable if attempts remain
- `killed` cancels retries
- Interval processes create the next run only after the final terminal outcome of the current run
- Dependency checks use the final terminal outcome, including retries

Overlap and concurrency rules:

- If `allow_parallel = false`, `retrying` still blocks overlap
- If `allow_parallel = true`, `retrying` counts toward `max_parallel`
- `waiting_for_worker` does not consume concurrency
- Queued runs store `queue_reason`
- Waiting runs store `waiting_reason`

Run states:

- `pending`
- `waiting_for_worker`
- `queued`
- `running`
- `retrying`
- `kill_requested`
- `completed`
- `failed`
- `hung`
- `killed`
- `skipped`
- `cancelled`
- `paused`

## Queues and Jobs

`queue.name` is unique per workspace.

Queue defaults:

- Queue defines execution method and default runtime
- Jobs may override runtime and worker selectors
- Jobs may not override execution method

Idempotency:

- `idempotency_key` is scoped to the workspace
- Duplicate `idempotency_key` returns `409 Conflict`
- Error includes `existing_job_id`

Batch enqueue:

- Atomic only
- A single conflict or validation failure fails the entire batch

Replay:

- Job replay is allowed only for terminal jobs
- Replay uses the original effective snapshot
- Replay supports controlled overrides
- Replay stores `replayed_from_job_id`

Job states:

- `pending`
- `waiting_for_worker`
- `running`
- `retrying`
- `kill_requested`
- `completed`
- `failed`
- `killed`
- `cancelled`

## Execution Methods and Runtimes

Canonical execution methods:

- `http`
- `ssh`
- `ssm`
- `k8s`

Canonical runtimes:

- `direct`
- `worker`

Rules:

- Default runtime is `direct`
- `direct` is for destinations reachable from the SaaS control plane
- `worker` is for private or internal connectivity inside the customer environment
- Both runtimes are valid for all execution methods, subject to plan and availability

## Worker Model

The worker is a workspace-scoped runtime and network gateway.

Core rules:

- A worker belongs to one workspace only
- Communication is outbound only
- Initial implementation uses long polling
- Worker credentials are separate from normal API keys
- Enrollment uses a temporary enrollment token and results in a dedicated worker credential
- Capabilities are auto-reported by the worker
- Labels are admin-managed
- Routing supports:
  - explicit `worker_id`
  - label matching
  - automatic least-loaded compatible worker selection
- Workers can be distributed as native binaries and Docker images
- Compatibility checks may block dispatch if the worker version is too old

Operational fields:

- `enabled/disabled` admin control
- derived status:
  - `online`
  - `offline`
  - `unhealthy`
- heartbeat every 15 seconds
- offline after 60 seconds without signal
- unhealthy after 5 consecutive failures
- returns to online after 3 healthy checks
- worker-defined `max_concurrency`

Routing behavior:

- If no selector is given, CronControl may auto-pick a compatible worker
- If `worker_id` is present, it overrides labels
- If labels match multiple workers, choose the least-loaded compatible worker
- `waiting_for_worker` is used when no compatible worker is currently available
- `waiting_reason` must be recorded

Failure behavior:

- Pending work can be reassigned if it was not pinned to a specific worker
- Running work that loses its worker becomes `hung` after liveness timeout
- No late reconciliation reopens a hung run
- Disabled workers stop receiving new assignments but can still be tested

## Method-Specific Rules

### HTTP

- Success is `2xx` only
- No automatic redirects by default
- Body allowed only on `POST`, `PUT`, `PATCH`
- Header names are normalized case-insensitively
- Supports simple built-in template variables only
- `{{now}}` renders as UTC ISO 8601
- HTTP is request/response only
- No async `202` lifecycle in the canonical model
- Heartbeat/progress does not apply to HTTP
- HTTP config remains inline rather than reusable

### SSH

- Key-based auth only
- Strict host key verification
- SSH credentials are reusable workspace resources
- Host/target stays inline on the process or queue

### SSM

- Targeting supports instance IDs and tags
- The target must resolve to exactly one instance
- SSM profiles are reusable workspace resources
- Targeting details remain inline on the process or queue

### K8s

- Always creates a Kubernetes Job
- `k8s_cluster` is a reusable workspace resource
- Resource may define an optional default namespace
- Process or queue may override namespace
- Only a controlled subset of Job fields is configurable
- Pod logs are captured as output

### Credentials

- Managed credentials and worker-local credentials are both supported
- Managed credentials override worker-local credentials when both exist
- Secrets are encrypted at rest and redacted in API, UI, logs, and historical attempts

## Templating

Templating is simple variable substitution only.

No conditionals, loops, or advanced logic.

Supported variable families include:

- `workspace.*`
- `process.*`
- `queue.*`
- `run.*`
- `job.*`
- `attempt.number`
- `now`

## Observability and Output

- Heartbeat/progress is supported for non-HTTP methods
- `duration_ms` is stored for runs and job attempts
- Attempts are nested inside run/job detail responses, not separate public resources
- Dashboard should show run state and latest attempt summary during `retrying`
- Output retrieval is simple and size-limited in the first version
- Truncated output must expose `truncated = true` and original size estimate when available

## Events and Webhooks

There is one unified event webhook system.

Core rules:

- Multiple webhook subscriptions per workspace
- Secret per subscription
- Filter by event type only in the first version
- HMAC-SHA256 signatures
- Headers:
  - `X-CronControl-Signature`
  - `X-CronControl-Timestamp`
  - `X-CronControl-Delivery-Id`
- Delivery guarantee is at-least-once
- Auto-disable after 20 consecutive failures
- Reactivation is manual only
- Test delivery endpoint is supported

Event families include:

- `run.*`
- `job.*`
- `usage.warning`
- `webhook.disabled`
- worker lifecycle events such as `worker.offline` and `worker.unhealthy`
- workspace lifecycle events such as `workspace.restricted` and `workspace.archived`

## Health

- Public basic health is unauthenticated
- Basic health returns minimal JSON:
  - `status`
  - `version`
  - `time`
- Public API exposes workspace health only
- A global admin health view may exist internally but is not a public product endpoint

## Permissions

`viewer`

- Can view runs, jobs, outputs, attempts, and redacted snapshots
- Cannot mutate operational or sensitive configuration

`operator`

- Can trigger runs
- Can replay runs and jobs
- Can kill runs and jobs
- Can pause and resume processes
- Can enqueue jobs into existing queues
- Cannot edit processes, queues, workers, or sensitive resources

`admin`

- Full configuration and governance authority
- Manages users, memberships, workers, API keys, webhook subscriptions, and reusable credentials/resources

Sensitive resource management is admin-only for:

- workers
- webhook subscriptions
- ssh credentials
- ssm profiles
- k8s clusters

## Audit Rules

- Audit all mutations
- Audit sensitive reads
- Do not audit every ordinary list/read action
- Use unified actor model:
  - `actor_type`: `user`, `api_key`, `worker`, `system`
  - `actor_id`

## Search and Filtering

- Default list ordering is newest first
- Lists use `page/per_page`
- Runs support filters including origin
- Tag filters are simple exact-match filters
- Runs and jobs store snapshots of inherited tags from their source process or queue
