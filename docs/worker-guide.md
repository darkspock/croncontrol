# CronControl Worker Guide

## Overview

The CronControl Worker is a lightweight binary deployed on customer infrastructure. It acts as an execution runtime and network gateway, allowing CronControl to run tasks inside private networks that the control plane cannot reach directly.

## Architecture

```
Control Plane (SaaS/hosted)          Customer Network
┌──────────────────────┐             ┌───────────────────────┐
│  CronControl Server  │◄── HTTPS ──►│  CronControl Worker   │
│  - API               │  outbound   │  - Polls for tasks    │
│  - Dispatcher        │  only       │  - Executes locally   │
│  - Dashboard         │             │  - Reports results    │
└──────────────────────┘             └───────────┬───────────┘
                                                 │
                                     ┌───────────▼───────────┐
                                     │  Private targets      │
                                     │  - SSH hosts          │
                                     │  - Internal HTTP APIs │
                                     │  - K8s clusters       │
                                     │  - AWS SSM instances  │
                                     └───────────────────────┘
```

## Key Properties

- **Outbound only**: The worker initiates all connections. No inbound ports required.
- **One workspace**: Each worker authenticates to exactly one workspace.
- **Long polling**: The worker polls `GET /api/v1/workers/poll` for tasks.
- **Heartbeat**: Reports liveness every 15 seconds to `POST /api/v1/workers/heartbeat`.
- **Execution methods**: Supports HTTP and SSH from the worker's local network. SSM and K8s support depends on the worker having the necessary AWS/K8s credentials.

## Enrollment Flow

### 1. Admin creates worker

```bash
curl -X POST http://croncontrol.example.com/api/v1/workers \
  -H "X-API-Key: cc_live_..." \
  -H "Content-Type: application/json" \
  -d '{"name": "prod-worker-01", "max_concurrency": 5}'
```

Response includes a temporary enrollment token (valid for 1 hour):

```json
{
  "data": {
    "worker": { "id": "wrk_01HYX...", "name": "prod-worker-01" },
    "enrollment_token": "enroll_abc123...",
    "hint": "Use this token to enroll the worker binary."
  }
}
```

### 2. Worker binary enrolls

```bash
croncontrol-worker --url http://croncontrol.example.com --credential enroll_abc123...
```

Or via the enrollment API:

```bash
curl -X POST http://croncontrol.example.com/api/v1/workers/enroll \
  -H "Content-Type: application/json" \
  -d '{"token": "enroll_abc123..."}'
```

Response includes the permanent credential:

```json
{
  "data": {
    "worker_id": "wrk_01HYX...",
    "credential": "wrk_cred_def456...",
    "hint": "Use this credential with the worker binary. It will not be shown again."
  }
}
```

### 3. Worker runs with permanent credential

```bash
croncontrol-worker --url http://croncontrol.example.com --credential wrk_cred_def456...
```

Or via environment variables:

```bash
export CRONCONTROL_URL=http://croncontrol.example.com
export CRONCONTROL_CREDENTIAL=wrk_cred_def456...
croncontrol-worker
```

## Worker Status

| Status | Condition |
|--------|-----------|
| `online` | Heartbeat received within 60 seconds |
| `offline` | No heartbeat for 60 seconds |
| `unhealthy` | 5 consecutive failures |

Recovery: returns to `online` after 3 consecutive healthy heartbeats.

## Concurrency

- Each worker has a `max_concurrency` limit (default: 5).
- The dispatcher checks `CountRunningByWorker` before dispatching.
- If a worker is at capacity, the task is not dispatched and waits for availability.

## Failure Handling

| Scenario | Behavior |
|----------|----------|
| Worker goes offline | Runs in `waiting_for_worker` are reassigned to pending (for other workers or direct execution) |
| Worker crashes mid-task | Running tasks become `hung` after the monitor detects heartbeat/execution timeout |
| No worker available | Run transitions to `waiting_for_worker` with a reason recorded |
| Worker disabled by admin | Stops receiving new tasks. Running tasks continue to completion |

## Task Execution

The worker uses the same execution method implementations as the control plane:

- **HTTP**: Sends requests to URLs reachable from the worker's network
- **SSH**: Connects to hosts accessible from the worker

Environment variables injected into every task:

- `CRONCONTROL_RUN_ID`: The run or job ID (for heartbeat reporting)
- `CRONCONTROL_API_URL`: The control plane URL (for heartbeat API calls)

## Security

- Worker credentials are separate from API keys (prefixed `wrk_cred_`).
- Credentials are SHA-256 hashed in the database.
- Enrollment tokens expire after 1 hour and are single-use.
- Communication is always HTTPS in production.
- No secrets from the control plane are stored on the worker filesystem.
