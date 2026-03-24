# EPIC-12: Worker Relay — Optional Local Proxy for SDK Calls

## Outcome

The CronControl worker optionally exposes a local HTTP endpoint that acts as a relay/proxy for SDK calls (heartbeats, results, chat, artifacts). Tasks running on the same host as the worker can send telemetry to `localhost` instead of directly to the control plane, gaining resilience, lower latency, and buffering. The relay is opt-in and only benefits tasks that execute on the worker host.

## Why This Epic Exists

Today, SDKs call the CronControl control plane directly for heartbeats, results, artifacts, and chat. This works but has three weaknesses:

1. **Fragility**: If the control plane is temporarily unavailable (deploy, network blip), the SDK call blocks or fails inside the running task, potentially affecting its execution.
2. **Latency**: Every heartbeat is a remote HTTP call (50-200ms). For tasks reporting progress every second, this adds up.
3. **Network isolation**: In private networks, tasks may not have direct route to the control plane. The worker already has connectivity — tasks should leverage it.

## Scope: Which Execution Methods Benefit

The relay only works when the task process runs **on the same host** as the worker binary.

| Execution Method | Relay Works? | Why |
|-----------------|-------------|-----|
| HTTP (sync) | No | HTTP tasks call external URLs, no subprocess spawned |
| HTTP (async_tracked) | No | Same — the "task" is a remote HTTP server |
| SSH (foreground) | No | Task runs on remote SSH host, not on worker host |
| SSH (detached) | No | Same — remote host |
| SSM | No | Task runs on EC2 instance, not on worker host |
| K8s | Partial | Only if worker and task pod share network (sidecar pattern) |
| Container (Docker) | Yes | If worker runs on Swarm manager, containers share host network |
| Future: subprocess/exec | Yes | Task is a child process of the worker |

**Key insight**: The relay is most useful for a future subprocess/exec execution method where the worker spawns tasks as local child processes. For SSH/SSM, tasks must continue calling the control plane directly.

## How It Works

### Architecture

```
Task Process (same host)     Worker (localhost:9091)           CronControl
    |                              |                              |
    |-- POST /heartbeat ---------->|                              |
    |<-- 202 Accepted -------------|                              |
    |                              |-- (buffer, batch, retry) --->|
    |                              |<-- 200 OK -------------------|
    |                              |                              |
    |-- PATCH /runs/{id}/result -->|                              |
    |<-- 202 Accepted -------------|                              |
    |                              |-- forward (with API key) --->|
```

### Opt-in Activation

Disabled by default. Enabled via worker flag:

```bash
croncontrol-worker --url https://croncontrol.io --credential wrk_cred_abc \
    --relay-port 9091
```

When enabled:
- Worker starts a local HTTP server on `--relay-port` (default: 0 = disabled)
- Worker injects `CRONCONTROL_RELAY_URL=http://localhost:9091` into task environment
- SDKs check `CRONCONTROL_RELAY_URL` first, fall back to `CRONCONTROL_URL`

### Authentication Model

The control plane includes a **task-scoped API key** in the task payload dispatched to the worker. This key has the same permissions as the workspace API key but is short-lived (expires when the task finishes).

```
Control Plane                    Worker                        Task
    |                              |                            |
    |-- dispatch(task, api_key) -->|                            |
    |                              |-- inject RELAY_URL ------->|
    |                              |   store api_key locally    |
    |                              |                            |
    |                              |<-- POST /heartbeat --------|
    |                              |   (no auth, unauthenticated endpoint)
    |                              |                            |
    |                              |<-- PATCH /result ----------|
    |<-- forward + X-API-Key ------|   (relay adds stored key)  |
```

**Heartbeats**: unauthenticated (both on relay and control plane). No key needed.
**Results, chat, artifacts**: relay injects the task-scoped API key from the stored payload.

This requires:
- New field `TaskAPIKey string` in `worker.Task` struct
- Control plane generates a short-lived key on dispatch, includes it in the task payload
- Worker stores it per active task and adds `X-API-Key` header when forwarding

### Environment Variable Naming

Current state (inconsistent):
- Worker injects: `CRONCONTROL_API_URL`
- SDKs read: `CRONCONTROL_URL`
- Relay adds: `CRONCONTROL_RELAY_URL`

**Resolution**: Standardize on `CRONCONTROL_URL` everywhere. The worker should inject `CRONCONTROL_URL` (not `CRONCONTROL_API_URL`). SDKs already read it. The relay URL is a separate variable:

| Variable | Set By | Used For |
|----------|--------|----------|
| `CRONCONTROL_URL` | Worker or user | Base URL for all SDK calls |
| `CRONCONTROL_RELAY_URL` | Worker (when relay enabled) | Override for write operations |
| `CRONCONTROL_API_KEY` | Worker or user | Authentication |

### SDK Changes

Both `CRONCONTROL_RELAY_URL` detection and write routing need to be added. This is ~10 lines per SDK, not 1 line.

