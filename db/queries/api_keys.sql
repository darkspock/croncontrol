-- name: CreateAPIKey :one
INSERT INTO api_keys (id, workspace_id, name, key_hash, key_prefix, role, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT ak.*, w.state AS workspace_state
FROM api_keys ak
JOIN workspaces w ON w.id = ak.workspace_id
WHERE ak.key_hash = $1 AND ak.enabled = true;

-- name: ListAPIKeysByWorkspace :many
SELECT * FROM api_keys WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: DeleteAPIKey :execrows
DELETE FROM api_keys WHERE id = $1 AND workspace_id = $2;

-- name: UpdateAPIKeyUsage :exec
UPDATE api_keys SET last_used_at = now(), last_ip = $2, last_user_agent = $3 WHERE id = $1;
