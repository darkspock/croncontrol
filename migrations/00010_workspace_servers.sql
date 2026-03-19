-- Workspace servers: auto-provisioned Hetzner servers per workspace.

CREATE TABLE IF NOT EXISTS workspace_servers (
    id                  TEXT PRIMARY KEY,
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    hetzner_id          BIGINT NOT NULL,
    name                VARCHAR(100) NOT NULL,
    ip_address          VARCHAR(45),
    state               VARCHAR(20) NOT NULL DEFAULT 'provisioning'
                        CHECK (state IN ('provisioning', 'ready', 'active', 'idle', 'destroying', 'destroyed')),
    server_type         VARCHAR(20) NOT NULL DEFAULT 'cx22',
    datacenter          VARCHAR(20) NOT NULL DEFAULT 'fsn1',
    containers_running  INTEGER NOT NULL DEFAULT 0,
    max_containers      INTEGER NOT NULL DEFAULT 4,
    last_activity_at    TIMESTAMPTZ,
    monthly_cost        NUMERIC(10,2) NOT NULL DEFAULT 4.50,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    destroyed_at        TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_servers_workspace ON workspace_servers(workspace_id, state);
