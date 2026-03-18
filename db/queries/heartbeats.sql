-- name: CreateHeartbeat :one
INSERT INTO heartbeats (id, run_id, total, current, progress, message)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListHeartbeatsByRun :many
SELECT * FROM heartbeats WHERE run_id = $1 ORDER BY created_at;
