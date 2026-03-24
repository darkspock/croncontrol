# EPIC-12: Worker Relay — Optional Local Proxy for SDK Calls

## Outcome

The CronControl worker optionally exposes a local HTTP endpoint that acts as a relay for all SDK write calls (heartbeats, results, chat, artifacts). Tasks running as subprocesses on the worker host send telemetry to `localhost` instead of directly to the control plane, gaining resilience, lower latency, and buffering.

## Why This Epic Exists

When a worker runs a local subprocess (EPIC-13 `exec` method), the subprocess uses the CronControl SDK to report heartbeats, set results, post chat messages, and upload artifacts. These calls go directly to the control plane over the network. If the control plane is temporarily unavailable, the SDK call blocks or fails — impacting the running task.

The relay buffers, batches, and retries these calls locally. The task always gets a fast `202 Accepted` from localhost and continues working.

## Scope

The relay benefits tasks that run **on the same host** as the worker:

| Execution Method | Relay Works? | Notes |
|-----------------|-------------|-------|
| `exec` (EPIC-13) | **Yes** | Primary use case — subprocess on worker host |
| Container (Docker) | Yes | If containers share host network |
| K8s (sidecar) | Partial | Only with sidecar deployment pattern |
| HTTP / SSH / SSM | No | Tasks run on remote hosts |

## How It Works

### Architecture

```
Subprocess (same host)       Worker Relay (localhost:9091)     CronControl
    |                              |                              |
    |-- POST /heartbeat ---------->|                              |
    |<-- 202 Accepted -------------|                              |
    |                              |-- batch flush every 3s ----->|
    |                              |                              |
    |-- PATCH /runs/{id}/result -->|                              |
    |<-- 202 Accepted -------------|                              |
    |                              |-- POST /workers/{id}/relay ->|
    |                              |   (worker credential auth)   |
    |                              |<-- 200 OK -------------------|
```

### Authentication

No task-scoped API keys. The relay uses the **worker credential** to forward all calls through a single control plane endpoint:

```
POST /api/v1/workers/{id}/relay
X-Worker-Credential: wrk_cred_abc123
Content-Type: application/json

{
  "method": "PATCH",
  "path": "/api/v1/runs/run_01ABC/result",
  "body": {"data": {"deleted": 42}}
}
```

The control plane verifies the worker credential, checks the run/job belongs to the worker's workspace, and executes the operation internally.

**Heartbeats** are the exception — they are unauthenticated on both relay and control plane, so they are forwarded directly to `POST /api/v1/heartbeat` without the relay endpoint.

### Opt-in Activation

```bash
croncontrol-worker --url https://croncontrol.io --credential wrk_cred_abc \
    --relay-port 9091
```

When enabled:
- Worker starts local HTTP server on `--relay-port` (default: 0 = disabled)
- Worker injects `CRONCONTROL_RELAY_URL=http://localhost:9091` into subprocess env
- SDKs detect `CRONCONTROL_RELAY_URL` and route write operations through it

### Environment Variables (Breaking Change)

Rename `CRONCONTROL_API_URL` → `CRONCONTROL_URL` everywhere:

| Variable | Set By | Used For |
|----------|--------|----------|
| `CRONCONTROL_URL` | Worker or user | Base URL for all SDK calls |
| `CRONCONTROL_RELAY_URL` | Worker (when relay enabled) | Override for write operations |
| `CRONCONTROL_API_KEY` | Worker or user | Authentication |
| `CRONCONTROL_RUN_ID` | Worker | Current run ID |

### SDK Changes (~10 lines per SDK)