```python
# Python SDK
class CronControl:
    def __init__(self, base_url=None, api_key=None):
        self.relay_url = os.environ.get('CRONCONTROL_RELAY_URL')
        self.base_url = (base_url or os.environ.get('CRONCONTROL_URL', 'http://localhost:8080')).rstrip('/')
        self.api_key = api_key or os.environ.get('CRONCONTROL_API_KEY', '')

    def _request(self, method, path, body=None, use_relay=False):
        base = self.relay_url if (use_relay and self.relay_url) else self.base_url
        # ... rest unchanged

    def heartbeat(self, run_id, total, current, message=''):
        return self._request('POST', '/heartbeat', body={...}, use_relay=True)

    def set_result(self, run_id, data):
        return self._request('PATCH', f'/runs/{run_id}/result', body=data, use_relay=True)

    # Read operations always go direct
    def get_result(self, run_id):
        return self._request('GET', f'/runs/{run_id}/result', use_relay=False)
```

### Relayed Endpoints

| Endpoint | Buffered | Batched | Auth Required | Notes |
|----------|----------|---------|---------------|-------|
| `POST /heartbeat` | Yes | Yes (aggregate by run_id) | No | Most frequent |
| `PATCH /runs/{id}/result` | Yes | No | Yes (task key) | Forward immediately |
| `POST /orchestras/{id}/chat` | Yes | No | Yes (task key) | Forward immediately |
| `POST /runs/{id}/artifacts` | Yes | No | Yes (task key) | Stream-forward |
| `GET /*` | No | No | — | Pass-through or reject |

### Buffering Strategy

```
Heartbeats:
- Buffer in memory: map[run_id] → latest heartbeat
- Flush every 3 seconds (configurable)
- On flush: POST /heartbeat for each buffered entry
- On control plane error: retain buffer, retry next flush
- Max buffer: 1000 entries (oldest evicted on overflow)

Results / Chat / Artifacts:
- Forward immediately (no aggregation)
- On failure: retry 3x with 1s backoff
- On persistent failure: return 502 to SDK (task can handle or ignore)
```

## Configuration

```bash
# Worker flags
--relay-port 9091          # Enable relay on this port (0 = disabled, default)
--relay-flush-interval 3s  # Heartbeat flush interval (default 3s)
--relay-max-buffer 1000    # Max buffered heartbeats (default 1000)
--relay-retry-count 3      # Retries for forwarded calls (default 3)
```

No control plane configuration needed beyond generating the task-scoped API key on dispatch.

## Data Model

### Task struct change

```go
type Task struct {
    // ... existing fields
    TaskAPIKey string `json:"task_api_key,omitempty"` // short-lived workspace key for relay forwarding
}
```

### Task-scoped API key

Generated on dispatch, stored in `api_keys` table with:
- `name`: `task_{run_id}` or `task_{job_id}`
- `role`: `operator`
- `expires_at`: task timeout or 24h max
- Deleted on task completion

No schema migration needed — uses existing `api_keys` table.

## Implementation Notes

### SSH/SSM Detached Env Gap

The SSH detached start path (`ssh.go` line 140) does not pass `params.Environment` to `runRemoteCommand`. This is a pre-existing gap. Even without the relay, `CRONCONTROL_URL` and custom env vars are not available to detached SSH tasks. This should be fixed independently of EPIC-12.

### K8s Sidecar Pattern

For K8s, the relay can work if the worker runs as a sidecar container in the same pod as the task. In this case, `localhost` correctly resolves within the pod network. This is an advanced deployment pattern documented separately.

## Security

- Relay listens on `localhost` only — not exposed to external network
- No authentication on relay endpoints (same-host trust model)
- Relay authenticates to control plane using task-scoped API key (not worker credential)
- Task-scoped API key is short-lived and auto-deleted on task completion
- Artifacts forwarded with task-scoped key (workspace-scoped permissions)

## Out of Scope

- Persistent buffering (disk-based) — memory only
- Relay for SSH/SSM tasks (they run on remote hosts)
- Multi-worker relay aggregation
- WebSocket/SSE relay for real-time chat streaming
- Subprocess/exec execution method (separate epic)

## Dependencies

- EPIC-11 (async execution handles — worker control poll)
- Worker binary (`cmd/croncontrol-worker`)
- All 5 SDKs
- Fix: rename `CRONCONTROL_API_URL` → `CRONCONTROL_URL` in worker (pre-requisite)

## Acceptance Criteria

- [ ] Worker `--relay-port` flag enables local HTTP relay
- [ ] Heartbeats buffered and flushed in batch every 3s
- [ ] Results, chat, artifacts forwarded immediately with task-scoped API key
- [ ] Task-scoped API key generated on dispatch, deleted on completion
- [ ] `CRONCONTROL_RELAY_URL` injected into task environment
- [ ] SDKs detect relay URL and use it for write operations (heartbeat, result, chat, artifacts)
- [ ] SDKs use direct URL for read operations
- [ ] Relay gracefully handles control plane unavailability (buffer, retry, 502)
- [ ] Relay listens on localhost only
- [ ] No impact when relay is disabled (default behavior unchanged)
- [ ] Worker logs relay stats (buffered count, flush success/failure)
- [ ] Environment variable renamed: `CRONCONTROL_API_URL` → `CRONCONTROL_URL`
