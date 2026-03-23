-- name: CreateJob :one
INSERT INTO jobs (
    id, workspace_id, queue_id, payload, priority,
    max_attempts, retry_backoff, scheduled_at, expires_at,
    reference, idempotency_key,
    runtime_override, worker_id_override, worker_labels_override,
    actor_type, actor_id, tags
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11,
    $12, $13, $14,
    $15, $16, $17
)
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs WHERE id = $1 AND workspace_id = $2;

-- name: GetJobWithQueue :one
SELECT j.*, q.name AS queue_name, q.execution_method, q.method_config AS queue_method_config,
       q.max_attempts AS queue_max_attempts, q.retry_backoff AS queue_retry_backoff,
       q.job_timeout, q.max_response_size, q.concurrency AS queue_concurrency
FROM jobs j
JOIN queues q ON q.id = j.queue_id
WHERE j.id = $1 AND j.workspace_id = $2;

-- name: ClaimPendingJobs :many
SELECT j.*, q.execution_method, q.method_config AS queue_method_config,
       q.max_attempts AS queue_max_attempts, q.retry_backoff AS queue_retry_backoff,
       q.job_timeout, q.max_response_size, q.concurrency AS queue_concurrency,
       q.runtime AS queue_runtime, q.worker_id AS queue_worker_id
FROM jobs j
JOIN queues q ON q.id = j.queue_id
JOIN workspaces w ON w.id = j.workspace_id
WHERE (
    (j.state = 'pending' AND j.scheduled_at <= now())
    OR (j.state = 'retrying' AND j.next_attempt_at <= now())
)
AND q.enabled = true
AND w.state = 'active'
ORDER BY j.priority DESC, j.scheduled_at, j.created_at
LIMIT $1
FOR UPDATE OF j SKIP LOCKED;

-- name: UpdateJobState :exec
UPDATE jobs SET
    state = $2,
    attempt = COALESCE(sqlc.narg('attempt'), attempt),
    next_attempt_at = sqlc.narg('next_attempt_at'),
    cancel_reason = sqlc.narg('cancel_reason'),
    waiting_reason = sqlc.narg('waiting_reason'),
    worker_id = sqlc.narg('worker_id'),
    duration_ms = sqlc.narg('duration_ms'),
    updated_at = now()
WHERE id = $1;

-- name: SnapshotJobConfig :exec
UPDATE jobs SET effective_config = $2, updated_at = now() WHERE id = $1;

-- name: SaveJobExecutionHandle :exec
UPDATE jobs SET
    execution_handle = $2,
    stdout_offset = 0,
    stderr_offset = 0,
    updated_at = now()
WHERE id = $1;

-- name: ClearJobExecutionHandle :exec
UPDATE jobs SET
    execution_handle = NULL,
    stdout_offset = 0,
    stderr_offset = 0,
    updated_at = now()
WHERE id = $1;

-- name: UpdateJobOffsets :exec
UPDATE jobs SET
    stdout_offset = $2,
    stderr_offset = $3,
    updated_at = now()
WHERE id = $1;

-- name: AppendJobAttemptResponseChunk :exec
UPDATE job_attempts
SET response_body = COALESCE(response_body, '') || $2
WHERE id = $1;

-- name: GetJobExecutionHandle :one
SELECT execution_handle, stdout_offset, stderr_offset
FROM jobs
WHERE id = $1 AND execution_handle IS NOT NULL;

-- name: ListActiveAsyncJobs :many
SELECT
    j.id,
    j.workspace_id,
    j.queue_id,
    j.attempt,
    j.state,
    COALESCE(ja.id, '') AS attempt_id,
    ja.started_at,
    j.execution_handle,
    j.stdout_offset,
    j.stderr_offset,
    q.execution_method,
    COALESCE(j.max_attempts, q.max_attempts) AS max_attempts,
    COALESCE(j.retry_backoff, q.retry_backoff) AS retry_backoff,
    q.max_response_size
FROM jobs j
JOIN queues q ON q.id = j.queue_id
LEFT JOIN LATERAL (
    SELECT id, started_at
    FROM job_attempts
    WHERE job_id = j.id AND finished_at IS NULL
    ORDER BY attempt_number DESC
    LIMIT 1
) ja ON true
WHERE j.state IN ('running', 'kill_requested')
  AND j.execution_handle IS NOT NULL;

-- name: ListKillRequestedAsyncJobIDs :many
SELECT id
FROM jobs
WHERE state = 'kill_requested'
  AND execution_handle IS NOT NULL;

-- name: ListJobs :many
SELECT j.*, q.name AS queue_name
FROM jobs j
JOIN queues q ON q.id = j.queue_id
WHERE j.workspace_id = $1
  AND ($2 = '' OR j.queue_id = $2)
  AND ($3 = '' OR j.state = $3)
  AND ($4 = '' OR j.reference = $4)
ORDER BY j.created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountJobs :one
SELECT count(*) FROM jobs
WHERE workspace_id = $1
  AND ($2 = '' OR queue_id = $2)
  AND ($3 = '' OR state = $3)
  AND ($4 = '' OR reference = $4);

-- name: CountRunningByQueue :one
SELECT count(*) FROM jobs WHERE queue_id = $1 AND state = 'running';

-- name: CheckIdempotencyKey :one
SELECT id FROM jobs WHERE workspace_id = $1 AND idempotency_key = $2 LIMIT 1;

-- name: ListExpiredPendingJobs :many
SELECT * FROM jobs
WHERE state = 'pending' AND expires_at IS NOT NULL AND expires_at <= now()
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: CancelJob :exec
UPDATE jobs SET state = 'cancelled', cancel_reason = $2, updated_at = now()
WHERE id = $1;

-- name: CreateReplayJob :one
INSERT INTO jobs (
    id, workspace_id, queue_id, payload, priority,
    max_attempts, retry_backoff, reference, replayed_from_job_id,
    actor_type, actor_id, tags
)
SELECT $2, j.workspace_id, j.queue_id,
    COALESCE(@override_payload::jsonb, j.payload),
    COALESCE(@override_priority::integer, j.priority),
    j.max_attempts, j.retry_backoff, j.reference, j.id,
    $3, $4, j.tags
FROM jobs j WHERE j.id = $1
RETURNING *;
