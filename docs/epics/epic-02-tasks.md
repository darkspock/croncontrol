# EPIC-02 Tasks: Scheduling and Orchestration Core

> Status: DONE (98%) — audited 2026-03-18

## T02.1 Planner Component
- [x] `internal/planner/planner.go`:
  - Background goroutine with configurable tick interval (default 1h).
  - Planning horizon configurable (default 24h).
  - Only handles `schedule_type = 'cron'` processes.
  - For each enabled cron process in active workspaces:
    1. Evaluate cron expression with process timezone (or workspace default).
    2. Check if run already exists for (process_id, scheduled_at, origin='cron').
    3. Create run with `state='pending'`, `origin='cron'`, `max_attempts` from process.
  - Idempotent: duplicate runs never created.
- [x] Cron expression parsing: `robfig/cron/v3` with timezone support.
- [x] DST handling: non-existent local time = skip, duplicated time = execute once on first occurrence.
- [x] Unit tests: slot creation, idempotency, DST edge cases.

## T02.2 Fixed-Delay Chain
- [x] After any run reaches a **final terminal state** (no retries remaining):
  - If process `schedule_type = 'fixed_delay'`:
    - All terminal states continue chain EXCEPT `paused`.
    - Create next run: `scheduled_at = finished_at + delay_duration`, `origin = 'fixed_delay'`.
    - `paused` state stops the chain.
- [x] On process enable/resume: create initial run with `scheduled_at = now`, `origin = 'fixed_delay'`.
- [x] Important: interval chain waits for **final** outcome (including retries). Don't create next run on first failure if retries remain.
- [x] Tests: chain continuation, paused stops chain, resume restarts chain.

## T02.3 On-Demand Processes
- [x] Processes with `schedule_type = 'on_demand'` are never auto-planned.
- [x] Only executed via manual trigger, one-time scheduling, or dependency trigger.
- [x] Planner ignores them completely.

## T02.4 Missed-Run Recovery
- [x] On CronControl startup:
  - **Cron processes** with `miss_policy = 'execute'`:
    - Compare last completed/failed run with expected schedule.
    - Create recovery runs for missed times, capped by `max_recovery_slots`.
    - Origin: `recovery`.
  - **Cron processes** with `miss_policy = 'skip'`: only plan future runs.
  - **Fixed-delay processes**:
    - If last run completed and no pending run exists: create recovery run at `now`.
    - If last run was `running` (orphaned): mark as `hung`, then create recovery run.
  - **Orphan detection**: all `running` runs from before restart → mark `hung`.
- [x] Runs missed during `restricted` or `suspended` workspace periods are skipped, not recovered.
- [x] Tests: recovery capping, orphan detection, workspace state filtering.

## T02.5 Executor Orchestrator
- [x] `internal/executor/orchestrator.go`:
  - Background goroutine with configurable tick interval (default 30s).
  - `ClaimPendingRuns` using `SELECT FOR UPDATE SKIP LOCKED`.
  - Also `ClaimRetryingRuns` for runs in `retrying` state where `next_attempt_at <= now`.
  - For each claimed run:
    1. Check workspace state (must be `active`).
    2. Check global and per-method concurrency limits.
    3. Check per-process parallelism (`allow_parallel`, `max_parallel`).
    4. If blocked by parallelism and `on_overlap = 'queue'`: update state to `queued`, store `queue_reason`.
    5. If blocked by parallelism and `on_overlap = 'skip'`: update state to `skipped`.
    6. If worker runtime needed and no worker available: state `waiting_for_worker`, store `waiting_reason`.
    7. If allowed: snapshot config on first attempt, dispatch to execution target, state `running`.
- [x] `retrying` state still blocks overlap if `allow_parallel = false`.
- [x] `waiting_for_worker` does NOT consume concurrency.
- [x] Worker dispatch integration: orchestrator transitions to `waiting_for_worker` when `runtime = "worker"`, dispatcher picks up via its own polling.
- [x] Tests: concurrency limits, parallelism rules, queue/skip/wait behavior.

