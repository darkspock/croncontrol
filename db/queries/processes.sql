-- name: CreateProcess :one
INSERT INTO processes (
    id, workspace_id, name, schedule_type, schedule, delay_duration, timezone,
    miss_policy, max_recovery_slots, allow_parallel, max_parallel, on_overlap,
    execution_method, runtime, method_config,
    max_attempts, retry_backoff, execution_timeout, heartbeat_timeout, timeout_action,
    ssh_credential_id, ssm_profile_id, k8s_cluster_id,
    worker_id, worker_labels,
    depends_on_process_id, dependency_type,
    environment, tags, enabled
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14, $15,
    $16, $17, $18, $19, $20,
    $21, $22, $23,
    $24, $25,
    $26, $27,
    $28, $29, $30
)
RETURNING *;

-- name: GetProcess :one
SELECT * FROM processes WHERE id = $1 AND workspace_id = $2;

-- name: GetProcessByName :one
SELECT * FROM processes WHERE name = $1 AND workspace_id = $2;

-- name: ListProcesses :many
SELECT * FROM processes
WHERE workspace_id = $1
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: CountProcesses :one
SELECT count(*) FROM processes WHERE workspace_id = $1;

-- name: UpdateProcess :one
UPDATE processes SET
    name = $3, schedule_type = $4, schedule = $5, delay_duration = $6, timezone = $7,
    miss_policy = $8, max_recovery_slots = $9, allow_parallel = $10, max_parallel = $11, on_overlap = $12,
    execution_method = $13, runtime = $14, method_config = $15,
    max_attempts = $16, retry_backoff = $17, execution_timeout = $18, heartbeat_timeout = $19, timeout_action = $20,
    ssh_credential_id = $21, ssm_profile_id = $22, k8s_cluster_id = $23,
    worker_id = $24, worker_labels = $25,
    depends_on_process_id = $26, dependency_type = $27,
    environment = $28, tags = $29, enabled = $30,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteProcess :execrows
DELETE FROM processes WHERE id = $1 AND workspace_id = $2;

-- name: SetProcessEnabled :exec
UPDATE processes SET enabled = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: ListEnabledCronProcesses :many
SELECT p.*, w.state AS workspace_state
FROM processes p
JOIN workspaces w ON w.id = p.workspace_id
WHERE p.enabled = true AND p.schedule_type = 'cron'
  AND w.state = 'active';

-- name: ListEnabledFixedDelayProcesses :many
SELECT p.*, w.state AS workspace_state
FROM processes p
JOIN workspaces w ON w.id = p.workspace_id
WHERE p.enabled = true AND p.schedule_type = 'fixed_delay'
  AND w.state = 'active';

-- name: GetDependentProcesses :many
SELECT * FROM processes
WHERE depends_on_process_id = $1 AND enabled = true;
