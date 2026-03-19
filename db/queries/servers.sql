-- name: CreateServer :one
INSERT INTO workspace_servers (id, workspace_id, hetzner_id, name, ip_address, state, server_type, datacenter, max_containers, monthly_cost)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetServer :one
SELECT * FROM workspace_servers WHERE id = $1;

-- name: ListServersByWorkspace :many
SELECT * FROM workspace_servers WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: ListActiveServersByWorkspace :many
SELECT * FROM workspace_servers WHERE workspace_id = $1 AND state NOT IN ('destroyed') ORDER BY created_at DESC;

-- name: UpdateServerState :exec
UPDATE workspace_servers SET state = $2, updated_at = now() WHERE id = $1;

-- name: MarkServerReady :exec
UPDATE workspace_servers SET state = 'ready', ip_address = COALESCE(sqlc.narg('ip_address'), ip_address), updated_at = now() WHERE id = $1;

-- name: MarkServerActive :exec
UPDATE workspace_servers SET state = 'active', last_activity_at = now(), updated_at = now() WHERE id = $1;

-- name: IncrementContainers :exec
UPDATE workspace_servers SET
    containers_running = containers_running + 1,
    state = 'active',
    last_activity_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: DecrementContainers :exec
UPDATE workspace_servers SET
    containers_running = GREATEST(containers_running - 1, 0),
    last_activity_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: ListIdleServers :many
SELECT * FROM workspace_servers
WHERE state IN ('ready', 'active')
  AND containers_running = 0
  AND last_activity_at < now() - sqlc.arg('grace_period')::interval
ORDER BY last_activity_at ASC;

-- name: MarkServerDestroyed :exec
UPDATE workspace_servers SET state = 'destroyed', destroyed_at = now(), updated_at = now() WHERE id = $1;

-- name: FindServerWithCapacity :one
SELECT * FROM workspace_servers
WHERE workspace_id = $1
  AND state IN ('ready', 'active')
  AND containers_running < max_containers
ORDER BY containers_running ASC
LIMIT 1;

-- name: CountServersByWorkspace :one
SELECT count(*) FROM workspace_servers WHERE workspace_id = $1 AND state NOT IN ('destroyed', 'destroying');

-- name: DestroyServersByWorkspace :exec
UPDATE workspace_servers SET state = 'destroying', updated_at = now()
WHERE workspace_id = $1 AND state NOT IN ('destroyed', 'destroying');