```python
class CronControl:
    def __init__(self, base_url=None, api_key=None):
        self.relay_url = os.environ.get('CRONCONTROL_RELAY_URL')
        self.base_url = (base_url or os.environ.get('CRONCONTROL_URL', 'http://localhost:8080')).rstrip('/')
        self.api_key = api_key or os.environ.get('CRONCONTROL_API_KEY', '')

    def _request(self, method, path, body=None, use_relay=False):
        base = self.relay_url if (use_relay and self.relay_url) else self.base_url
        # ... existing logic

    # Write operations → relay
    def heartbeat(self, run_id, total, current, message=''):
        return self._request('POST', '/heartbeat', body={...}, use_relay=True)
    def set_result(self, run_id, data):
        return self._request('PATCH', f'/runs/{run_id}/result', body=data, use_relay=True)
    def post_chat(self, orchestra_id, content, msg_type='text'):
        return self._request('POST', f'/orchestras/{orchestra_id}/chat', body={...}, use_relay=True)

    # Read operations → direct
    def get_result(self, run_id):
        return self._request('GET', f'/runs/{run_id}/result')
```

### Relayed Endpoints

| Endpoint | Buffered | Batched | Forwarded Via |
|----------|----------|---------|---------------|
| `POST /heartbeat` | Yes | Yes (latest per run_id, flush 3s) | Direct (unauthenticated) |
| `PATCH /runs/{id}/result` | Yes | No | `/workers/{id}/relay` |
| `POST /orchestras/{id}/chat` | Yes | No | `/workers/{id}/relay` |
| `POST /runs/{id}/artifacts` | Yes | No | `/workers/{id}/relay` |

### Buffering

```
Heartbeats:
- map[run_id] → latest heartbeat
- Flush every 3s (configurable)
- On error: retain, retry next flush
- Max 1000 entries (evict oldest on overflow)

Results / Chat / Artifacts:
- Forward immediately, retry 3x with 1s backoff
- On persistent failure: return 502 to SDK
```

## Control Plane: Relay Endpoint

New endpoint on the control plane:

```go
// POST /api/v1/workers/{id}/relay
// Auth: worker credential
func (s *Service) WorkerRelay(w http.ResponseWriter, r *http.Request) {
    worker := authenticateWorkerRequest(r)

    var req struct {
        Method string          `json:"method"`
        Path   string          `json:"path"`
        Body   json.RawMessage `json:"body"`
    }

    // Validate path is in allowlist
    // Execute the operation internally with workspace context
    // Return result to relay
}
```

Allowlisted paths:
- `PATCH /api/v1/runs/*/result`
- `POST /api/v1/orchestras/*/chat`
- `POST /api/v1/runs/*/artifacts`

## Configuration

```bash
--relay-port 9091          # 0 = disabled (default)
--relay-flush-interval 3s  # Heartbeat flush interval
--relay-max-buffer 1000    # Max buffered heartbeats
--relay-retry-count 3      # Retries for forwarded calls
```

## Data Model

No database changes. No new tables. The relay endpoint uses existing worker authentication.

## Security

- Relay listens on localhost only
- No auth on relay (same-host trust)
- Relay → control plane uses worker credential (existing auth)
- Control plane validates workspace ownership on every relayed operation
- Allowlisted paths prevent arbitrary API access through relay

## Dependencies

- **EPIC-13** (exec method — primary use case for relay)
- EPIC-11 (Start/Poll/Kill contract)
- All 5 SDKs

## Out of Scope

- Persistent buffering (disk)
- Relay for SSH/SSM tasks (remote hosts)
- WebSocket/SSE relay

## Acceptance Criteria

- [ ] Worker `--relay-port` flag enables local HTTP relay
- [ ] Heartbeats buffered and flushed in batch every 3s
- [ ] Results, chat, artifacts forwarded via `/workers/{id}/relay`
- [ ] Control plane relay endpoint with worker credential auth and path allowlist
- [ ] `CRONCONTROL_RELAY_URL` injected into subprocess env
- [ ] SDKs detect relay URL for write operations
- [ ] Relay handles control plane unavailability (buffer, retry, 502)
- [ ] localhost only binding
- [ ] No impact when disabled
- [ ] Breaking change: `CRONCONTROL_API_URL` → `CRONCONTROL_URL`
