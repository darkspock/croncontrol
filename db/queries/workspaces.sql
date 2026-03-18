-- name: CreateWorkspace :one
INSERT INTO workspaces (id, slug, name, state)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces WHERE slug = $1;

-- name: UpdateWorkspace :one
UPDATE workspaces SET
    name = COALESCE(sqlc.narg('name'), name),
    default_timezone = COALESCE(sqlc.narg('default_timezone'), default_timezone),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateWorkspaceState :exec
UPDATE workspaces SET state = $2, updated_at = now() WHERE id = $1;

