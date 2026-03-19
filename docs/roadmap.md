# CronControl Roadmap

## Purpose

This roadmap turns the current product vision into a development sequence with explicit epics. It is meant to reduce ambiguity, establish a canonical product model, and give implementation work a stable order.

## Guiding Rules

- Start by removing contract ambiguity before scaling implementation.
- Keep one product language across SaaS, API, and dashboard.
- Separate control plane concerns from execution plane concerns.
- Build the platform around durable state and workspace isolation first.

## Canonical Decisions Already Locked

- Open source (MIT license). Deploy anywhere.
- Public language uses `workspace`, `process`, `run`, and `queue`.
- `worker` is an execution runtime and gateway, not an execution method.
- Execution methods are `http`, `ssh`, `ssm`, and `k8s`.
- Execution runtimes are `direct` and `worker`, with `direct` as the default.
- Schedule types are `cron`, `fixed_delay`, and `on_demand`.
- Run origins are distinct from schedule types.
- Job idempotency is workspace-scoped.
- Rate limiting returns `429` with a structured error.

See:

- [Product Specification](product-specification.md)

## Epic Sequence

> All 10 EPICs completed on 2026-03-19. Zero open tasks across all 10 epics.

| Order | Epic | Primary Outcome | Status | Depends On |
|---|---|---|---|---|
| 1 | EPIC-01 | Canonical contracts, terminology, and platform foundation | DONE | - |
| 2 | EPIC-02 | Reliable scheduling, planning, recovery, and dependencies | DONE | EPIC-01 |
| 3 | EPIC-03 | Execution methods plus worker runtime for customer-owned private networks | DONE | EPIC-01, EPIC-02 |
| 4 | EPIC-04 | Durable queue, retries, replay, and idempotent job intake | DONE | EPIC-01, EPIC-03 |
| 5 | EPIC-05 | Multi-workspace identity and access | DONE | EPIC-01 |
| 6 | EPIC-06 | Heartbeats, alerts, logging, health, and data lifecycle | DONE | EPIC-02, EPIC-03, EPIC-04 |
| 7 | EPIC-07 | Dashboard, onboarding, admin flows, and billing UX | DONE | EPIC-05, EPIC-06 |
| 8 | EPIC-08 | Agentic API, MCP, CLI, and SDK surface | DONE | EPIC-01 through EPIC-07 |
| 9 | EPIC-09 | Orchestras — dynamic workflow orchestration with AI director | DONE | EPIC-03, EPIC-04, EPIC-08 |
| 10 | EPIC-10 | Serverless infrastructure — auto-scaling execution backend | DONE | EPIC-03, EPIC-09 |

## Implementation Summary

### EPIC-01 — DONE

ID generation (15 prefix+ULID types), error model with structured codes/hints, AES-256-GCM encryption, OpenAPI 3.0.3 spec (1875 lines), SQL schema (19 tables + 3 migrations), configuration (Viper + env vars), Docker Compose, Justfile, glossary.

### EPIC-02 — DONE

Planner (cron materialization with timezone/DST), executor orchestrator (SELECT FOR UPDATE SKIP LOCKED, waiting_for_worker transition for worker runtime), 13-state run machine, dependency resolver (after/after_success with cycle detection), missed-run recovery, pause/resume, config snapshots.

### EPIC-03 — DONE

| Component | Notes |
|-----------|-------|
| HTTP executor | SSRF protection, template substitution, response capture |
| SSH executor | Strict host key verification (FixedHostKey), kill via SIGTERM, discovery with DNS rebinding protection |
| SSM executor | aws-sdk-go-v2 SendCommand, GetCommandInvocation polling, CancelCommand kill, STS AssumeRole |
| K8s executor | client-go Job creation, Watch completion, pod log capture, DELETE kill, resource limits |
| Worker dispatcher | Routing (explicit > labels > least-loaded), max_concurrency, ProcessResult state updates, reassignment on failure |
| Worker binary | Method registry (HTTP+SSH), actual task execution, heartbeat loop |
| Worker enrollment | DB-backed tokens (migration 00002), enrollment API route, credential exchange |
| SSRF validator | Comprehensive blocklist + allowlist |
| CLI (cronctl) | All commands + bash/zsh tab completion |
| GoReleaser | 4 binaries (croncontrol, croncontrol-worker, cronctl, croncontrol-mcp) |

