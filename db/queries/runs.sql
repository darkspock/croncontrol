-- name: CreateRun :one
INSERT INTO runs (
    id, workspace_id, process_id, scheduled_at, state, origin,
    max_attempts, actor_type, actor_id,
    triggered_by_run_id, replayed_from_run_id,
    runtime, worker_id, tags
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9,
    $10, $11,
    $12, $13, $14
)
RETURNING *;

-- name: GetRun :one
SELECT * FROM runs WHERE id = $1 AND workspace_id = $2;

-- name: GetRunWithProcess :one
SELECT r.*, p.name AS process_name, p.execution_method, p.method_config,
       p.execution_timeout, p.heartbeat_timeout, p.timeout_action,
       p.max_attempts AS process_max_attempts, p.retry_backoff AS process_retry_backoff
FROM runs r
JOIN processes p ON p.id = r.process_id
WHERE r.id = $1 AND r.workspace_id = $2;

-- name: ClaimPendingRuns :many
SELECT r.*
FROM runs r
JOIN workspaces w ON w.id = r.workspace_id
WHERE r.state IN ('pending', 'queued')
  AND r.scheduled_at <= now()
  AND w.state = 'active'
ORDER BY r.scheduled_at
LIMIT $1
FOR UPDATE OF r SKIP LOCKED;

-- name: ClaimRetryingRuns :many
SELECT r.*
FROM runs r
JOIN workspaces w ON w.id = r.workspace_id
WHERE r.state = 'retrying'
  AND r.next_attempt_at <= now()
  AND w.state = 'active'
ORDER BY r.next_attempt_at
LIMIT $1
FOR UPDATE OF r SKIP LOCKED;

-- name: UpdateRunState :exec
UPDATE runs SET
    state = $2,
    started_at = COALESCE(sqlc.narg('started_at'), started_at),
    finished_at = COALESCE(sqlc.narg('finished_at'), finished_at),
    duration_ms = COALESCE(sqlc.narg('duration_ms'), duration_ms),
    exit_code = COALESCE(sqlc.narg('exit_code'), exit_code),
    attempt = COALESCE(sqlc.narg('attempt'), attempt),
    next_attempt_at = sqlc.narg('next_attempt_at'),
    queue_reason = sqlc.narg('queue_reason'),
    waiting_reason = sqlc.narg('waiting_reason'),
    killed_by_actor_type = COALESCE(sqlc.narg('killed_by_actor_type'), killed_by_actor_type),
    killed_by_actor_id = COALESCE(sqlc.narg('killed_by_actor_id'), killed_by_actor_id),
    worker_id = COALESCE(sqlc.narg('worker_id'), worker_id),
    updated_at = now()
WHERE id = $1;

-- name: UpdateRunProgress :exec
UPDATE runs SET
    progress_total = $2,
    progress_current = $3,
    progress = $4,
    progress_message = sqlc.narg('progress_message'),
    last_heartbeat_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: SnapshotRunConfig :exec
UPDATE runs SET effective_config = $2, updated_at = now() WHERE id = $1;

-- name: SetRunChoiceConfig :exec
UPDATE runs SET choice_config = $2, updated_at = now() WHERE id = $1;

-- name: SetRunChosenIndex :exec
UPDATE runs SET chosen_index = $2, updated_at = now() WHERE id = $1;

-- name: SaveRunExecutionHandle :exec
UPDATE runs SET
    execution_handle = $2,
    stdout_offset = 0,
    stderr_offset = 0,
    updated_at = now()
WHERE id = $1;

-- name: ClearRunExecutionHandle :exec
UPDATE runs SET
    execution_handle = NULL,
    stdout_offset = 0,
    stderr_offset = 0,
    updated_at = now()
WHERE id = $1;

-- name: UpdateRunOffsets :exec
UPDATE runs SET
    stdout_offset = $2,
    stderr_offset = $3,
    updated_at = now()
WHERE id = $1;

-- name: GetRunExecutionHandle :one
SELECT execution_handle, stdout_offset, stderr_offset
FROM runs
WHERE id = $1 AND execution_handle IS NOT NULL;

-- name: ListActiveAsyncRuns :many
SELECT
    r.id,
    r.workspace_id,
    r.process_id,
    r.attempt,
    r.state,
    r.started_at,
    COALESCE(ra.id, '') AS attempt_id,
    r.execution_handle,
    r.stdout_offset,
    r.stderr_offset,
    p.execution_method
FROM runs r
JOIN processes p ON p.id = r.process_id
LEFT JOIN LATERAL (
    SELECT id
    FROM run_attempts
    WHERE run_id = r.id AND finished_at IS NULL
    ORDER BY attempt_number DESC
    LIMIT 1
) ra ON true
WHERE r.state IN ('running', 'kill_requested')
  AND r.execution_handle IS NOT NULL;

-- name: ListRuns :many
SELECT r.*, p.name AS process_name
FROM runs r
JOIN processes p ON p.id = r.process_id
WHERE r.workspace_id = $1
  AND ($2 = '' OR r.process_id = $2)
  AND ($3 = '' OR r.state = $3)
  AND ($4 = '' OR r.origin = $4)
ORDER BY r.created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountRuns :one
SELECT count(*) FROM runs
WHERE workspace_id = $1
  AND ($2 = '' OR process_id = $2)
  AND ($3 = '' OR state = $3)
  AND ($4 = '' OR origin = $4);

-- name: CountActiveByProcess :one
SELECT count(*) FROM runs
WHERE process_id = $1 AND state IN ('running', 'retrying', 'waiting_for_worker');

-- name: CountRunningByWorkspace :one
SELECT count(*) FROM runs WHERE workspace_id = $1 AND state = 'running';

-- name: ListRunningRuns :many
SELECT r.*, p.execution_timeout, p.heartbeat_timeout, p.timeout_action, p.execution_method
FROM runs r
JOIN processes p ON p.id = r.process_id
WHERE r.state = 'running';

-- name: ListKillRequestedRuns :many
SELECT r.*, p.execution_method, p.runtime
FROM runs r
JOIN processes p ON p.id = r.process_id
WHERE r.state = 'kill_requested';

-- name: GetLastRunByProcess :one
SELECT * FROM runs
WHERE process_id = $1
ORDER BY scheduled_at DESC
LIMIT 1;

-- name: RunExists :one
SELECT EXISTS(
    SELECT 1 FROM runs
    WHERE process_id = $1 AND scheduled_at = $2 AND origin = $3
);

-- name: BulkPauseRuns :execrows
UPDATE runs SET state = 'paused', updated_at = now()
WHERE process_id = $1 AND workspace_id = $2 AND state = 'pending';

-- name: BulkCancelRuns :execrows
UPDATE runs SET state = 'cancelled', updated_at = now()
WHERE process_id = $1 AND workspace_id = $2 AND state = 'pending';

-- name: BulkResumeRuns :execrows
UPDATE runs SET state = 'pending', updated_at = now()
WHERE process_id = $1 AND workspace_id = $2 AND state = 'paused';

-- name: ListUpcomingRuns :many
SELECT r.*, p.name AS process_name
FROM runs r
JOIN processes p ON p.id = r.process_id
WHERE r.workspace_id = $1 AND r.state IN ('pending', 'queued')
ORDER BY r.scheduled_at
LIMIT $2;
