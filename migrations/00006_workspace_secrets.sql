-- Workspace secrets: encrypted key-value vault.
-- Used by orchestras for injecting secrets into musicians.

CREATE TABLE IF NOT EXISTS workspace_secrets (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    value_enc       BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

CREATE INDEX IF NOT EXISTS idx_secrets_workspace ON workspace_secrets(workspace_id);
