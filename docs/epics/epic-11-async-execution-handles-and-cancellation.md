# EPIC-11: Async Execution Handles and Unified Cancellation

## Outcome

Redesign the execution plane around durable asynchronous handles so CronControl can start, monitor, and cancel work consistently across direct runtime and worker runtime for HTTP, SSH, SSM, and Kubernetes.

## Why This Epic Exists

The current execution model is still shaped like a blocking executor:

- `Execute()` assumes the caller waits for terminal completion.
- `cancelled`, `kill_requested`, and `killed` are not applied consistently.
- `http` is request/response only.
- `ssh` and `ssm` cannot safely support detached remote processes.
- Worker runtime can receive work, but cannot yet receive a proper kill signal for running tasks.
- In-memory handles are not enough for restarts, failover, or durable monitoring.

CronControl is becoming a control plane, not a local task runner. That requires a durable execution handle model with explicit start, poll, and kill phases.

## Goals

- Introduce a canonical async execution contract: `Start`, `Poll`, `Kill`.
- Persist execution handles and log offsets so monitoring survives process restarts.
- Make `cancel` and `kill` semantics valid against the state machine.
- Support detached remote execution for SSH and SSM using remote temp state and PID-based kill.
- Support fast worker-side cancellation without breaking the outbound-only network model.
- Allow HTTP execution to choose between synchronous and asynchronous dispatch modes.
- Keep method behavior explicit and auditable.

## In Scope

- New execution method interface
- Durable execution handle persistence
- Run monitoring loop for async executions
- Unified cancel/kill semantics for runs and jobs
- Worker control polling for kill signals
- HTTP `sync` vs `async` dispatch modes
- SSH detached execution with remote temp files and PID tracking
- SSM detached execution with remote temp files and PID tracking
- Kubernetes Job monitoring under the same async model
- Queue processor alignment with the same primitives

## Out of Scope

- Billing changes
- Frontend redesign beyond what is needed to expose async state correctly
- Container executor redesign beyond alignment with the same handle contract
- Rich workflow semantics outside execution and cancellation

## Canonical Model

Execution methods no longer behave as "run and wait" by default. They behave as controlled remote workloads with a durable handle.

### Canonical interface

```go
type Method interface {
    Start(ctx context.Context, params StartParams) (StartResult, error)
    Poll(ctx context.Context, handle Handle, cursor PollCursor) (PollResult, error)
    Kill(ctx context.Context, handle Handle) error
    SupportsKill() bool
    SupportsHeartbeat() bool
    IsAsync() bool
}
```

### Canonical phases

1. `Start`
   - Validate config
   - Dispatch the remote work
   - Return a durable handle
   - May also return initial log chunks

2. `Poll`
   - Check current remote state
   - Return incremental stdout/stderr chunks
   - Return terminal outcome when finished

3. `Kill`
   - Attempt to stop the remote work using the method-specific handle
   - Never marks the run as terminal by itself

## State Semantics

This epic clarifies the meaning of the canonical run states:

- `cancelled`
  - work was prevented from starting or was abandoned before active execution began
  - valid for `pending`, `queued`, `waiting_for_worker`, `paused`, and equivalent job states

- `kill_requested`
  - kill has been requested for active execution
  - not terminal
  - must remain active until the remote process is confirmed stopped or finishes on its own

- `killed`
  - the remote execution was actively stopped after it had started
  - terminal
  - cancels retries

### Rules

- `CancelRun` must not set `cancelled` on a `running` run.
- `CancelRun` on a running run becomes `kill_requested`.
- `processKillRequests()` must not mark a run as `killed` immediately after invoking `Kill()`.
- Final state is decided only by the monitor after remote confirmation.

The same semantic model applies to jobs.

## Durable Handle Persistence

In-memory handle tracking is not enough.

### New run fields

- `execution_handle JSONB`
- `stdout_offset BIGINT NOT NULL DEFAULT 0`
- `stderr_offset BIGINT NOT NULL DEFAULT 0`
- optional `monitor_after TIMESTAMPTZ`

### New job fields

- `execution_handle JSONB`
- `stdout_offset BIGINT NOT NULL DEFAULT 0`
- `stderr_offset BIGINT NOT NULL DEFAULT 0`
- optional `monitor_after TIMESTAMPTZ`

### Rules

- A handle must contain only the data needed to poll and kill the execution.
- A handle must be durable across CronControl restarts.
- A handle must be method-specific but shaped as JSON.

## Runtime Architecture

### Start path

1. Claim run/job
2. Transition to `running`
3. Call `Start()`
4. Persist handle and zeroed offsets
5. Enter monitored execution lifecycle

### Monitor path

A dedicated monitor loop polls active async executions:

- reads `execution_handle`
- calls `Poll()`
- appends new log chunks
- advances offsets
- transitions to terminal when remote state is terminal

### Kill path

1. API sets `kill_requested`
2. control plane or worker notices the signal
3. call `Kill()`
4. keep polling until terminal
5. mark `killed` only when confirmed

