-- name: AppendRunOutput :one
INSERT INTO run_output (id, run_id, stream, content, truncated)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRunOutput :many
SELECT * FROM run_output
WHERE run_id = $1
  AND ($2::text IS NULL OR stream = $2)
ORDER BY created_at;
