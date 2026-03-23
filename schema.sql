-- CronControl Canonical Database Schema
-- Source of truth for Atlas declarative migrations
-- Follows docs/product-specification.md
--
-- Public terms: workspace, user, process, run, queue, job, worker
-- Internal terms: tenant (maps to workspace), scheduled_slot (maps to run)
-- IDs: prefix + ULID stored as TEXT (e.g. wsp_01HYX..., prc_01HYX...)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- WORKSPACES, USERS, MEMBERSHIPS
-- ============================================================================

CREATE TABLE workspaces (
    id                  TEXT PRIMARY KEY,       -- wsp_ + ULID
    slug                VARCHAR(100) NOT NULL UNIQUE,
    name                VARCHAR(255) NOT NULL,
    state               VARCHAR(30) NOT NULL DEFAULT 'active'
                        CHECK (state IN ('active', 'suspended', 'archived')),
    default_timezone    VARCHAR(50) NOT NULL DEFAULT 'UTC',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id                      TEXT PRIMARY KEY,   -- usr_ + ULID
    email                   VARCHAR(255) NOT NULL UNIQUE,
    name                    VARCHAR(255) NOT NULL,
    auth_provider           VARCHAR(20) NOT NULL CHECK (auth_provider IN ('google', 'email')),
    password_hash           VARCHAR(255),        -- nullable, email auth only
    email_verified          BOOLEAN NOT NULL DEFAULT false,
    is_platform_admin       BOOLEAN NOT NULL DEFAULT false,
    active_workspace_id     TEXT REFERENCES workspaces(id) ON DELETE SET NULL,
    last_login_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspace_memberships (
    id              TEXT PRIMARY KEY,           -- wmb_ + ULID
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(20) NOT NULL DEFAULT 'viewer'
                    CHECK (role IN ('admin', 'operator', 'viewer')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, user_id)
);

CREATE INDEX idx_memberships_workspace ON workspace_memberships(workspace_id);
CREATE INDEX idx_memberships_user ON workspace_memberships(user_id);

-- ============================================================================
-- API KEYS
-- ============================================================================

CREATE TABLE api_keys (
    id              TEXT PRIMARY KEY,           -- key_ + ULID
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    key_hash        VARCHAR(255) NOT NULL UNIQUE,
    key_prefix      VARCHAR(10) NOT NULL,
    role            VARCHAR(20) NOT NULL DEFAULT 'operator'
                    CHECK (role IN ('admin', 'operator', 'viewer')),
    expires_at      TIMESTAMPTZ,                -- optional expiration
    created_by      TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_used_at    TIMESTAMPTZ,
    last_ip         VARCHAR(45),
    last_user_agent TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_workspace ON api_keys(workspace_id);

-- ============================================================================
-- WORKERS
-- ============================================================================

CREATE TABLE workers (
    id                  TEXT PRIMARY KEY,       -- wrk_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    credential_hash     VARCHAR(255) NOT NULL UNIQUE,
    labels              JSONB NOT NULL DEFAULT '{}',
    capabilities        JSONB NOT NULL DEFAULT '{}',
    max_concurrency     INTEGER NOT NULL DEFAULT 5,
    version             VARCHAR(50),
    enabled             BOOLEAN NOT NULL DEFAULT true,
    status              VARCHAR(20) NOT NULL DEFAULT 'offline'
                        CHECK (status IN ('online', 'offline', 'unhealthy')),
    last_heartbeat_at   TIMESTAMPTZ,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    consecutive_healthy  INTEGER NOT NULL DEFAULT 0,
    enrollment_token_hash VARCHAR(255),
    enrollment_token_expires_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX idx_workers_workspace ON workers(workspace_id);
CREATE INDEX idx_workers_status ON workers(workspace_id, status) WHERE enabled = true;
CREATE INDEX idx_workers_enrollment_token ON workers(enrollment_token_hash) WHERE enrollment_token_hash IS NOT NULL;

-- ============================================================================
-- REUSABLE CREDENTIALS / RESOURCES
-- ============================================================================

CREATE TABLE user_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL,
    token_type  VARCHAR(20) NOT NULL CHECK (token_type IN ('email_verify', 'password_reset')),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_tokens_hash ON user_tokens(token_hash) WHERE used_at IS NULL;
CREATE INDEX idx_user_tokens_user ON user_tokens(user_id, token_type);

-- ============================================================================
-- REUSABLE CREDENTIALS / RESOURCES (continued)
-- ============================================================================

CREATE TABLE ssh_credentials (
    id                  TEXT PRIMARY KEY,       -- ssh_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    private_key_enc     BYTEA NOT NULL,         -- AES-256-GCM encrypted
    fingerprint         VARCHAR(100) NOT NULL,
    username            VARCHAR(100),
    port                INTEGER DEFAULT 22,
    strict_host_key     BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE ssm_profiles (
    id                  TEXT PRIMARY KEY,       -- ssp_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    region              VARCHAR(50) NOT NULL,
    role_arn            VARCHAR(500),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE k8s_clusters (
    id                  TEXT PRIMARY KEY,       -- k8c_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    kubeconfig_enc      BYTEA NOT NULL,         -- AES-256-GCM encrypted
    default_namespace   VARCHAR(255) DEFAULT 'default',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

-- ============================================================================
-- PROCESSES
-- ============================================================================

CREATE TABLE processes (
    id                      TEXT PRIMARY KEY,   -- prc_ + ULID
    workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                    VARCHAR(255) NOT NULL,
    schedule_type           VARCHAR(20) NOT NULL
                            CHECK (schedule_type IN ('cron', 'fixed_delay', 'on_demand')),
    schedule                VARCHAR(100),        -- cron expression, nullable
    delay_duration          VARCHAR(20),          -- e.g. '5m', '1h', nullable
    timezone                VARCHAR(50),          -- inherits workspace default if null
    miss_policy             VARCHAR(20) DEFAULT 'skip'
                            CHECK (miss_policy IN ('execute', 'skip')),
    max_recovery_slots      INTEGER NOT NULL DEFAULT 10,
    allow_parallel          BOOLEAN NOT NULL DEFAULT false,
    max_parallel            INTEGER NOT NULL DEFAULT 1,
    on_overlap              VARCHAR(20) NOT NULL DEFAULT 'skip'
                            CHECK (on_overlap IN ('skip', 'queue')),
    execution_method        VARCHAR(20) NOT NULL
                            CHECK (execution_method IN ('http', 'ssh', 'ssm', 'k8s')),
    runtime                 VARCHAR(20) NOT NULL DEFAULT 'direct'
                            CHECK (runtime IN ('direct', 'worker')),
    method_config           JSONB NOT NULL DEFAULT '{}',
    -- retry model (canonical: runs support retry)
    max_attempts            INTEGER NOT NULL DEFAULT 1,
    retry_backoff           TEXT,                 -- comma-separated durations, nullable
    execution_timeout       INTERVAL NOT NULL DEFAULT '1 hour',
    heartbeat_timeout       INTERVAL,             -- nullable = disabled
    timeout_action          VARCHAR(20) NOT NULL DEFAULT 'both'
                            CHECK (timeout_action IN ('kill', 'alert', 'both')),
    -- references to reusable credentials
    ssh_credential_id       TEXT REFERENCES ssh_credentials(id) ON DELETE SET NULL,
    ssm_profile_id          TEXT REFERENCES ssm_profiles(id) ON DELETE SET NULL,
    k8s_cluster_id          TEXT REFERENCES k8s_clusters(id) ON DELETE SET NULL,
    -- worker routing
    worker_id               TEXT REFERENCES workers(id) ON DELETE SET NULL,
    worker_labels           JSONB,
    -- dependency
    depends_on_process_id   TEXT REFERENCES processes(id) ON DELETE SET NULL,
    dependency_type         VARCHAR(20) CHECK (dependency_type IN ('after', 'after_success')),
    -- metadata
    environment             JSONB DEFAULT '{}',
    tags                    TEXT[] DEFAULT '{}',
    enabled                 BOOLEAN NOT NULL DEFAULT true,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX idx_processes_workspace ON processes(workspace_id);
CREATE INDEX idx_processes_workspace_enabled ON processes(workspace_id) WHERE enabled = true;
CREATE INDEX idx_processes_depends_on ON processes(depends_on_process_id) WHERE depends_on_process_id IS NOT NULL;

-- ============================================================================
-- RUNS (public term for scheduled_slots)
-- ============================================================================

CREATE TABLE runs (
    id                      TEXT PRIMARY KEY,   -- run_ + ULID
    workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    process_id              TEXT NOT NULL REFERENCES processes(id) ON DELETE CASCADE,
    scheduled_at            TIMESTAMPTZ NOT NULL,
    state                   VARCHAR(20) NOT NULL DEFAULT 'pending'
                            CHECK (state IN (
                                'pending', 'waiting_for_worker', 'queued', 'running', 'retrying',
                                'kill_requested', 'completed', 'failed', 'hung',
                                'killed', 'skipped', 'cancelled', 'paused'
                            )),
    origin                  VARCHAR(20) NOT NULL
                            CHECK (origin IN ('cron', 'fixed_delay', 'manual', 'one_time', 'recovery', 'dependency', 'replay')),
    -- attempt tracking
    attempt                 INTEGER NOT NULL DEFAULT 0,
    max_attempts            INTEGER NOT NULL DEFAULT 1,
    next_attempt_at         TIMESTAMPTZ,
    -- timing
    started_at              TIMESTAMPTZ,
    finished_at             TIMESTAMPTZ,
    duration_ms             BIGINT,
    exit_code               INTEGER,
    -- progress (non-HTTP only)
    progress_total          INTEGER,
    progress_current        INTEGER,
    progress                INTEGER,             -- calculated %
    progress_message        TEXT,
    last_heartbeat_at       TIMESTAMPTZ,
    -- overlap / waiting
    queue_reason            TEXT,
    waiting_reason          TEXT,
    -- actor
    actor_type              VARCHAR(20) CHECK (actor_type IN ('user', 'api_key', 'worker', 'system')),
    actor_id                TEXT,
    killed_by_actor_type    VARCHAR(20),
    killed_by_actor_id      TEXT,
    -- lineage
    triggered_by_run_id     TEXT REFERENCES runs(id) ON DELETE SET NULL,
    replayed_from_run_id    TEXT REFERENCES runs(id) ON DELETE SET NULL,
    -- configuration snapshot
    effective_config        JSONB,               -- snapshotted at first attempt
    execution_handle        JSONB,               -- durable async execution handle
    stdout_offset           BIGINT NOT NULL DEFAULT 0,
    stderr_offset           BIGINT NOT NULL DEFAULT 0,
    -- runtime routing
    runtime                 VARCHAR(20) CHECK (runtime IN ('direct', 'worker')),
    worker_id               TEXT REFERENCES workers(id) ON DELETE SET NULL,
    -- metadata snapshot
    tags                    TEXT[] DEFAULT '{}',
    -- timestamps
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (process_id, scheduled_at, origin)
) WITH (fillfactor = 70);

-- Executor scan: find actionable runs
CREATE INDEX idx_runs_pending ON runs(workspace_id, scheduled_at)
    WHERE state IN ('pending', 'queued');
-- Retry scan
CREATE INDEX idx_runs_retrying ON runs(next_attempt_at)
    WHERE state = 'retrying';
-- Worker waiting scan
CREATE INDEX idx_runs_waiting_worker ON runs(workspace_id)
    WHERE state = 'waiting_for_worker';
-- Monitor: find running
CREATE INDEX idx_runs_running ON runs(workspace_id)
    WHERE state = 'running';
-- Kill requested
CREATE INDEX idx_runs_kill_requested ON runs(id)
    WHERE state = 'kill_requested';
CREATE INDEX idx_runs_async_handle ON runs(id)
    WHERE execution_handle IS NOT NULL;
-- Parallelism check
CREATE INDEX idx_runs_process_active ON runs(process_id, state);
-- History
CREATE INDEX idx_runs_process_history ON runs(process_id, scheduled_at DESC);
CREATE INDEX idx_runs_workspace ON runs(workspace_id, created_at DESC);

ALTER TABLE runs SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);

-- ============================================================================
-- RUN ATTEMPTS
-- ============================================================================

CREATE TABLE run_attempts (
    id              TEXT PRIMARY KEY,           -- rat_ + ULID
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    attempt_number  INTEGER NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ,
    duration_ms     BIGINT,
    exit_code       INTEGER,
    worker_id       TEXT REFERENCES workers(id) ON DELETE SET NULL,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_run_attempts_run ON run_attempts(run_id, attempt_number);

-- ============================================================================
-- RUN OUTPUT
-- ============================================================================

CREATE TABLE run_output (
    id          TEXT PRIMARY KEY,               -- no prefix, internal
    run_id      TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    stream      VARCHAR(10) NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    content     TEXT NOT NULL,
    truncated   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_run_output_run ON run_output(run_id, created_at);

-- ============================================================================
-- HEARTBEATS
-- ============================================================================

CREATE TABLE heartbeats (
    id          TEXT PRIMARY KEY,               -- no prefix, internal
    run_id      TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    total       INTEGER,
    current     INTEGER,
    progress    INTEGER,
    message     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_heartbeats_run ON heartbeats(run_id, created_at DESC);

-- ============================================================================
-- QUEUES
-- ============================================================================

CREATE TABLE queues (
    id                  TEXT PRIMARY KEY,       -- que_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    execution_method    VARCHAR(20) NOT NULL
                        CHECK (execution_method IN ('http', 'ssh', 'ssm', 'k8s')),
    runtime             VARCHAR(20) NOT NULL DEFAULT 'direct'
                        CHECK (runtime IN ('direct', 'worker')),
    method_config       JSONB NOT NULL DEFAULT '{}',
    concurrency         INTEGER NOT NULL DEFAULT 1,
    max_attempts        INTEGER NOT NULL DEFAULT 3,
    retry_backoff       TEXT NOT NULL DEFAULT '1m,5m,15m,1h',
    job_timeout         INTERVAL NOT NULL DEFAULT '5 minutes',
    max_response_size   INTEGER NOT NULL DEFAULT 5242880, -- 5MB
    -- references to reusable credentials
    ssh_credential_id   TEXT REFERENCES ssh_credentials(id) ON DELETE SET NULL,
    ssm_profile_id      TEXT REFERENCES ssm_profiles(id) ON DELETE SET NULL,
    k8s_cluster_id      TEXT REFERENCES k8s_clusters(id) ON DELETE SET NULL,
    -- worker routing defaults
    worker_id           TEXT REFERENCES workers(id) ON DELETE SET NULL,
    worker_labels       JSONB,
    -- metadata
    tags                TEXT[] DEFAULT '{}',
    enabled             BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX idx_queues_workspace ON queues(workspace_id);

-- ============================================================================
-- JOBS
-- ============================================================================

CREATE TABLE jobs (
    id                  TEXT PRIMARY KEY,       -- job_ + ULID
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    queue_id            TEXT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    -- runtime overrides (may override worker/labels, NOT execution_method)
    runtime_override    VARCHAR(20) CHECK (runtime_override IN ('direct', 'worker')),
    worker_id_override  TEXT REFERENCES workers(id) ON DELETE SET NULL,
    worker_labels_override JSONB,
    -- payload
    payload             JSONB NOT NULL DEFAULT '{}',
    priority            INTEGER NOT NULL DEFAULT 0,
    -- retry
    max_attempts        INTEGER,                 -- override queue default, nullable
    attempt             INTEGER NOT NULL DEFAULT 0,
    retry_backoff       TEXT,                     -- override queue default, nullable
    -- state
    state               VARCHAR(20) NOT NULL DEFAULT 'pending'
                        CHECK (state IN (
                            'pending', 'waiting_for_worker', 'running', 'retrying',
                            'kill_requested', 'completed', 'failed', 'killed', 'cancelled'
                        )),
    scheduled_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ,
    next_attempt_at     TIMESTAMPTZ,
    -- metadata
    reference           VARCHAR(500),
    idempotency_key     VARCHAR(255),
    replayed_from_job_id TEXT REFERENCES jobs(id) ON DELETE SET NULL,
    cancel_reason       TEXT,
    waiting_reason      TEXT,
    -- actor
    actor_type          VARCHAR(20) CHECK (actor_type IN ('user', 'api_key', 'worker', 'system')),
    actor_id            TEXT,
    -- snapshot
    effective_config    JSONB,                   -- snapshotted at first attempt
    execution_handle    JSONB,                   -- durable async execution handle
    stdout_offset       BIGINT NOT NULL DEFAULT 0,
    stderr_offset       BIGINT NOT NULL DEFAULT 0,
    tags                TEXT[] DEFAULT '{}',      -- inherited from queue
    -- routing
    worker_id           TEXT REFERENCES workers(id) ON DELETE SET NULL,
    -- timing
    duration_ms         BIGINT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
) WITH (fillfactor = 70);

-- Queue processor scan
CREATE INDEX idx_jobs_queue_pending ON jobs(queue_id, priority DESC, scheduled_at, created_at)
    WHERE state = 'pending';
CREATE INDEX idx_jobs_retrying ON jobs(next_attempt_at)
    WHERE state = 'retrying';
CREATE INDEX idx_jobs_waiting_worker ON jobs(workspace_id)
    WHERE state = 'waiting_for_worker';
CREATE INDEX idx_jobs_kill_requested ON jobs(id)
    WHERE state = 'kill_requested';
CREATE INDEX idx_jobs_async_handle ON jobs(id)
    WHERE execution_handle IS NOT NULL;
CREATE INDEX idx_jobs_expiring ON jobs(expires_at)
    WHERE state = 'pending' AND expires_at IS NOT NULL;
CREATE UNIQUE INDEX idx_jobs_idempotency ON jobs(workspace_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_jobs_reference ON jobs(workspace_id, reference)
    WHERE reference IS NOT NULL;
CREATE INDEX idx_jobs_replayed_from ON jobs(replayed_from_job_id)
    WHERE replayed_from_job_id IS NOT NULL;
CREATE INDEX idx_jobs_queue_running ON jobs(queue_id)
    WHERE state = 'running';
CREATE INDEX idx_jobs_workspace ON jobs(workspace_id, created_at DESC);

ALTER TABLE jobs SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);

-- ============================================================================
-- JOB ATTEMPTS
-- ============================================================================

CREATE TABLE job_attempts (
    id              TEXT PRIMARY KEY,           -- jat_ + ULID
    job_id          TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    attempt_number  INTEGER NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ,
    duration_ms     BIGINT,
    request         JSONB NOT NULL,
    response_code   INTEGER,
    response_headers JSONB,
    response_body   TEXT,
    truncated       BOOLEAN NOT NULL DEFAULT false,
    original_size   BIGINT,                     -- original size when truncated
    error_message   TEXT,
    worker_id       TEXT REFERENCES workers(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_attempts_job ON job_attempts(job_id, attempt_number);

-- ============================================================================
-- WEBHOOK SUBSCRIPTIONS
-- ============================================================================

CREATE TABLE webhook_subscriptions (
    id                      TEXT PRIMARY KEY,   -- whs_ + ULID
    workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    url                     TEXT NOT NULL,
    secret                  VARCHAR(255) NOT NULL, -- for HMAC-SHA256
    event_types             TEXT[] NOT NULL,      -- filter: e.g. {'run.*', 'job.failed'}
    enabled                 BOOLEAN NOT NULL DEFAULT true,
    consecutive_failures    INTEGER NOT NULL DEFAULT 0,
    auto_disabled           BOOLEAN NOT NULL DEFAULT false,
    last_delivery_at        TIMESTAMPTZ,
    last_failure_at         TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_subs_workspace ON webhook_subscriptions(workspace_id);

-- ============================================================================
-- AUDIT LOG
-- ============================================================================

CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,           -- no prefix, internal
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_type      VARCHAR(20) NOT NULL
                    CHECK (actor_type IN ('user', 'api_key', 'worker', 'system')),
    actor_id        TEXT,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       TEXT NOT NULL,
    action          VARCHAR(50) NOT NULL,
    details         JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity ON audit_log(workspace_id, entity_type, entity_id, created_at DESC);
CREATE INDEX idx_audit_workspace ON audit_log(workspace_id, created_at DESC);
