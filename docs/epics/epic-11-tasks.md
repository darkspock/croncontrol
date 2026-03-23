# EPIC-11 Tasks: Async Execution Handles and Unified Cancellation

> Status: IN PROGRESS — compatibility refactor started 2026-03-23

## T11.1 Canonical Execution Contract
- [x] Replace `Execute()`-centric registry contract in [method.go](/Users/juanmacias/Projects/croncontrol/internal/executor/method.go) with `Start()`, `Poll()`, `Kill()`.
- [x] Introduce `StartParams`, `StartResult`, `PollCursor`, `PollResult`, and canonical `RemoteState`.
- [x] Add `IsAsync()` capability to methods.
- [x] Keep method-specific `Handle.Data` as JSON-safe durable state.
- [x] Add compatibility adapter for existing blocking methods so current backends keep working during migration.

## T11.2 Run and Job Persistence
- [x] Add `execution_handle JSONB` to `runs`.
- [x] Add `stdout_offset BIGINT NOT NULL DEFAULT 0` to `runs`.
- [x] Add `stderr_offset BIGINT NOT NULL DEFAULT 0` to `runs`.
- [x] Add equivalent fields to `jobs`.
- [x] Add query definitions and DB-layer methods to persist handle and update offsets.
- [ ] Regenerate sqlc artifacts from those query definitions.
- [x] Add migration and schema updates.
- [x] Remove temporary runtime raw-SQL helpers by moving handle/offset operations behind the DB layer.

## T11.3 Run Cancellation Semantics
- [x] Change `CancelRun` so it only sets `cancelled` for non-started states.
- [x] Route `CancelRun` on `running` to `kill_requested`.
- [ ] Keep `KillRun` explicit, but reuse the same internal path.
- [x] Stop marking runs as `killed` immediately inside `processKillRequests()`.
- [ ] Let terminal `killed` be decided only after polling confirms stop.

## T11.4 Job Cancellation Semantics
- [x] Add `kill_requested` handling to the queue processor lifecycle.
- [x] Change `CancelJob` semantics to mirror runs.
- [x] Stop treating running job cancel as immediate `cancelled`.
- [x] Add durable kill processing for jobs.

## T11.5 Orchestrator Redesign
- [ ] Split the current dispatch path into:
  - `startLoop`
  - `pollLoop`
  - `killLoop`
- [x] Persist handles after `Start()`.
- [x] Append incremental stdout/stderr from `Poll()`.
- [x] Finalize state only from polled terminal status for async executions.
- [x] Keep in-memory cancel funcs for immediate local interruption, but do not rely on them for durability.
- [x] Restore `Kill()` for persisted async run handles after process restarts.

## T11.6 Queue Processor Redesign
- [x] Refactor queue dispatch to use the same `Start/Poll/Kill` lifecycle as runs.
- [x] Persist job execution handles and offsets.
- [x] Align retries with async terminal detection rather than one blocking call.
- [x] Add async job kill processing based on persisted handles.

## T11.7 Worker Control Poll
- [x] Add `POST /api/v1/workers/control-poll`.
- [x] Worker sends active task IDs.
- [x] Server returns per-task commands such as `kill`.
- [x] Worker keeps active task registry with `cancel()` and handle metadata.
- [x] Worker applies local context cancel first, then `method.Kill()` if supported.
- [x] Keep task pickup on long polling.

## T11.8 Worker Binary Changes
- [x] Update worker binary to execute methods via `Start/Poll/Kill` compatibility layer.
- [x] Add active task registry keyed by task ID.
- [x] Implement control poll loop every `5s`.
- [x] Keep heartbeat every `15s`.
- [x] Preserve outbound-only connectivity.

## T11.9 HTTP Dispatch Modes
- [x] Add `dispatch_mode` support:
  - `sync`
  - `async_blind`
  - `async_tracked`
- [x] `sync`: keep request/response behavior.
- [x] `async_blind`: treat accepted dispatch as terminal success of dispatch only.
- [x] `async_tracked`: persist remote job handle from response.
- [x] Support configurable accepted status codes for async dispatch.
- [x] Support optional response extraction for `job_id`, `status_url`, `cancel_url`.

## T11.10 SSH Detached Execution
- [x] Replace blocking `session.Run()` path with detached wrapper support.
- [x] Standardize remote temp directory layout.
- [x] Return handle including `host`, `port`, `username`, `ssh_credential_id`, `base_path`, `pid`.
- [x] Implement incremental `Poll()` via remote file checks and ranged reads.
- [x] Implement PID-based `Kill()` with `TERM` then `KILL`.
- [x] Preserve optional foreground mode only if still needed explicitly.

## T11.11 SSM Detached Execution
- [x] Add detached wrapper support mirroring SSH.
- [x] Return handle including `instance_id`, `region`, `ssm_profile_id`, `base_path`, `pid`.
- [x] Implement `Poll()` via short SSM control commands.
- [x] Implement PID-based `Kill()` for detached mode.
- [x] Keep `CancelCommand` only for true foreground mode.

## T11.12 Kubernetes Handle Alignment
- [x] Refactor K8s method to use `Start()` and `Poll()`.
- [x] Persist handle with cluster, namespace, and job name.
- [x] Capture incremental pod logs with offsets/cursors where possible.
- [x] Ensure delete-based kill only requests stop; terminal state comes from monitor.

## T11.13 Observability and Audit
- [ ] Audit log kill request actor separately from terminal outcome.
- [ ] Record when remote confirmation of kill happened.
- [ ] Expose handle lifecycle and polling errors in logs/metrics.
- [ ] Add metrics for:
  - active async executions
  - poll failures
  - kill latency
  - kill success rate

## T11.14 API and SDK Alignment
- [x] Update OpenAPI for cancel/kill semantics.
- [x] Document `dispatch_mode` for HTTP.
- [ ] Expose handle-aware states in SDKs and CLI.
- [ ] Ensure CLI verbs do not imply false immediate termination.

## T11.15 Documentation
- [x] Update [product-specification.md](/Users/juanmacias/Projects/croncontrol/docs/product-specification.md) with canonical async execution contract.
- [x] Update [worker-guide.md](/Users/juanmacias/Projects/croncontrol/docs/worker-guide.md) with control polling and cancellation.
- [ ] Update queue/run docs to clarify `cancelled` vs `killed`.
- [ ] Add method-specific docs for HTTP async, SSH detached, and SSM detached.

## Acceptance Checklist
- [ ] Active executions survive server restarts without losing control.
- [ ] Running work is never marked `cancelled`.
- [ ] Running work is never marked `killed` before confirmation.
- [ ] Worker runtime can cancel active tasks while staying outbound only.
- [x] HTTP supports explicit sync and async behavior.
- [x] SSH and SSM support detached execution with PID-based kill.
- [x] Kubernetes follows the same durable handle lifecycle.
