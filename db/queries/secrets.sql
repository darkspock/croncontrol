-- name: CreateSecret :one
INSERT INTO workspace_secrets (id, workspace_id, name, value_enc)
VALUES ($1, $2, $3, $4)
RETURNING id, workspace_id, name, created_at, updated_at;

-- name: ListSecretsByWorkspace :many
SELECT id, workspace_id, name, created_at, updated_at
FROM workspace_secrets WHERE workspace_id = $1 ORDER BY name;

-- name: GetSecret :one
SELECT * FROM workspace_secrets WHERE workspace_id = $1 AND name = $2;

-- name: UpdateSecret :exec
UPDATE workspace_secrets SET value_enc = $3, updated_at = now()
WHERE workspace_id = $1 AND name = $2;

-- name: DeleteSecret :execrows
DELETE FROM workspace_secrets WHERE workspace_id = $1 AND name = $2;