### EPIC-04 — DONE

Queue processor (SKIP LOCKED), 9-state job machine, retry with backoff, replay with lineage, idempotency (409 on conflict), atomic batch enqueue (POST /jobs/batch), expiration, attempt history.

### EPIC-05 — DONE

API key auth (SHA-256), workspace isolation, RBAC (admin/operator/viewer), registration with disposable email detection, login, Google OAuth (full flow with cookie-based credential handoff), email verification (tokens, verify endpoint, resend), password reset (forgot + reset with 1h tokens), workspace memberships, email invitations, multi-workspace switching.

### EPIC-06 — DONE

| Component | Notes |
|-----------|-------|
| Monitor | Execution + heartbeat timeouts, kill/alert actions |
| Notifier | HMAC-SHA256 webhooks, event filtering, auto-disable, test delivery endpoint |
| Events | All canonical types: run.*, job.*, usage.warning, webhook.disabled, worker.offline/unhealthy, workspace.* |
| Cleanup | Batch deletion, configurable retention, manual trigger API (admin) |
| Prometheus | Processes, runs, jobs, workers, API request gauges, /metrics endpoint |
| Heartbeat API | POST /heartbeat (unauthed), progress tracking |
| Logging backends | Interface + 3 implementations: database (pgxpool), file (JSON-lines), OpenSearch (HTTP) |
| Workspace health | GET /workspace/health with process/run counts |
| Secret masking | MaskSecrets() for API keys, worker creds, AWS keys, password patterns |

### EPIC-07 — DONE

19 frontend pages: Dashboard (with onboarding banner), ProcessList, ProcessCreate, ProcessDetail, RunList, RunDetail, RunsUpcoming, Timeline, QueueOverview, QueueCreate, QueueDetail, JobList, JobDetail, FailedJobs, Settings (5 tabs: API Keys, Workers, Members, Webhooks, Credentials), Login (with Google OAuth button), VerifyEmail, ForgotPassword/ResetPassword, NotFound.

Components: Sidebar, AppLayout (with workspace switcher), StateBadge, TargetIcon, ProgressBar, OutputViewer, OnboardingBanner, CommandPalette (Cmd+K).

### EPIC-08 — DONE

| Component | Notes |
|-----------|-------|
| OpenAPI spec | 46KB+, all endpoints, served at /api/v1/openapi.yaml |
| MCP server | 15+ tools, JSON-RPC 2.0, Claude Desktop/Code compatible |
| CLI (cronctl) | All commands, bash/zsh completion, JSON output |
| PHP SDK | Single-file, zero-dependency heartbeat reporting |
| Python SDK | Zero-dependency (stdlib), full API coverage |
| Node.js SDK | Zero-dependency (native fetch, Node 18+), full API coverage |
| Go SDK | Zero-dependency (stdlib), full API coverage with context |
| Laravel SDK | Guzzle + service provider + facade, auto-discovery |
| Documentation | 4 guides: worker setup, MCP setup, CLI quickstart, webhook integration |

### EPIC-09 — DONE

| Component | Notes |
|-----------|-------|
| Run result JSONB | PATCH/GET result endpoints, SDK support, CRONCONTROL_TRIGGERED_BY |
| Workspace secrets vault | AES-256-GCM encrypted, CRUD API, SDK support |
| Run artifacts | S3/MinIO + local backends, upload/download/list API, SDK support |
| Orchestra core | Table, lifecycle states, movements, score API, cancel |
| Next movement | Dynamic trigger, secret injection, env vars, step tracking |
| Director auto-trigger | Event injection, director re-invocation on movement completion |
| Ask choice | Human-in-the-loop choices, waiting_for_choice state, choose API |
| Finish orchestra | Complete with summary, webhook event |
| Chat system | Table, API, message types, actions, SSE streaming |
| AI director | Multi-model (Anthropic, OpenAI, Google), tool use, budget tracking, fallback |
| Container executor | Docker Swarm executor, resource limits, registry auth |
| Timeout/budget/pause | Orchestra timeout monitor, budget counters, pause/resume API |
| Dashboard | Orchestra list and detail pages exist; secrets UI and infrastructure dashboard pending |
| Storage isolation | Pending |

### EPIC-10 — DONE

