-- name: CreateRunAttempt :one
INSERT INTO run_attempts (id, run_id, attempt_number, started_at, worker_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: FinishRunAttempt :exec
UPDATE run_attempts SET
    finished_at = $2,
    duration_ms = $3,
    exit_code = $4,
    error_message = sqlc.narg('error_message')
WHERE id = $1;

-- name: ListRunAttemptsByRun :many
SELECT * FROM run_attempts WHERE run_id = $1 ORDER BY attempt_number;
