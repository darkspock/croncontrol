-- name: CreateMembership :one
INSERT INTO workspace_memberships (id, workspace_id, user_id, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM workspace_memberships
WHERE workspace_id = $1 AND user_id = $2;

-- name: ListMembershipsByWorkspace :many
SELECT wm.*, u.email, u.name AS user_name
FROM workspace_memberships wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
ORDER BY wm.created_at;

-- name: ListMembershipsByUser :many
SELECT wm.*, w.name AS workspace_name, w.slug AS workspace_slug
FROM workspace_memberships wm
JOIN workspaces w ON w.id = wm.workspace_id
WHERE wm.user_id = $1
ORDER BY wm.created_at;

-- name: UpdateMembershipRole :one
UPDATE workspace_memberships SET role = $3, updated_at = now()
WHERE workspace_id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMembership :execrows
DELETE FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2;

-- name: CountAdmins :one
SELECT count(*) FROM workspace_memberships
WHERE workspace_id = $1 AND role = 'admin';
