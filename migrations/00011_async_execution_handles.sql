-- Durable async execution handles for runs and jobs.

ALTER TABLE runs
    ADD COLUMN IF NOT EXISTS execution_handle JSONB,
    ADD COLUMN IF NOT EXISTS stdout_offset BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS stderr_offset BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_runs_async_handle
    ON runs(id)
    WHERE execution_handle IS NOT NULL;

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS execution_handle JSONB,
    ADD COLUMN IF NOT EXISTS stdout_offset BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS stderr_offset BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_jobs_async_handle
    ON jobs(id)
    WHERE execution_handle IS NOT NULL;
