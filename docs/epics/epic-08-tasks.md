# EPIC-08 Tasks: Agentic API, MCP, CLI, and SDKs

> Status: DONE (95%) — updated 2026-03-18

## T08.1 OpenAPI Contract Polish
- [x] Complete `api/openapi.yaml` with ALL canonical resources:
  - Workspaces, users, memberships.
  - Workers (CRUD, enroll, heartbeat).
  - Processes (CRUD, trigger, pause/resume, replay).
  - Runs (list, get, cancel, kill, output, replay).
  - Queues (CRUD, pause/resume).
  - Jobs (enqueue, list, get, cancel, replay).
  - Webhook subscriptions (CRUD).
  - API keys (CRUD).
  - Credentials: ssh_credentials, ssm_profiles, k8s_clusters (CRUD each).
  - Heartbeat.
  - Health.
  - Registration + auth.
- [x] All schemas use canonical terms: workspace, run, fixed_delay, on_demand.
- [x] All IDs are prefixed strings in examples: `wsp_01HYX...`.
- [x] Response envelope: `{ data, meta }` for lists, `{ data }` for single, `{ error }` for errors.
- [x] Error payload: code, message, hint, details (optional).
- [x] Validation errors: field-level structured details.
- [x] Publish spec at `/api/v1/openapi.yaml` (auto-served by backend).
- [x] 46KB+, 1875 lines, comprehensive endpoint coverage.

## T08.2 Agent-Friendly Error Model
- [x] Every error has: `code` (machine-readable), `message` (human), `hint` (what to do next).
- [x] Examples:
  - `PLAN_LIMIT_EXCEEDED`: hint about upgrading
  - `WORKSPACE_SUSPENDED`: hint about workspace state
  - `EMAIL_NOT_VERIFIED`: hint about verification
  - `IDEMPOTENCY_CONFLICT`: includes `existing_job_id`
- [x] Consistent across all endpoints.

## T08.3 MCP Server
- [x] `mcp/server.go` — Go binary with JSON-RPC 2.0 via stdin/stdout.
- [x] Configuration: CronControl API URL + API key (env vars).
- [x] Tools exposed (15+):
  | Tool | Maps to |
  |---|---|
  | `list_processes` | GET /processes |
  | `create_process` | POST /processes |
  | `get_process` | GET /processes/{id} |
  | `update_process` | PUT /processes/{id} |
  | `delete_process` | DELETE /processes/{id} |
  | `trigger_process` | POST /processes/{id}/trigger |
  | `pause_process` | POST /processes/{id}/pause |
  | `resume_process` | POST /processes/{id}/resume |
  | `list_runs` | GET /runs |
  | `get_run` | GET /runs/{id} |
  | `cancel_run` | POST /runs/{id}/cancel |
  | `kill_run` | POST /runs/{id}/kill |
  | `list_jobs` | GET /jobs |
  | `enqueue_job` | POST /jobs |
  | `get_job` | GET /jobs/{id} |
- [x] Each tool has JSON schema for agent input validation.
- [x] Error responses include hint for agent to suggest next action.
- [x] Compatible with: Claude Desktop, Claude Code, any MCP-compatible client.
- [x] `cmd/croncontrol-mcp/main.go` — binary entry point.
- [x] `mcp/client.go` — HTTP client for CronControl API.
- [x] Tests: tool invocations with mock API.

## T08.4 CLI Tool (cronctl)
- [x] `cmd/cronctl/main.go` — Go binary.
- [x] Auth:
  - `cronctl login --key cc_live_...` — non-interactive.
  - `cronctl login --key ... --url ...` — custom server.
  - Config stored in `~/.cronctl/config.json`.
- [x] Resource commands:
  - `cronctl processes list`
  - `cronctl processes trigger <id>`
  - `cronctl processes pause <id>`
  - `cronctl processes resume <id>`
  - `cronctl runs list [--process <id>] [--state failed]`
  - `cronctl runs get <id>`
  - `cronctl runs kill <id>`
  - `cronctl queues list`
  - `cronctl queues create --name "..." --method http --url "..."`
  - `cronctl jobs enqueue --queue <id> --payload '{"..."}'`
  - `cronctl jobs list [--queue <id>] [--state failed]`
  - `cronctl jobs get <id>`
  - `cronctl jobs cancel <id>`
  - `cronctl workers list`
  - `cronctl workers enroll --name "..." --url "..."`
  - `cronctl health`
  - `cronctl version`
  - `cronctl help`
- [x] Output formats: JSON output supported.
- [x] Tab completion: `cronctl completion bash` and `cronctl completion zsh` generate shell completion scripts. Commands + subcommands.
- [x] Tests: command parsing, output formatting.

## T08.5 PHP Heartbeat SDK
- [x] `php/CronControl.php`:
  - Single file, zero dependencies.
  - Auto-detect `CRONCONTROL_RUN_ID` and `CRONCONTROL_API_URL` from environment.
  - `heartbeat(int $total, int $current, string $message = '')`.
  - Uses `file_get_contents` with stream context.
  - Errors logged to stderr, never thrown.
- [x] `php/example.php` — usage example.

## T08.6 Lightweight SDKs
- [x] **Python SDK** (`sdk/python/croncontrol.py`): zero-dependency (stdlib only), full API coverage, `CronControlError` with code/hint, env var config.
- [x] **Node.js SDK** (`sdk/node/croncontrol.js`): zero-dependency (native fetch, Node 18+), full API coverage, `CronControlError`, CommonJS module with package.json.
- [x] **Go SDK** (`sdk/go/croncontrol.go`): zero-dependency (stdlib only), full API coverage, `Error` type with status/code/hint, `ListResponse`/`SingleResponse` types.
- [x] **Laravel SDK** (`sdk/laravel/`): Guzzle-based, service provider + facade + config, full API coverage, `CronControlException`, auto-discovery via `composer.json` extra.
- [x] All SDKs: same terminology, same error model, same response types, same env var convention (`CRONCONTROL_URL`, `CRONCONTROL_API_KEY`).

## T08.7 Documentation and Examples
- [x] API reference: OpenAPI spec served at `/api/v1/openapi.yaml`.
- [x] Getting started guide: README.md quick start + seed script.
- [x] Worker setup guide: `docs/guides/worker-setup.md` — download, enroll, systemd service, configure process with worker runtime.
- [x] MCP setup guide: `docs/guides/mcp-setup.md` — download, Claude Desktop/Code config, available tools, example prompts.
- [x] CLI quickstart: `docs/guides/cli-quickstart.md` — install, login, processes/runs/queues/jobs/workers commands, output formats.
- [x] Webhook integration guide: `docs/guides/webhook-integration.md` — create subscription, receive events, verify signature (4 languages), event types, delivery guarantees.

## Acceptance Checklist
- [x] API exposes same capabilities as dashboard for core flows.
- [x] Error responses structured and predictable for humans and agents.
- [x] MCP tools cover most important operational workflows.
- [x] CLI useful for real operator tasks, not only demos.
- [x] SDKs thin and aligned with canonical API contract (Python, Node.js, Go, Laravel).
