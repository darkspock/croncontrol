-- name: CreateChatMessage :one
INSERT INTO orchestra_chat (id, orchestra_id, sender_type, sender_id, message_type, content, data)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListChatMessages :many
SELECT * FROM orchestra_chat
WHERE orchestra_id = $1 AND created_at > $2
ORDER BY created_at
LIMIT $3;

-- name: ListChatMessagesAll :many
SELECT * FROM orchestra_chat
WHERE orchestra_id = $1
ORDER BY created_at
LIMIT $2 OFFSET $3;
