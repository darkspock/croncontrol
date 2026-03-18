-- name: CreateAuditEntry :one
INSERT INTO audit_log (id, workspace_id, actor_type, actor_id, entity_type, entity_id, action, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListAuditByEntity :many
SELECT * FROM audit_log
WHERE workspace_id = $1 AND entity_type = $2 AND entity_id = $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: ListAuditByWorkspace :many
SELECT * FROM audit_log
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