## T02.6 Run State Machine
- [x] Implement and enforce all 13 state transitions:
  ```
  pending → waiting_for_worker → running → completed/failed/hung/killed
  pending → queued → running → ...
  pending → skipped
  pending → cancelled
  pending → paused
  queued → cancelled
  queued → paused
  running → retrying (if attempts remain)
  running → kill_requested → killed
  retrying → running (next attempt)
  retrying → failed (max attempts exhausted)
  retrying → cancelled
  retrying → killed
  paused → pending (on resume)
  hung → retrying (if attempts remain)
  ```
- [x] `kill_requested` is a transitional state: executor sees it, performs kill, transitions to `killed`.
- [x] `killed` cancels all remaining retries.
- [x] Create state transition validation function: `ValidateTransition(from, to) error`.
- [x] Helper functions: `IsTerminal()`, `IsFinalTerminal()`, `IsActive()`, `BlocksOverlap()`, `ContinuesFixedDelayChain()`.
- [x] Tests: all valid transitions, invalid transitions rejected.

## T02.7 Run Retry Model
- [x] `max_attempts` includes the first attempt (default 1 = no retry).
- [x] `retry_backoff` is a list of durations.
- [x] On failure: if `attempt < max_attempts`, state → `retrying`, calculate `next_attempt_at`.
- [x] On `hung`: retryable if attempts remain.
- [x] On `killed`: cancels retries (terminal).
- [x] Retries count toward monthly usage.
- [x] `scheduled_at` stays unchanged across retries.
- [x] Tests: retry progression, backoff timing, hung retry, killed stops retries.

## T02.8 Configuration Snapshot
- [x] On first attempt start: snapshot effective config into `runs.effective_config` (JSONB).
- [x] All retries use the snapshotted config, not current process config.
- [x] Snapshot includes: method_config, environment, execution_method, runtime, credential references, timeout settings.
- [x] Tests: config change between attempts uses snapshot.

## T02.9 Dependencies
- [x] `internal/dependency/resolver.go`:
  - After run reaches **final terminal state**: check `GetDependentProcesses`.
  - `dependency_type = 'after'`: create run for dependent process.
  - `dependency_type = 'after_success'` AND parent `completed`: create run.
  - `dependency_type = 'after_success'` AND parent NOT `completed`: no run created.
  - Origin: `dependency`. Store `triggered_by_run_id`.
  - Dependency checks use final terminal outcome (including retries).
- [x] Circular dependency validation at process create/update time (`ValidateNoCycles`).
- [x] Tests: after, after_success, circular rejection, retry-then-succeed triggers dependency.

## T02.10 Pause and Resume
- [x] Pause a process:
  - Mark process `enabled = false`.
  - `cancel_pending = false` (default): pending runs → `paused`.
  - `cancel_pending = true`: pending runs → `cancelled`.
  - `queued` runs → `paused` or `cancelled` (same logic).
  - `running` runs are NEVER affected.
  - Fixed-delay chain stops (no next run created when current finishes with `paused`).
- [x] Resume a process:
  - Mark process `enabled = true`.
  - `paused` runs → `pending`.
  - Planner generates future runs / fixed-delay creates initial run.
- [x] Tests: pause/resume with cancel_pending variants, running not affected.

## T02.11 One-Time Scheduling
- [x] API allows creating a run for any process at a specific datetime.
- [x] Origin: `one_time`.
- [x] Follows normal execution rules (parallelism, concurrency).
- [x] Tests: one-time run creation and execution.

## Acceptance Checklist
- [x] Cron schedules plan future runs idempotently.
- [x] Fixed-delay schedules create next run from terminal state.
- [x] Missed-run recovery has explicit caps and deterministic behavior.
- [x] Dependencies validated for cycles and execute predictably.
- [x] Run lifecycle states and transitions are unambiguous.
- [x] Timezone and DST behavior documented and tested.
