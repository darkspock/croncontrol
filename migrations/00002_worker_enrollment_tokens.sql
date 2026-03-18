-- Add enrollment token columns to workers table.
-- Enrollment tokens are temporary (1 hour) and used for the two-phase
-- worker registration flow: admin creates worker → gets token → worker
-- binary exchanges token for permanent credential.

ALTER TABLE workers
    ADD COLUMN enrollment_token_hash VARCHAR(255),
    ADD COLUMN enrollment_token_expires_at TIMESTAMPTZ;

CREATE INDEX idx_workers_enrollment_token ON workers(enrollment_token_hash)
    WHERE enrollment_token_hash IS NOT NULL;
