// Admin queries for platform-level operations.
// source: admin.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const listAllWorkspaces = `-- name: ListAllWorkspaces :many
SELECT id, name, slug, state, default_timezone, created_at, updated_at FROM workspaces ORDER BY created_at DESC LIMIT $1 OFFSET $2
`

type ListAllWorkspacesParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (q *Queries) ListAllWorkspaces(ctx context.Context, arg ListAllWorkspacesParams) ([]Workspace, error) {
	rows, err := q.db.Query(ctx, listAllWorkspaces, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Workspace{}
	for rows.Next() {
		var i Workspace
		if err := rows.Scan(&i.ID, &i.Name, &i.Slug, &i.State, &i.DefaultTimezone, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const listAllUsers = `-- name: ListAllUsers :many
SELECT id, email, name, auth_provider, email_verified, is_platform_admin, active_workspace_id, last_login_at, created_at
FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
`

type ListAllUsersParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

type ListAllUsersRow struct {
	ID                string             `json:"id"`
	Email             string             `json:"email"`
	Name              string             `json:"name"`
	AuthProvider      string             `json:"auth_provider"`
	EmailVerified     bool               `json:"email_verified"`
	IsPlatformAdmin   bool               `json:"is_platform_admin"`
	ActiveWorkspaceID *string            `json:"active_workspace_id"`
	LastLoginAt       pgtype.Timestamptz `json:"last_login_at"`
	CreatedAt         pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) ListAllUsers(ctx context.Context, arg ListAllUsersParams) ([]ListAllUsersRow, error) {
	rows, err := q.db.Query(ctx, listAllUsers, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListAllUsersRow{}
	for rows.Next() {
		var i ListAllUsersRow
		if err := rows.Scan(&i.ID, &i.Email, &i.Name, &i.AuthProvider, &i.EmailVerified, &i.IsPlatformAdmin, &i.ActiveWorkspaceID, &i.LastLoginAt, &i.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const setPlatformAdmin = `-- name: SetPlatformAdmin :exec
UPDATE users SET is_platform_admin = $2, updated_at = now() WHERE id = $1
`

type SetPlatformAdminParams struct {
	ID              string `json:"id"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
}

func (q *Queries) SetPlatformAdmin(ctx context.Context, arg SetPlatformAdminParams) error {
	_, err := q.db.Exec(ctx, setPlatformAdmin, arg.ID, arg.IsPlatformAdmin)
	return err
}

// UpdateWorkspaceState is already defined in workspaces.sql.go — reused here.

const getPlatformStats = `-- name: GetPlatformStats :one
SELECT
    (SELECT count(*) FROM workspaces) AS total_workspaces,
    (SELECT count(*) FROM workspaces WHERE state = 'active') AS active_workspaces,
    (SELECT count(*) FROM users) AS total_users,
    (SELECT count(*) FROM users WHERE is_platform_admin = true) AS platform_admins,
    (SELECT count(*) FROM processes) AS total_processes,
    (SELECT count(*) FROM runs WHERE state = 'running') AS running_runs,
    (SELECT count(*) FROM runs WHERE state = 'failed' AND created_at > now() - interval '24 hours') AS failed_runs_24h,
    (SELECT count(*) FROM jobs WHERE state = 'running') AS running_jobs,
    (SELECT count(*) FROM workers WHERE status = 'online') AS online_workers
`

type PlatformStats struct {
	TotalWorkspaces  int64 `json:"total_workspaces"`
	ActiveWorkspaces int64 `json:"active_workspaces"`
	TotalUsers       int64 `json:"total_users"`
	PlatformAdmins   int64 `json:"platform_admins"`
	TotalProcesses   int64 `json:"total_processes"`
	RunningRuns      int64 `json:"running_runs"`
	FailedRuns24h    int64 `json:"failed_runs_24h"`
	RunningJobs      int64 `json:"running_jobs"`
	OnlineWorkers    int64 `json:"online_workers"`
}

func (q *Queries) GetPlatformStats(ctx context.Context) (PlatformStats, error) {
	row := q.db.QueryRow(ctx, getPlatformStats)
	var s PlatformStats
	err := row.Scan(&s.TotalWorkspaces, &s.ActiveWorkspaces, &s.TotalUsers, &s.PlatformAdmins, &s.TotalProcesses, &s.RunningRuns, &s.FailedRuns24h, &s.RunningJobs, &s.OnlineWorkers)
	return s, err
}
