-- name: CreateUserToken :one
INSERT INTO user_tokens (id, user_id, token_hash, token_type, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: InvalidatePriorTokens :exec
UPDATE user_tokens SET used_at = now()
WHERE user_id = $1 AND token_type = $2 AND used_at IS NULL;

-- name: GetValidToken :one
SELECT * FROM user_tokens
WHERE token_hash = $1 AND token_type = $2 AND used_at IS NULL AND expires_at > now();

-- name: MarkTokenUsed :exec
UPDATE user_tokens SET used_at = now() WHERE id = $1;
