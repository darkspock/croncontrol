-- name: CreateOrchestra :one
INSERT INTO orchestras (id, workspace_id, name, director_type, director_process_id, ai_config, secrets, budget, timeout, timeout_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetOrchestra :one
SELECT * FROM orchestras WHERE id = $1 AND workspace_id = $2;

-- name: ListOrchestrasByWorkspace :many
SELECT * FROM orchestras WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateOrchestraState :exec
UPDATE orchestras SET state = $2, updated_at = now() WHERE id = $1;

-- name: FinishOrchestra :exec
UPDATE orchestras SET state = 'completed', summary = $2, updated_at = now() WHERE id = $1;

-- name: IncrementMovementCount :exec
UPDATE orchestras SET movement_count = movement_count + 1, updated_at = now() WHERE id = $1;

-- name: UpdateOrchestraBudgetUsed :exec
UPDATE orchestras SET budget_used = $2, updated_at = now() WHERE id = $1;

-- name: ListMovementsByOrchestra :many
SELECT * FROM runs WHERE orchestra_id = $1 ORDER BY orchestra_step;
