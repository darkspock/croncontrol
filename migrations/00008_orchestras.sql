-- Orchestras: dynamic workflow orchestration.

CREATE TABLE IF NOT EXISTS orchestras (
    id                  TEXT PRIMARY KEY,
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    director_type       VARCHAR(20) NOT NULL DEFAULT 'none'
                        CHECK (director_type IN ('process', 'ai', 'none')),
    director_process_id TEXT,
    ai_config           JSONB,
    state               VARCHAR(30) NOT NULL DEFAULT 'active'
                        CHECK (state IN ('active', 'waiting_for_choice', 'paused', 'completed', 'cancelled', 'failed')),
    movement_count      INTEGER NOT NULL DEFAULT 0,
    secrets             TEXT[],
    summary             TEXT,
    budget              JSONB,
    budget_used         JSONB DEFAULT '{}',
    timeout             INTERVAL,
    timeout_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orchestras_workspace ON orchestras(workspace_id);
CREATE INDEX IF NOT EXISTS idx_orchestras_state ON orchestras(workspace_id, state);

-- Add waiting_for_choice to runs state check (need to drop and recreate)
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_state_check;
ALTER TABLE runs ADD CONSTRAINT runs_state_check CHECK (state IN (
    'pending', 'waiting_for_worker', 'queued', 'running', 'retrying',
    'kill_requested', 'completed', 'failed', 'hung', 'killed',
    'skipped', 'cancelled', 'paused', 'waiting_for_choice'
));
