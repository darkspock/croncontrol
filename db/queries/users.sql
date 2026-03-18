-- name: CreateUser :one
INSERT INTO users (id, email, name, auth_provider, password_hash, email_verified, active_workspace_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateLastLogin :exec
UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1;

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true, updated_at = now() WHERE id = $1;

-- name: SetActiveWorkspace :exec
UPDATE users SET active_workspace_id = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;