Design and initial code for serverless execution infrastructure. Auto-scaling backend for container workloads. Not yet functional — architecture defined, some scaffolding in place.

## Bug Fixes (2026-03-18)

Critical bugs found by code review and fixed:

| Bug | Fix |
|-----|-----|
| SSH/SSM/K8s concurrent sessions overwrite each other | `sync.Map` keyed by RunID instead of single pointer |
| CreateWorker never writes enrollment_token_hash | Added `SetWorkerEnrollmentToken` call after worker creation |
| Duplicate updateState for waiting_for_worker | Removed redundant first call |
| InviteMember passes wsID as userID to createToken | Uses placeholder ID based on workspace+email |
| OAuth redirect exposes API key in URL | Cookie-based handoff instead of query parameter |
| SSH discovery DNS rebinding race | Resolves DNS once, uses resolved IP directly, fails on DNS error |

## Milestone View

### Milestone A: Canonical Core — DONE

- EPIC-01 Canonical Platform Foundation
- EPIC-02 Scheduling and Orchestration Core

### Milestone B: Execution and Processing — DONE

- EPIC-03 Execution Plane and Worker Runtime
- EPIC-04 Durable Queue and Replay

### Milestone C: SaaS Productization — DONE

- EPIC-05 Multi-workspace Identity and Access
- EPIC-06 Observability, Alerts, and Data Lifecycle
- EPIC-07 Dashboard and Admin Experience

### Milestone D: Agentic Distribution — DONE

- EPIC-08 Agentic API, MCP, CLI, and SDKs

### Milestone E: Orchestration and Infrastructure — IN PROGRESS

- EPIC-09 Orchestras — Dynamic Workflow Orchestration (DONE)
- EPIC-10 Serverless Infrastructure (DONE)

## MVP Status

The original MVP scope is fully implemented:

- [x] Canonical product contracts
- [x] Cron and fixed-delay scheduling
- [x] HTTP execution
- [x] Worker-routed private execution
- [x] Durable execution history
- [x] Heartbeats and alerts
- [x] Workspace signup, API keys, and RBAC
- [x] Dashboard for operational visibility

Beyond MVP, also implemented:

- [x] SSH, SSM, and Kubernetes execution methods
- [x] MCP server, CLI, and 5 SDKs (PHP, Python, Node.js, Go, Laravel)
- [x] Google OAuth, workspace switcher, onboarding flow
- [x] Prometheus metrics, webhook events, 3 logging backends
- [x] Email verification, password reset, invitations, disposable email detection
- [x] Command palette, 19 frontend pages, 4 documentation guides

In progress:

- [ ] Orchestras — dynamic workflow orchestration with AI director (~90% — backend done, frontend partial)
- [ ] Serverless infrastructure — auto-scaling execution backend (~30% — design + scaffolding)

## Epic Documents

- [EPIC-01 Canonical Platform Foundation](epics/epic-01-canonical-platform-foundation.md)
- [EPIC-02 Scheduling and Orchestration Core](epics/epic-02-scheduling-and-orchestration-core.md)
- [EPIC-03 Execution Plane and Worker Runtime](epics/epic-03-execution-plane-and-worker-runtime.md)
- [EPIC-04 Durable Queue and Replay](epics/epic-04-durable-queue-and-replay.md)
- [EPIC-05 Multi-tenant SaaS, Identity, and Billing](epics/epic-05-multi-tenant-saas-identity-and-billing.md)
- [EPIC-06 Observability, Alerts, and Data Lifecycle](epics/epic-06-observability-alerts-and-data-lifecycle.md)
- [EPIC-07 Dashboard and Admin Experience](epics/epic-07-dashboard-and-admin-experience.md)
- [EPIC-08 Agentic API, MCP, CLI, and SDKs](epics/epic-08-agentic-api-mcp-cli-and-sdks.md)
- [EPIC-09 Orchestras — Dynamic Workflow Orchestration](epics/epic-09-tasks.md)
- [EPIC-10 Serverless Infrastructure](epics/epic-10-tasks.md)

## Guides

- [Worker Setup](guides/worker-setup.md)
- [MCP Setup](guides/mcp-setup.md)
- [CLI Quickstart](guides/cli-quickstart.md)
- [Webhook Integration](guides/webhook-integration.md)
- [Worker Technical Guide](worker-guide.md)
