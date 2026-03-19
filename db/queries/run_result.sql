-- name: SetRunResult :exec
UPDATE runs SET result = $2, updated_at = now() WHERE id = $1;

-- name: GetRunResult :one
SELECT id, result FROM runs WHERE id = $1;
