-- name: CreateArtifact :one
INSERT INTO run_artifacts (id, run_id, workspace_id, name, content_type, size_bytes, storage_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListArtifactsByRun :many
SELECT id, run_id, name, content_type, size_bytes, created_at
FROM run_artifacts WHERE run_id = $1 ORDER BY created_at;

-- name: GetArtifact :one
SELECT * FROM run_artifacts WHERE run_id = $1 AND name = $2;

-- name: DeleteArtifact :execrows
DELETE FROM run_artifacts WHERE run_id = $1 AND name = $2;
