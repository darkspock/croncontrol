// Code manually added to match sqlc pattern.
// source: user_tokens.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// UserToken represents a row in the user_tokens table.
type UserToken struct {
	ID        string             `json:"id"`
	UserID    string             `json:"user_id"`
	TokenHash string             `json:"token_hash"`
	TokenType string             `json:"token_type"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
	UsedAt    pgtype.Timestamptz `json:"used_at"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

const createUserToken = `-- name: CreateUserToken :one
INSERT INTO user_tokens (id, user_id, token_hash, token_type, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, token_hash, token_type, expires_at, used_at, created_at
`

type CreateUserTokenParams struct {
	ID        string             `json:"id"`
	UserID    string             `json:"user_id"`
	TokenHash string             `json:"token_hash"`
	TokenType string             `json:"token_type"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

func (q *Queries) CreateUserToken(ctx context.Context, arg CreateUserTokenParams) (UserToken, error) {
	row := q.db.QueryRow(ctx, createUserToken, arg.ID, arg.UserID, arg.TokenHash, arg.TokenType, arg.ExpiresAt)
	var t UserToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.TokenType, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	return t, err
}

const invalidatePriorTokens = `-- name: InvalidatePriorTokens :exec
UPDATE user_tokens SET used_at = now()
WHERE user_id = $1 AND token_type = $2 AND used_at IS NULL
`

type InvalidatePriorTokensParams struct {
	UserID    string `json:"user_id"`
	TokenType string `json:"token_type"`
}

func (q *Queries) InvalidatePriorTokens(ctx context.Context, arg InvalidatePriorTokensParams) error {
	_, err := q.db.Exec(ctx, invalidatePriorTokens, arg.UserID, arg.TokenType)
	return err
}

const getValidToken = `-- name: GetValidToken :one
SELECT id, user_id, token_hash, token_type, expires_at, used_at, created_at FROM user_tokens
WHERE token_hash = $1 AND token_type = $2 AND used_at IS NULL AND expires_at > now()
`

type GetValidTokenParams struct {
	TokenHash string `json:"token_hash"`
	TokenType string `json:"token_type"`
}

func (q *Queries) GetValidToken(ctx context.Context, arg GetValidTokenParams) (UserToken, error) {
	row := q.db.QueryRow(ctx, getValidToken, arg.TokenHash, arg.TokenType)
	var t UserToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.TokenType, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	return t, err
}

const markTokenUsed = `-- name: MarkTokenUsed :exec
UPDATE user_tokens SET used_at = now() WHERE id = $1
`

func (q *Queries) MarkTokenUsed(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, markTokenUsed, id)
	return err
}
