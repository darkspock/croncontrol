// Workspace secrets queries.
// source: secrets.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type WorkspaceSecret struct {
	ID          string             `json:"id"`
	WorkspaceID string             `json:"workspace_id"`
	Name        string             `json:"name"`
	ValueEnc    []byte             `json:"-"` // never serialize
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
}

// SecretListItem is returned by list — no value.
type SecretListItem struct {
	ID          string             `json:"id"`
	WorkspaceID string             `json:"workspace_id"`
	Name        string             `json:"name"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
}

const createSecret = `-- name: CreateSecret :one
INSERT INTO workspace_secrets (id, workspace_id, name, value_enc)
VALUES ($1, $2, $3, $4)
RETURNING id, workspace_id, name, created_at, updated_at
`

type CreateSecretParams struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	ValueEnc    []byte `json:"-"`
}

func (q *Queries) CreateSecret(ctx context.Context, arg CreateSecretParams) (SecretListItem, error) {
	row := q.db.QueryRow(ctx, createSecret, arg.ID, arg.WorkspaceID, arg.Name, arg.ValueEnc)
	var s SecretListItem
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Name, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

const listSecretsByWorkspace = `-- name: ListSecretsByWorkspace :many
SELECT id, workspace_id, name, created_at, updated_at
FROM workspace_secrets WHERE workspace_id = $1 ORDER BY name
`

func (q *Queries) ListSecretsByWorkspace(ctx context.Context, workspaceID string) ([]SecretListItem, error) {
	rows, err := q.db.Query(ctx, listSecretsByWorkspace, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecretListItem{}
	for rows.Next() {
		var s SecretListItem
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.Name, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

const getSecret = `-- name: GetSecret :one
SELECT id, workspace_id, name, value_enc, created_at, updated_at
FROM workspace_secrets WHERE workspace_id = $1 AND name = $2
`

type GetSecretParams struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

func (q *Queries) GetSecret(ctx context.Context, arg GetSecretParams) (WorkspaceSecret, error) {
	row := q.db.QueryRow(ctx, getSecret, arg.WorkspaceID, arg.Name)
	var s WorkspaceSecret
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Name, &s.ValueEnc, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

const updateSecret = `-- name: UpdateSecret :exec
UPDATE workspace_secrets SET value_enc = $3, updated_at = now()
WHERE workspace_id = $1 AND name = $2
`

func (q *Queries) UpdateSecret(ctx context.Context, workspaceID, name string, valueEnc []byte) error {
	_, err := q.db.Exec(ctx, updateSecret, workspaceID, name, valueEnc)
	return err
}

const deleteSecret = `-- name: DeleteSecret :execrows
DELETE FROM workspace_secrets WHERE workspace_id = $1 AND name = $2
`

type DeleteSecretParams struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

func (q *Queries) DeleteSecret(ctx context.Context, arg DeleteSecretParams) (int64, error) {
	result, err := q.db.Exec(ctx, deleteSecret, arg.WorkspaceID, arg.Name)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
