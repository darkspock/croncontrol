-- name: CreateQueue :one
INSERT INTO queues (
    id, workspace_id, name, execution_method, runtime, method_config,
    concurrency, max_attempts, retry_backoff, job_timeout, max_response_size,
    ssh_credential_id, ssm_profile_id, k8s_cluster_id,
    worker_id, worker_labels, tags, enabled
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14,
    $15, $16, $17, $18
)
RETURNING *;

-- name: GetQueue :one
SELECT * FROM queues WHERE id = $1 AND workspace_id = $2;

-- name: ListQueues :many
SELECT * FROM queues WHERE workspace_id = $1 ORDER BY name;

-- name: CountQueues :one
SELECT count(*) FROM queues WHERE workspace_id = $1;

-- name: UpdateQueue :one
UPDATE queues SET
    name = $3, execution_method = $4, runtime = $5, method_config = $6,
    concurrency = $7, max_attempts = $8, retry_backoff = $9, job_timeout = $10, max_response_size = $11,
    ssh_credential_id = $12, ssm_profile_id = $13, k8s_cluster_id = $14,
    worker_id = $15, worker_labels = $16, tags = $17, enabled = $18,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteQueue :execrows
DELETE FROM queues WHERE id = $1 AND workspace_id = $2;

-- name: SetQueueEnabled :exec
UPDATE queues SET enabled = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: GetQueueStats :one
SELECT
    q.id,
    q.name,
    (SELECT count(*) FROM jobs j WHERE j.queue_id = q.id AND j.state = 'pending') AS pending_count,
    (SELECT count(*) FROM jobs j WHERE j.queue_id = q.id AND j.state = 'running') AS running_count,
    (SELECT count(*) FROM jobs j WHERE j.queue_id = q.id AND j.state = 'retrying') AS retrying_count,
    (SELECT count(*) FROM jobs j WHERE j.queue_id = q.id AND j.state = 'failed'
        AND j.updated_at >= now() - interval '24 hours') AS failed_24h,
    (SELECT count(*) FROM jobs j WHERE j.queue_id = q.id AND j.state = 'completed'
        AND j.updated_at >= now() - interval '24 hours') AS completed_24h
FROM queues q
WHERE q.id = $1 AND q.workspace_id = $2;
