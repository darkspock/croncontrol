-- Run artifacts: files attached to runs, stored in S3/MinIO or local filesystem.

CREATE TABLE IF NOT EXISTS run_artifacts (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    workspace_id    TEXT NOT NULL,
    name            VARCHAR(255) NOT NULL,
    content_type    VARCHAR(100),
    size_bytes      BIGINT,
    storage_key     TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(run_id, name)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_run ON run_artifacts(run_id);
