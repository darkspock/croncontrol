// Run artifacts queries.
// source: artifacts.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type RunArtifact struct {
	ID          string             `json:"id"`
	RunID       string             `json:"run_id"`
	WorkspaceID string             `json:"workspace_id"`
	Name        string             `json:"name"`
	ContentType *string            `json:"content_type"`
	SizeBytes   *int64             `json:"size_bytes"`
	StorageKey  string             `json:"-"` // internal
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

const createArtifact = `-- name: CreateArtifact :one
INSERT INTO run_artifacts (id, run_id, workspace_id, name, content_type, size_bytes, storage_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, run_id, workspace_id, name, content_type, size_bytes, storage_key, created_at
`

type CreateArtifactParams struct {
	ID          string  `json:"id"`
	RunID       string  `json:"run_id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	ContentType *string `json:"content_type"`
	SizeBytes   *int64  `json:"size_bytes"`
	StorageKey  string  `json:"-"`
}

func (q *Queries) CreateArtifact(ctx context.Context, arg CreateArtifactParams) (RunArtifact, error) {
	row := q.db.QueryRow(ctx, createArtifact, arg.ID, arg.RunID, arg.WorkspaceID, arg.Name, arg.ContentType, arg.SizeBytes, arg.StorageKey)
	var a RunArtifact
	err := row.Scan(&a.ID, &a.RunID, &a.WorkspaceID, &a.Name, &a.ContentType, &a.SizeBytes, &a.StorageKey, &a.CreatedAt)
	return a, err
}

const listArtifactsByRun = `-- name: ListArtifactsByRun :many
SELECT id, run_id, name, content_type, size_bytes, created_at
FROM run_artifacts WHERE run_id = $1 ORDER BY created_at
`

type ArtifactListItem struct {
	ID          string             `json:"id"`
	RunID       string             `json:"run_id"`
	Name        string             `json:"name"`
	ContentType *string            `json:"content_type"`
	SizeBytes   *int64             `json:"size_bytes"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) ListArtifactsByRun(ctx context.Context, runID string) ([]ArtifactListItem, error) {
	rows, err := q.db.Query(ctx, listArtifactsByRun, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ArtifactListItem{}
	for rows.Next() {
		var a ArtifactListItem
		if err := rows.Scan(&a.ID, &a.RunID, &a.Name, &a.ContentType, &a.SizeBytes, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

const getArtifact = `-- name: GetArtifact :one
SELECT id, run_id, workspace_id, name, content_type, size_bytes, storage_key, created_at
FROM run_artifacts WHERE run_id = $1 AND name = $2
`

func (q *Queries) GetArtifact(ctx context.Context, runID, name string) (RunArtifact, error) {
	row := q.db.QueryRow(ctx, getArtifact, runID, name)
	var a RunArtifact
	err := row.Scan(&a.ID, &a.RunID, &a.WorkspaceID, &a.Name, &a.ContentType, &a.SizeBytes, &a.StorageKey, &a.CreatedAt)
	return a, err
}

const deleteArtifact = `-- name: DeleteArtifact :execrows
DELETE FROM run_artifacts WHERE run_id = $1 AND name = $2
`

func (q *Queries) DeleteArtifact(ctx context.Context, runID, name string) (int64, error) {
	result, err := q.db.Exec(ctx, deleteArtifact, runID, name)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
