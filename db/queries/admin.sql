-- name: ListAllWorkspaces :many
SELECT * FROM workspaces ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: ListAllUsers :many
SELECT id, email, name, auth_provider, email_verified, is_platform_admin, active_workspace_id, last_login_at, created_at
FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: SetPlatformAdmin :exec
UPDATE users SET is_platform_admin = $2, updated_at = now() WHERE id = $1;

-- name: GetPlatformStats :one
SELECT
    (SELECT count(*) FROM workspaces) AS total_workspaces,
    (SELECT count(*) FROM workspaces WHERE state = 'active') AS active_workspaces,
    (SELECT count(*) FROM users) AS total_users,
    (SELECT count(*) FROM users WHERE is_platform_admin = true) AS platform_admins,
    (SELECT count(*) FROM processes) AS total_processes,
    (SELECT count(*) FROM runs WHERE state = 'running') AS running_runs,
    (SELECT count(*) FROM runs WHERE state = 'failed' AND created_at > now() - interval '24 hours') AS failed_runs_24h,
    (SELECT count(*) FROM jobs WHERE state = 'running') AS running_jobs,
    (SELECT count(*) FROM workers WHERE status = 'online') AS online_workers;
