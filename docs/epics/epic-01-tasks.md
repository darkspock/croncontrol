# EPIC-01 Tasks: Canonical Platform Foundation

> Status: DONE (100%) — audited 2026-03-18

## T01.1 Documentation Migration
- [x] Confirm `docs/` as canonical root. Add `docs/README.md` with rules.
- [x] Mark root-level legacy docs with deprecation notice.
- [x] Ensure all new documents go into `docs/` only.

## T01.2 Core Glossary Enforcement
- [x] Create `docs/glossary.md` with public vs internal terms:
  - Public: workspace, user, process, run, queue, job, worker, webhook subscription
  - Internal only: tenant, scheduled_slot
- [x] Audit all code, schema, config, and comments for terminology violations.
- [x] Schema uses public terms for table names (workspaces, runs, etc.).
- [x] API uses public terms in endpoints and response bodies.

## T01.3 ID Convention (Prefix + ULID)
- [x] Create `internal/id/id.go`:
  - `New(prefix string) string` — generates `prefix + ULID`
  - Constants for all 15 prefixes: `wsp_`, `usr_`, `wmb_`, `wrk_`, `prc_`, `run_`, `rat_`, `que_`, `job_`, `jat_`, `whs_`, `key_`, `ssh_`, `ssp_`, `k8c_`
  - `Parse(id string) (prefix, ulid string, err error)` — validates format
  - Individual generators per resource (e.g., `NewWorkspace()`, `NewProcess()`)
- [x] All database IDs are TEXT with prefix+ULID.
- [x] Unit tests for ID generation and parsing.

## T01.4 Response Envelope and Error Model
- [x] Define Go types for API responses:
  - Success list: `{ "data": [...], "meta": { "page", "per_page", "total" } }`
  - Success single: `{ "data": { ... } }`
  - Error: `{ "error": { "code", "message", "hint", "details" } }`
- [x] Define error codes as constants: `VALIDATION_ERROR`, `NOT_FOUND`, `CONFLICT`, `IDEMPOTENCY_CONFLICT`, `PLAN_LIMIT_EXCEEDED`, `RATE_LIMITED`, `WORKSPACE_SUSPENDED`, `EMAIL_NOT_VERIFIED`, etc.
- [x] Validation errors use field-level structured `details`.
- [x] Plan exhaustion returns HTTP 429 (not 402) with `PLAN_LIMIT_EXCEEDED`.
- [x] Encode in OpenAPI spec.

## T01.5 Secret Classification and Redaction
- [x] Define secret categories: SSH keys, kubeconfig, webhook secrets, API key raw values, passwords.
- [x] All secrets encrypted at rest (AES-256-GCM).
- [x] `internal/crypto/crypto.go`: encrypt/decrypt with 32-byte key validation.
- [x] Redaction rules: secrets never appear in API responses, logs, audit details, or historical attempt snapshots.
- [x] API responses for credentials return metadata only (fingerprint, name, not raw keys).

## T01.6 OpenAPI Conventions
- [x] Versioning: `/api/v1/...`
- [x] Resource endpoints use plural nouns: `/processes`, `/runs`, `/queues`, `/jobs`, `/workers`
- [x] Workspace scoping is implicit (derived from API key or session).
- [x] Public API uses `runs`, not `slots`.
- [x] Standard pagination: `page`, `per_page` query params, `meta` block in response.
- [x] Write `api/openapi.yaml` (1875 lines, full endpoint coverage).

## T01.7 SQL Schema Conventions
- [x] All tables have `created_at` and `updated_at` (where applicable).
- [x] IDs are `TEXT PRIMARY KEY` with prefix+ULID.
- [x] Foreign keys use `ON DELETE CASCADE` or `ON DELETE SET NULL` as appropriate.
- [x] Check constraints on all enum-like columns.
- [x] Fillfactor 70% on high-update tables (runs, jobs).
- [x] Autovacuum tuning on high-update tables.
- [x] Schema verified — 19 tables in schema.sql (503 lines).

## T01.8 Configuration Surface
- [x] `config/config.go` with Viper: server, database, auth, scheduling, concurrency, retention, logging.
- [x] `config.yaml` for local dev defaults.
- [x] `.env.example` for production env vars.
- [x] Environment variable overrides follow `CC_SECTION_KEY` pattern.
- [x] No `CC_MODE` — open source, deploy anywhere.
- [x] Encryption key validation (32-byte requirement for AES-256).

## T01.9 Local Development Setup
- [x] Docker Compose: PostgreSQL (port 5435), optional OpenSearch, pgAdmin.
- [x] `Justfile` with setup, dev, test, generate, migrate commands.
- [x] `mise.toml` for tool versions.
- [x] Pre-commit hooks.
- [x] `just setup` gets a developer from zero to running.

## Acceptance Checklist
- [x] `docs/` is canonical root with no conflicts.
- [x] Public execution model uses worker runtime, no `local execution` concept anywhere.
- [x] API responses follow one consistent envelope.
- [x] Core cross-cutting contradictions resolved and documented.
- [x] Security guidance covers all secret types.
- [x] Data model and API use the same resource names.
