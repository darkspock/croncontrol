-- name: CreateWorker :one
INSERT INTO workers (id, workspace_id, name, credential_hash, labels, max_concurrency)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetWorker :one
SELECT * FROM workers WHERE id = $1 AND workspace_id = $2;

-- name: ListWorkersByWorkspace :many
SELECT * FROM workers WHERE workspace_id = $1 ORDER BY name;

-- name: UpdateWorkerHeartbeat :exec
UPDATE workers SET
    last_heartbeat_at = now(),
    status = 'online',
    capabilities = COALESCE(sqlc.narg('capabilities'), capabilities),
    version = COALESCE(sqlc.narg('version'), version),
    consecutive_failures = 0,
    consecutive_healthy = consecutive_healthy + 1,
    updated_at = now()
WHERE id = $1;

-- name: SetWorkerStatus :exec
UPDATE workers SET status = $2, updated_at = now() WHERE id = $1;

-- name: IncrementWorkerFailures :exec
UPDATE workers SET
    consecutive_failures = consecutive_failures + 1,
    consecutive_healthy = 0,
    updated_at = now()
WHERE id = $1;

-- name: SetWorkerEnabled :exec
UPDATE workers SET enabled = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: UpdateWorkerLabels :exec
UPDATE workers SET labels = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: DeleteWorker :execrows
DELETE FROM workers WHERE id = $1 AND workspace_id = $2;

-- name: ListOnlineWorkers :many
SELECT w.* FROM workers w
WHERE w.workspace_id = $1
  AND w.enabled = true
  AND w.status = 'online'
ORDER BY (SELECT count(*) FROM runs r WHERE r.worker_id = w.id AND r.state = 'running')
LIMIT $2;

-- name: ListStaleWorkers :many
SELECT * FROM workers
WHERE enabled = true
  AND status = 'online'
  AND last_heartbeat_at < now() - interval '60 seconds';

-- name: CountRunningByWorker :one
SELECT count(*) FROM runs WHERE worker_id = $1 AND state = 'running';

-- name: SetWorkerEnrollmentToken :exec
UPDATE workers SET
    enrollment_token_hash = $2,
    enrollment_token_expires_at = $3,
    updated_at = now()
WHERE id = $1;

-- name: GetWorkerByEnrollmentToken :one
SELECT * FROM workers
WHERE enrollment_token_hash = $1
  AND enrollment_token_expires_at > now();

-- name: UpdateWorkerCredentialHash :exec
UPDATE workers SET
    credential_hash = $2,
    enrollment_token_hash = NULL,
    enrollment_token_expires_at = NULL,
    updated_at = now()
WHERE id = $1;
