# CronControl

Open-source control plane for scheduled and event-driven operational workloads. Single Go binary + PostgreSQL. MIT licensed.

**Cron scheduling, durable queues, 5 execution methods, worker runtime, AI-powered orchestration, 19-page dashboard.**

[Website](https://croncontrol.dev) | [Live Demo](https://app.croncontrol.dev) | [Orchestras](https://croncontrol.dev/orchestras/) | [Compare](https://croncontrol.dev/compare/)

## Features

- **Scheduling**: Cron, fixed-delay, and on-demand with timezone/DST handling and missed-run recovery
- **Durable Queue**: Database-backed job queue with retry, exponential backoff, replay, batch enqueue, and idempotency keys
- **5 Execution Methods**: HTTP, SSH, AWS SSM, Kubernetes Jobs, Docker containers (via Swarm)
- **Worker Runtime**: Deploy workers inside private networks for air-gapped execution
- **Orchestras** (Coming Soon): Multi-step workflows with AI Director (Claude/GPT/Gemini), human-in-the-loop choices, real-time chat, and budget controls
- **Auto-Provisioned Infrastructure**: Hetzner servers created on demand for container execution, destroyed when idle
- **13-State Run Machine**: Validated transitions from pending to completed, with retrying, hung detection, kill, pause, and resume
- **Heartbeat & Progress**: Real-time progress tracking for long-running tasks
- **Workspace Secrets**: AES-256-GCM encrypted vault, injected as env vars into AgentNodes
- **Artifacts**: Upload/download files per run (S3/MinIO or local filesystem)
- **Webhook Events**: HMAC-SHA256 signed delivery for run.*, job.*, worker.* events
- **Prometheus Metrics**: `/metrics` endpoint with request latency, run duration, state gauges, 3 logging backends
- **MCP Server**: 15+ tools for AI agents (Claude, GPT, or any MCP client)
- **CLI**: `cronctl` with bash/zsh completion
- **5 SDKs**: PHP, Python, Node.js, Go, Laravel — all zero-dependency
- **Google OAuth + RBAC**: Admin, operator, viewer roles per workspace
- **Platform Admin**: Super admin cross-workspace management
- **19-Page Dashboard**: React 19 + shadcn/ui with dark theme
- **Single Binary**: Frontend embedded via `go:embed`. Deploy anywhere.

## Quick Start

```bash
# Prerequisites: Go 1.26+, Docker

git clone https://github.com/darkspock/croncontrol.git
cd croncontrol

# Start PostgreSQL
docker compose up -d

# Apply migrations and run
make migrate start

# Open the dashboard
open http://localhost:8090
```

### Seed Demo Data

```bash
./scripts/seed.sh
# Creates processes, runs, queues, jobs
# Login: demo@croncontrol.dev / demodemo1234
```

### Docker

```bash
docker build -t croncontrol .
docker run -p 8090:8090 -e CC_DATABASE_HOST=host.docker.internal croncontrol
```

## Dashboard

19 pages:

| Page | Description |
|---|---|
| Dashboard | Summary cards, recent runs, process status |
| Processes | List, create, delete, pause/resume, trigger |
| Process Detail | Stats, run history, configuration, dependencies |
| Runs | Filterable by state, origin, process |
| Run Detail | Progress bar, heartbeat timeline, output, kill/replay, error display |
| Upcoming Runs | Scheduled runs in the next 24h |
| Failed Jobs | Failed jobs across all queues |
| Timeline | Visual execution bars per process over time |
| Queues | Queue cards with stats |
| Queue Detail | Jobs, attempts, enqueue modal |
| Orchestras | Coming soon page |
| Settings: API Keys | Create, revoke, copy |
| Settings: Workers | Create with enrollment token, status monitoring |
| Settings: Secrets | Create, update, reveal/hide, delete (AES-256-GCM) |
| Settings: Members | Invite, role management |
| Settings: Webhooks | Create, test delivery, event filtering |
| Settings: Credentials | SSH keys, SSM profiles, K8s clusters |
| Settings: Infrastructure | Server list, provision, destroy, cost |
| Platform Admin | Cross-workspace stats, user management, infrastructure |

## Architecture

```
Control Plane (single Go binary)
├── Planner           — Materializes future runs from cron schedules
├── Executor          — Claims and dispatches runs (SELECT FOR UPDATE SKIP LOCKED)
├── Queue Processor   — Durable queue jobs with retry/backoff
├── Monitor           — Detects execution/heartbeat timeouts
├── Orchestra Monitor — Timeout and budget enforcement for orchestras
├── Notifier          — HMAC-signed webhook event delivery
├── Cleanup           — Retention-based data lifecycle
├── Metrics Collector — Prometheus gauges, counters, histograms
├── Worker Dispatcher — Routes tasks to private-network workers
├── Infra Provisioner — Auto-provisions/destroys Hetzner servers
└── AI Director       — Multi-model LLM orchestration (Claude/GPT/Gemini)

Execution: HTTP | SSH | SSM | K8s | Container (Docker Swarm)
Runtimes:  Direct | Worker (private networks) | Auto-provisioned
Database:  PostgreSQL 16 (25+ tables, prefix+ULID IDs)
Frontend:  React 19 + Vite + shadcn/ui + Tailwind CSS 4
Website:   Astro (croncontrol.dev)
```

## API

60+ endpoints. Authentication via `X-API-Key` header or Google OAuth. Full spec at `/api/v1/openapi.yaml`.

Key endpoints:

| Category | Endpoints |
|---|---|
| Auth | register, login, Google OAuth, forgot/reset password, verify email |
| Processes | CRUD, trigger, pause/resume, delete |
| Runs | list, detail, kill, cancel, replay, output, result, artifacts |
| Queues & Jobs | create queue, enqueue job, cancel, replay, batch |
| Workers | create (enrollment token), enroll, heartbeat, delete |
| Orchestras | create, score, finish, cancel, next movement, choose, chat (SSE) |
| Secrets | CRUD (AES-256-GCM encrypted) |
| Webhooks | CRUD, test delivery |
| Infrastructure | list servers, provision, destroy, pool overview, ready callback |
| Admin | platform stats, workspace management, user management, infra overview |

## SDKs

### Node.js
```javascript
const { CronControl } = require('./croncontrol');
const cc = new CronControl('http://localhost:8090', 'cc_live_...');
await cc.triggerProcess('prc_01HYX...');
await cc.createOrchestra({ name: 'cleanup', director_type: 'ai' });
```

### Python
```python
from croncontrol import CronControl
cc = CronControl('http://localhost:8090', 'cc_live_...')
cc.trigger_process('prc_01HYX...')
cc.create_orchestra(name='cleanup', director_type='ai')
```

### Go
```go
cc := croncontrol.New("http://localhost:8090", "cc_live_...")
cc.TriggerProcess(ctx, "prc_01HYX...")
cc.CreateOrchestra(ctx, croncontrol.CreateOrchestraParams{Name: "cleanup"})
```

### CLI
```bash
cronctl login --key cc_live_...
cronctl processes list
cronctl processes trigger prc_01HYX...
cronctl runs list --state failed
cronctl jobs enqueue --queue que_01HYX... --payload '{"key":"value"}'
```

### MCP Server
```bash
CRONCONTROL_URL=http://localhost:8090 CRONCONTROL_API_KEY=cc_live_... croncontrol-mcp
```
15+ tools for AI agents: list processes, trigger runs, enqueue jobs, manage orchestras, read scores, post to chat.

## Development

```bash
make setup      # Start PostgreSQL + apply migrations
make start      # Build frontend + run server
make test       # Run tests
make seed       # Seed demo data
make build      # Build all binaries (GoReleaser)
make deploy     # Deploy to production
```

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.26, chi router, SQLC, pgx/v5 |
| Database | PostgreSQL 16, incremental migrations |
| Frontend | React 19, Vite, shadcn/ui, Tailwind CSS 4, TanStack Query |
| Website | Astro, Tailwind CSS 4 |
| Auth | bcrypt, SHA-256 API keys, Google OAuth, RBAC |
| Encryption | AES-256-GCM (secrets), HMAC-SHA256 (webhooks) |
| Observability | Prometheus, 3 logging backends (database/file/OpenSearch) |
| Infrastructure | Hetzner Cloud API, Docker Swarm, cloud-init |
| AI | Anthropic, OpenAI, Google (direct API tool_use) |
| Tooling | Makefile, GoReleaser, GitHub Actions |

## Built with AI

CronControl was built in 2 days using [Claude Code](https://claude.com/claude-code) (Opus). 10 epics, 400+ tasks, 25,000+ lines of code. [Read the story](https://croncontrol.dev/built-with-ai/).

## License

MIT — see [LICENSE](LICENSE).
