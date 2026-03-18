# CronControl

Open-source control plane for scheduled and event-driven operational workloads. Manage cron jobs, durable queues, and infrastructure execution with full traceability, heartbeats, and replay.

## Features

- **Scheduling**: Cron, fixed-delay, and on-demand execution with missed-run recovery
- **Durable Queue**: Database-backed job queue with retry, replay, and full attempt history
- **Execution Methods**: HTTP, SSH, AWS SSM, Kubernetes Jobs
- **Worker Runtime**: Deploy workers in your infrastructure for private network execution
- **Heartbeat & Progress**: Real-time progress tracking (total/current/%) for long-running processes
- **Dependencies**: Trigger processes after upstream completion (after / after_success)
- **13-State Run Machine**: pending → running → completed/failed/hung/killed with retry support
- **Multi-Workspace**: Isolated workspaces with role-based access (admin/operator/viewer)
- **Webhook Events**: HMAC-SHA256 signed event delivery with auto-disable
- **Prometheus Metrics**: `/metrics` endpoint with request latency, run duration, and state gauges
- **Agentic-First**: MCP server for AI agents, CLI tool (`cronctl`), PHP heartbeat SDK
- **Dashboard**: 15-page React UI with dark/light mode, timeline visualization, and real-time polling
- **Single Binary**: Frontend embedded in Go binary via `go:embed`

## Quick Start

```bash
# Prerequisites: Go 1.24+, Docker

# Clone
git clone https://github.com/darkspock/croncontrol.git
cd croncontrol

# Start PostgreSQL and apply schema
docker compose up -d
docker compose exec -T postgres psql -U croncontrol -d croncontrol < schema.sql

# Run the server
go run .

# Open the dashboard
open http://localhost:8080

# Or use the API directly
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","name":"My Workspace","password":"mysecurepass12"}'
```

### Seed Demo Data

```bash
# Start the server, then in another terminal:
./scripts/seed.sh
# Creates 5 processes, runs, a queue, and sample jobs
# Login: demo@croncontrol.dev / demodemo1234
```

### Docker

```bash
docker build -t croncontrol .
docker run -p 8080:8080 -e CC_DATABASE_HOST=host.docker.internal croncontrol
```

## Dashboard

15 pages with dark/light mode:

| Page | Features |
|---|---|
| Dashboard | Summary cards, recent runs, process status |
| Processes | List, create, delete, pause/resume, trigger |
| Process Detail | Stats, runs history, full configuration, dependency info |
| Runs | Filterable list by state/origin |
| Run Detail | Progress bar, heartbeat timeline, output viewer, kill/replay |
| Timeline | Visual execution bars per process over time |
| Queues | Cards with stats, enqueue job modal |
| Queue Create | Method, concurrency, retry config |
| Jobs | Filterable list, state badges |
| Job Detail | Collapsible attempt history with request/response |
| Settings | API keys (create/revoke), workers, members |

## Architecture

```
Control Plane (Go)
├── Planner        — Materializes future runs from cron schedules
├── Executor       — Claims and dispatches runs (SELECT FOR UPDATE SKIP LOCKED)
├── Queue Proc.    — Processes durable queue jobs with retry/backoff
├── Monitor        — Detects execution/heartbeat timeouts
├── Notifier       — Delivers HMAC-signed webhook events
├── Cleanup        — Retention-based data lifecycle
├── Metrics        — Prometheus /metrics endpoint
└── Worker Disp.   — Routes tasks to customer-deployed workers

Execution Methods: HTTP | SSH | SSM | K8s
Runtimes: Direct | Worker (private networks)
Database: PostgreSQL 16 (18 tables, prefix+ULID IDs)
Frontend: React 19 + Vite + shadcn/ui + Tailwind
```

## API

31 endpoints. Authentication via `X-API-Key` header. Full spec at `/api/v1/openapi.yaml`.

| Method | Endpoint | Description |
|---|---|---|
| POST | /api/v1/register | Create workspace + get API key |
| POST | /api/v1/login | Email + password login |
| GET | /api/v1/processes | List processes |
| POST | /api/v1/processes | Create a process |
| GET | /api/v1/processes/:id | Get process details |
| POST | /api/v1/processes/:id/trigger | Trigger a manual run |
| POST | /api/v1/processes/:id/pause | Pause scheduling |
| GET | /api/v1/runs | List runs (filter by state/origin/process) |
| GET | /api/v1/runs/:id | Get run with attempts |
| POST | /api/v1/runs/:id/kill | Kill running execution |
| GET | /api/v1/runs/:id/output | Get stdout/stderr |
| POST | /api/v1/heartbeat | Report progress (no auth) |
| GET | /api/v1/queues | List queues |
| POST | /api/v1/queues | Create a queue |
| POST | /api/v1/jobs | Enqueue a job |
| GET | /api/v1/jobs/:id | Get job + attempt history |
| POST | /api/v1/jobs/:id/replay | Replay a failed job |
| GET | /api/v1/api-keys | List API keys |
| POST | /api/v1/api-keys | Create API key |
| GET | /api/v1/workers | List workers |
| POST | /api/v1/workers | Create worker + enrollment token |
| GET | /health | Health check |
| GET | /metrics | Prometheus metrics |

## Agentic

### MCP Server
```bash
CRONCONTROL_URL=http://localhost:8080 CRONCONTROL_API_KEY=cc_live_... croncontrol-mcp
```
12 tools: list_processes, create_process, trigger_process, list_runs, get_run, kill_run, enqueue_job, get_job, replay_job, get_health, and more.

### CLI
```bash
cronctl login --key cc_live_...
cronctl processes list
cronctl processes trigger prc_01HYX...
cronctl runs list --state failed
cronctl jobs enqueue --queue que_01HYX... --payload '{"to":"user@example.com"}'
```

### PHP SDK
```php
$cc = new CronControl();
$cc->heartbeat(1000, 0, "Starting...");
// ... work ...
$cc->heartbeat(1000, 500, "Halfway");
$cc->heartbeat(1000, 1000, "Done");
```

## Development

```bash
just setup          # Start PostgreSQL + apply schema
just dev            # Run with hot reload (air)
just build          # Build single binary (frontend + backend)
just test           # Run all 44 tests
just seed           # Seed demo data
just docker-build   # Build Docker image
just generate       # Regenerate API + DB code
just lint           # Run linters
```

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.24, chi, SQLC, pgx/v5 |
| Database | PostgreSQL 16, Atlas migrations |
| Frontend | React 19, Vite, shadcn/ui, Tailwind CSS 4, TanStack |
| Observability | Prometheus, structured JSON logging (slog) |
| Auth | bcrypt, SHA-256 API keys, RBAC |
| Tooling | Justfile, mise, pre-commit, GoReleaser |

## License

MIT — see [LICENSE](LICENSE).
