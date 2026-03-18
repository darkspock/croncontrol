-- Platform admin: a global role that can access all workspaces.
-- This is separate from workspace-level admin role.

ALTER TABLE users ADD COLUMN is_platform_admin BOOLEAN NOT NULL DEFAULT false;

-- Platform admin API keys are not scoped to a workspace.
-- They use a special workspace_id = 'platform' sentinel.
