-- name: CreateJobAttempt :one
INSERT INTO job_attempts (
    id, job_id, attempt_number, started_at, request, worker_id
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: FinishJobAttempt :exec
UPDATE job_attempts SET
    finished_at = $2,
    duration_ms = $3,
    response_code = $4,
    response_headers = sqlc.narg('response_headers'),
    response_body = sqlc.narg('response_body'),
    truncated = $5,
    original_size = sqlc.narg('original_size'),
    error_message = sqlc.narg('error_message')
WHERE id = $1;

-- name: ListJobAttemptsByJob :many
SELECT * FROM job_attempts WHERE job_id = $1 ORDER BY attempt_number;