## Worker Runtime Model

The worker remains outbound only.

### Canonical networking rule

- No inbound connection from control plane to worker
- No customer-side open listener is required

### v1 cancellation design

Keep long polling for task pickup, and add a second outbound control poll:

- task poll: long poll, `30-60s`
- heartbeat: every `15s`
- control poll: every `5s`

Example control poll request:

```json
{
  "active_tasks": ["run_123", "job_456"]
}
```

Example response:

```json
{
  "commands": [
    {"task_id": "run_123", "action": "kill"}
  ]
}
```

### Worker rules

- The worker keeps an in-memory registry of active task cancel funcs and method handles.
- On `kill`, the worker first cancels the local context.
- If the method supports kill, the worker also calls `method.Kill()`.
- Worker-side kill must use the same state semantics as direct runtime.

This avoids requiring WebSockets in v1 while keeping cancellation latency acceptable.

## HTTP Method

HTTP needs explicit dispatch modes.

### Canonical config

- `dispatch_mode = "sync" | "async_blind" | "async_tracked"`

### `sync`

- current request/response behavior
- success is based on final response code
- no remote lifecycle beyond the HTTP request

### `async_blind`

- CronControl sends the request and only verifies dispatch acceptance
- terminal success means the dispatch was accepted, not that remote work completed
- requires explicit accepted status rules, e.g. `202`
- no later poll or kill unless the remote API returns a usable handle

### `async_tracked`

- remote API returns a durable remote handle such as:
  - `job_id`
  - `status_url`
  - `cancel_url`
- CronControl stores that handle in `execution_handle`
- `Poll()` and `Kill()` use those URLs or identifiers

### Rules

- `http` remains valid for fully synchronous webhooks
- `http async` must be explicit in config
- `http async_blind` is dispatch tracking, not remote execution tracking

## SSH Method

SSH must support detached execution.

### Canonical detached model

The command runs via a wrapper script that:

- creates a run-specific temp directory
- launches the process in background
- writes `pid`
- writes `stdout` and `stderr` to files
- writes `exit_code` on completion
- returns handle metadata immediately

### Canonical remote layout

```text
/var/tmp/croncontrol/<run_id>/
  pid
  stdout.log
  stderr.log
  exit_code
  started_at
  finished_at
```

### Handle data

- `host`
- `port`
- `username`
- `ssh_credential_id`
- `base_path`
- `pid`

### Kill behavior

- send `TERM` to PID
- after grace period, send `KILL` if still alive
- terminal state becomes `killed` only after remote confirmation

## SSM Method

SSM must use the same detached model as SSH when async.

### Rules

- `CancelCommand` is only valid for foreground SSM execution
- detached SSM execution must kill the actual remote PID, not just the command envelope
- start, poll, and kill are implemented by short SSM control commands against the same temp directory

### Handle data

- `instance_id`
- `region`
- `ssm_profile_id`
- `base_path`
- `pid`

## Kubernetes Method

Kubernetes already behaves like a remote durable handle and should align naturally with the new model.

### Handle data

- `k8s_cluster_id`
- `namespace`
- `job_name`
- optional `pod_name`

### Rules

- `Start()` creates the Job and persists the handle
- `Poll()` watches Job status and captures incremental logs
- `Kill()` deletes the Job
- final terminal state is decided by the monitor, not by the kill request itself

## Queue and Job Alignment

Jobs must use the same start/poll/kill model as runs.

### Rules

- `CancelJob` on `pending` remains `cancelled`
- a running job must transition via `kill_requested`
- job attempt lifecycle must not assume one blocking `Execute()` call
- queues must persist execution handles and offsets exactly like runs

## Migration Constraints

- Existing `Execute()` implementations may be adapted incrementally behind shims, but the canonical model becomes async-handle based.
- `http sync` may remain internally blocking for simplicity, but it must comply with the same terminal semantics.
- Any method that cannot return a durable handle cannot claim tracked async execution.

## Acceptance Criteria

- Runs and jobs use a durable start/poll/kill model.
- `cancelled`, `kill_requested`, and `killed` are applied consistently with the state machine.
- CronControl survives restart without losing remote execution tracking.
- Worker runtime supports cancellation without introducing inbound networking.
- SSH and SSM support detached execution with PID-based kill.
- HTTP supports explicit synchronous and asynchronous dispatch modes.
- Kubernetes uses the same persistent handle lifecycle.
- The API and code no longer mark active work as `killed` before remote confirmation.

## Dependencies

- [EPIC-03 Execution Plane and Worker Runtime](epic-03-execution-plane-and-worker-runtime.md)
- [EPIC-04 Durable Queue and Replay](epic-04-durable-queue-and-replay.md)
- [EPIC-06 Observability, Alerts, and Data Lifecycle](epic-06-observability-alerts-and-data-lifecycle.md)

## Follow-on Impact

This epic affects:

- worker runtime
- monitoring
- queue processor
- run/job APIs
- dashboard state display
- replay correctness
- timeout handling
- orchestration semantics built on top of execution state
