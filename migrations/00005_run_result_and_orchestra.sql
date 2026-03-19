-- Run result: structured JSON output that the next run can read.
-- Orchestra fields: groups runs into orchestras (populated in Phase 2).

ALTER TABLE runs ADD COLUMN IF NOT EXISTS result JSONB;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS orchestra_id TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS orchestra_step INTEGER;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS choice_config JSONB;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS chosen_index INTEGER;

CREATE INDEX IF NOT EXISTS idx_runs_orchestra ON runs(orchestra_id) WHERE orchestra_id IS NOT NULL;
