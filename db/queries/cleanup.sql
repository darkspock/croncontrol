-- name: DeleteOldRuns :execrows
DELETE FROM runs
WHERE id IN (
    SELECT r.id FROM runs r
    WHERE r.state IN ('completed', 'failed', 'hung', 'killed', 'skipped', 'cancelled')
      AND r.finished_at < $1
    LIMIT $2
);

-- name: DeleteOldJobs :execrows
DELETE FROM jobs
WHERE id IN (
    SELECT j.id FROM jobs j
    WHERE j.state IN ('completed', 'failed', 'killed', 'cancelled')
      AND j.updated_at < $1
    LIMIT $2
);

-- name: DeleteOldAudit :execrows
DELETE FROM audit_log
WHERE id IN (
    SELECT a.id FROM audit_log a
    WHERE a.created_at < $1
    LIMIT $2
);
