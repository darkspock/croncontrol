# EPIC-09: Orchestras — Dynamic Workflow Orchestration

## Outcome

Enable runtime-driven workflow orchestration where an Orchestra Director coordinates AgentNodes (tasks) via a shared chat, shares results between movements, requests human decisions, supports sub-orchestras, and optionally uses AI (multi-model) to make autonomous decisions — all without keeping processes alive.

## Why This Epic Exists

CronControl schedules and executes individual tasks. But real operational workflows are multi-step, conditional, and require coordination between tasks and humans. This epic makes CronControl the orchestration layer for dynamic, multi-step workflows with real-time visibility.

## Core Terminology

| Term | Definition |
|------|-----------|
| **Orchestra** | A named group of related executions. Has its own lifecycle, director, chat, secrets, storage, budget, and timeout. Created from code via SDK. |
| **Director** | The decision-maker. Runs after each movement. Can be: a user process (code), a built-in AI Director (Claude/GPT/Gemini), or none (direct chaining). |
| **AgentNode** | Any process that does actual work. AgentNodes can post to the chat, request help, and share files — but don't manage the workflow. |
| **Movement** | A single run within an orchestra. Has a step number. |
| **Score** | The full history: all movements, results, chat messages, and decisions. |
| **Chat** | A shared message channel for the orchestra. Director, AgentNodes, and humans can post messages and trigger actions. |

## Lifecycle

```
1. SDK creates orchestra → state: active
2. First AgentNode triggered → movement 1
3. AgentNode executes, posts to chat, sets result, exits
4. CronControl triggers Director with event
5. Director reads score + chat + result, decides:
   a) next_movement → triggers AgentNode (goto 3)
   b) ask_choice → presents options to human, state: waiting_for_choice
   c) launch_sub_orchestra → creates child orchestra (wait or parallel)
   d) finish_orchestra → state: completed
6. Human can intervene via chat at any point
7. Timeout or budget exceeded → state: failed, notify
```

### Orchestra States

```
active → waiting_for_choice     (movement asked for human input)
active → paused                 (manual pause, context persisted)
active → completed              (director finished)
active → cancelled              (manual cancel)
active → failed                 (timeout, budget exceeded, unrecoverable error)
waiting_for_choice → active     (human chose, next movement triggered)
paused → active                 (resumed, context reloaded)
```

## Chat System

Every orchestra has a chat channel. Participants: Director, AgentNodes, Humans.

### Message Types

| Type | Who sends | What it does |
|------|-----------|--------------|
| `text` | anyone | Informational message |
| `result` | agent node | Structured JSON result attached |
| `request_help` | agent node | Asks for assistance, director/human can respond |
| `action` | director/human | Triggers a programmatic action (defined in code) |
| `choice` | director | Presents buttons to human |
| `choice_response` | human | Human clicked a choice button |
| `file` | anyone | Shares an artifact via message |
| `status` | system | Movement started/completed/failed |
| `warning` | system | Budget 80%, timeout approaching |

### Chat Actions

AgentNodes and directors can register actions that are triggerable via chat:

```python
# In an AgentNode
cc.register_action("retry_failed", handler=retry_failed_items)
cc.post_chat("Found 5 failures. Retry?", actions=[
    {"label": "Retry all", "action": "retry_failed", "params": {"all": True}},
    {"label": "Skip", "action": None},
])

# Human clicks "Retry all" in dashboard → CronControl calls the registered handler
```

### SDK Chat Methods

```python
cc.post_chat(orchestra_id, "Processing 150 items...")
cc.post_chat(orchestra_id, "Need review", type="request_help", data={...})
cc.get_chat(orchestra_id, since=timestamp)  # for polling
cc.register_action(name, handler)
```

### Dashboard Chat View

Real-time chat panel on the orchestra Score page (SSE/WebSocket):
```
[system]     Movement 1: scrape-products started
[agent node]   Scraping https://example.com...
[agent node]   Found 150 products ✅
[system]     Movement 1: completed (2.3s)
[director]   Launching validation for 150 products
[system]     Movement 2: validate-products started
[agent node]   30 invalid products found
[agent node]   ⚠️ Need review — 5 products have missing prices
[human]      @director skip the 5 missing prices, delete the rest
[director]   Acknowledged. Presenting options:
[director]   [🔴 Delete 25] [Export CSV] [Ignore]
[human]      → clicked "Delete 25"
[system]     Movement 3: delete-products started
```

## Sub-Orchestras

An orchestra can launch child orchestras:

```python
# Director launches a sub-orchestra
sub = cc.launch_sub_orchestra(
    parent_orchestra_id,
    name="run-tests",
    director="prc_test_director",
    first_agent_node="prc_unit_tests",
    wait=True,   # parent waits for child to complete
    # wait=False  # parent continues in parallel
)

# If wait=True, director gets event when sub-orchestra finishes:
# event.type = "sub_orchestra_completed"
# event.result = sub-orchestra's final summary
```

### Sub-Orchestra Rules

- Each sub-orchestra has its own chat (isolated)
- Each sub-orchestra has its own storage space
- Sub-orchestras inherit parent's secrets (can add more)
- Parent can wait (`wait=True`) or continue (`wait=False`)
- If parent is cancelled/failed, active sub-orchestras are also cancelled
- Sub-orchestra result is returned to parent director as an event
- Budget is shared: sub-orchestra costs count against parent's budget

### Data Model

```sql
ALTER TABLE orchestras ADD COLUMN parent_orchestra_id TEXT REFERENCES orchestras(id);
ALTER TABLE orchestras ADD COLUMN wait_for_parent BOOLEAN DEFAULT false;
```

## AI Director (Multi-Model)

Built-in Go component. Supports multiple LLM providers:

| Provider | Model examples | Config key |
|----------|---------------|------------|
| Anthropic | claude-sonnet-4-20250514, claude-opus-4 | `ANTHROPIC_API_KEY` |
| OpenAI | gpt-4o, gpt-4-turbo | `OPENAI_API_KEY` |
| Google | gemini-2.0-flash, gemini-pro | `GOOGLE_AI_API_KEY` |

```python
orchestra = cc.create_orchestra("cleanup",
    director="ai",
    ai_config={
        "provider": "anthropic",  # or "openai", "google"
        "model": "claude-sonnet-4-20250514",
        "system_prompt": "You are an ops director. Be conservative with deletes.",
        "temperature": 0.1,
    },
    first_agent_node="prc_scrape",
    secrets=["ANTHROPIC_API_KEY"],
)
```

### AI Director Tools

The AI Director receives these tools via the LLM's tool_use:

```json
[
  {"name": "next_movement", "description": "Trigger an AgentNode process", "parameters": {"process_id": "string", "payload": "object"}},
  {"name": "ask_choice", "description": "Present choices to human", "parameters": {"message": "string", "choices": "array"}},
  {"name": "post_chat", "description": "Send message to orchestra chat", "parameters": {"message": "string"}},
  {"name": "launch_sub_orchestra", "description": "Launch a child orchestra", "parameters": {"name": "string", "director": "string", "first_agent_node": "string", "wait": "boolean"}},
  {"name": "finish_orchestra", "description": "End the orchestra", "parameters": {"summary": "string"}},
  {"name": "get_score", "description": "Read full orchestra history", "parameters": {}},
  {"name": "list_agent_nodes", "description": "List available processes", "parameters": {}}
]
```

### Fallback

If LLM API fails after 3 retries:
- Posts to chat: "AI Director unavailable. Please decide manually."
- Presents ask_choice with available AgentNodes + cancel option
- Human takes over

## Timeout & Budget

### Timeout

```python
orchestra = cc.create_orchestra("cleanup",
    director="prc_director",
    first_agent_node="prc_scrape",
    timeout="2h",  # cancel if not finished in 2 hours
)
```

When timeout is reached:
- Orchestra state → `failed`
- Running movements are killed
- Chat message: "Orchestra timed out after 2h"
- Webhook event: `orchestra.timeout`

### Budget

```python
orchestra = cc.create_orchestra("ai-analysis",
    director="ai",
    ai_config={"provider": "anthropic", "model": "claude-sonnet-4-20250514"},
    budget={
        "max_movements": 20,          # max AgentNode executions
        "max_ai_calls": 50,           # max LLM API calls
        "max_duration_minutes": 120,   # max wall-clock time
    },
)
```

Budget tracking:
- Counters stored on orchestra record
- At 80%: chat warning + webhook `orchestra.budget_warning`
- At 100%: orchestra state → `failed`, chat message, webhook `orchestra.budget_exceeded`

```sql
ALTER TABLE orchestras ADD COLUMN budget JSONB;
ALTER TABLE orchestras ADD COLUMN budget_used JSONB DEFAULT '{}';
ALTER TABLE orchestras ADD COLUMN timeout INTERVAL;
ALTER TABLE orchestras ADD COLUMN timeout_at TIMESTAMPTZ;
```

## Pause / Resume

```python
cc.pause_orchestra(orchestra_id)   # state → paused
cc.resume_orchestra(orchestra_id)  # state → active, context reloaded
```

### What pause does:

1. Orchestra state → `paused`
2. No new movements are triggered
3. Running movements continue to completion (not killed)
4. Director is not invoked when running movements finish
5. Completed movement results are queued
6. Chat message: "Orchestra paused by {actor}"

### What resume does:

1. Orchestra state → `active`
2. Loads queued events (movements that completed during pause)
3. Triggers director with the accumulated events
4. Chat message: "Orchestra resumed by {actor}"
5. Director processes queued events and continues workflow

Context is always in the database (score, results, chat). Nothing in memory. Resume just re-triggers the director.

## Storage

### Isolated Spaces

Each orchestra and each participant has its own S3 prefix:

```
s3://croncontrol-artifacts/
  ├── orc_01HYX.../                    # orchestra space
  │   ├── shared/                      # orchestra-level shared files
  │   ├── run_01HYX.../               # movement-specific files
  │   └── run_01HYZ.../
  └── orc_02ABC.../                    # another orchestra (isolated)
```

### Sharing Files

Files are shared by posting them as chat messages:

```python
# AgentNode uploads to its own space
cc.upload_artifact(run_id, "report.pdf", file_bytes)

# Share with the orchestra via chat
cc.post_chat(orchestra_id, "Report ready", type="file",
    artifact={"run_id": run_id, "name": "report.pdf"})

# Another AgentNode or director reads it
url = cc.get_artifact_url(run_id, "report.pdf")
```

Files are never copied — the chat message contains a reference. Access is controlled by orchestra membership.

## Real-Time Dashboard

### Technology

Server-Sent Events (SSE) on `GET /orchestras/{id}/stream`:

```
event: chat
data: {"type": "text", "from": "agent_node", "message": "Processing..."}

event: movement
data: {"type": "started", "process": "scrape-products", "step": 1}

event: movement
data: {"type": "completed", "step": 1, "duration_ms": 2300}

event: choice
data: {"message": "Delete 30 products?", "choices": [...]}

event: state
data: {"state": "waiting_for_choice"}
```

### Dashboard Components

- **Score Timeline**: vertical timeline of movements (existing design)
- **Chat Panel**: real-time message list, input box for human messages, action buttons
- **Status Bar**: orchestra state, movement count, budget usage, time elapsed/remaining
- **Sub-Orchestras**: expandable tree showing child orchestras and their status

## Configuration

```yaml
# config.yaml
artifacts:
  backend: s3            # s3 | local
  s3_endpoint: http://localhost:9000
  s3_bucket: croncontrol-artifacts
  s3_access_key: minioadmin
  s3_secret_key: minioadmin
  s3_region: us-east-1
  local_path: ./data/artifacts   # fallback

orchestras:
  default_timeout: 24h
  max_sub_orchestra_depth: 5
  chat_retention_days: 30
```

## Database Changes Summary

### New Tables

| Table | Purpose |
|-------|---------|
| `orchestras` | Orchestra metadata, lifecycle, budget, timeout |
| `orchestra_chat` | Chat messages with type, sender, data, actions |
| `workspace_secrets` | Encrypted key-value vault |
| `run_artifacts` | File metadata with S3 storage keys |

### Modified Tables

| Table | New Columns |
|-------|-------------|
| `runs` | `orchestra_id`, `orchestra_step`, `result`, `choice_config`, `chosen_index` |
| `orchestras` | `parent_orchestra_id`, `wait_for_parent`, `budget`, `budget_used`, `timeout`, `timeout_at` |

### New Run State

`waiting_for_choice` — post-execution, needs human decision.

## Container Execution Method (Docker Swarm)

New execution method `container` for running AgentNodes as ephemeral Docker containers on a Swarm cluster.

### Why Swarm

- AgentNodes typically need 0.5 CPU — Swarm packs 4 per 2-CPU server automatically
- Zero overhead vs Nomad/K8s — comes with Docker, 1 command to setup
- Auto bin-packing, overlay networking, built-in secrets
- Adding capacity = `docker swarm join` on a new server

### Setup

```bash
# Manager (CronControl server or dedicated)
docker swarm init

# Worker nodes (Hetzner CX22: 2 vCPU, 4GB, €4.5/mo)
docker swarm join --token SWMTKN-xxx manager-ip:2377
```

### How It Works

```
1. AgentNode has execution_method: container
2. CronControl calls Docker Swarm API to create a service:
   - image, command, env vars (including secrets + orchestra vars)
   - resource limits (cpu: 0.5, memory: 1G)
   - restart-condition: none (ephemeral)
3. Swarm schedules on least-loaded node
4. Container runs, writes to stdout/stderr
5. CronControl polls service status until exit
6. Collects logs, exit code, removes service
7. Reports result to orchestrator
```

### Process Configuration

```json
{
  "name": "scrape-products",
  "execution_method": "container",
  "method_config": {
    "image": "ghcr.io/my-org/scraper:latest",
    "command": ["python", "scrape.py"],
    "resources": {
      "cpu": "0.5",
      "memory": "1G"
    },
    "registry_auth": "DOCKER_REGISTRY_SECRET",
    "pull_policy": "always"
  }
}
```

### Container Executor Implementation

New executor: `internal/executor/container/container.go`

```go
type Method struct {
    swarmClient *docker.Client  // Docker SDK
}

func (m *Method) Execute(ctx context.Context, params ExecuteParams) (Result, error) {
    // 1. Create service spec with resource limits
    // 2. Inject env vars: CRONCONTROL_RUN_ID, CRONCONTROL_API_URL, orchestra secrets
    // 3. Create service (Swarm schedules it)
    // 4. Poll service tasks until exit
    // 5. Collect logs via docker service logs
    // 6. Remove service
    // Return stdout, stderr, exit code
}

func (m *Method) Kill(ctx context.Context, handle Handle) error {
    // docker service rm
}
```

### Configuration

```yaml
# config.yaml
container:
  enabled: true
  swarm_endpoint: unix:///var/run/docker.sock  # or tcp://manager:2375
  default_cpu: "0.5"
  default_memory: "512M"
  pull_timeout: 120s
  execution_timeout: 300s
  network: croncontrol-orchestra   # overlay network
  log_driver: json-file
```

Environment variable overrides: `CC_CONTAINER_SWARM_ENDPOINT`, `CC_CONTAINER_DEFAULT_CPU`, etc.

### Registry Authentication

Private registries configured via workspace secrets:

```python
cc.create_secret("DOCKER_REGISTRY_SECRET", json.dumps({
    "registry": "ghcr.io",
    "username": "user",
    "password": "ghp_xxx"
}))
```

The container executor reads the secret, decrypts, and passes to `docker service create` as `--with-registry-auth`.

### Resource Pool Visibility

Dashboard > Settings > Container Pool:
- List of Swarm nodes: hostname, CPU total/used, memory total/used, containers running
- Health status per node
- Add node instructions

### Scaling

```bash
# Need more capacity? Add a server:
docker swarm join --token SWMTKN-xxx manager-ip:2377
# Swarm immediately starts scheduling new AgentNodes on it

# Scale down:
docker node update --availability drain node-id
# Swarm migrates containers to other nodes
```

## Out of Scope

- Visual flow/DAG builder (orchestras are code-only)
- Saga compensation / automatic rollback
- Orchestra templates or reusable definitions
- MCP-based AI Director (direct LLM API tool_use instead)
- Video/audio streaming in chat
- End-to-end encryption of chat messages
- Kubernetes as container runtime (use K8s executor for K8s Jobs instead)
- Auto-scaling Swarm nodes (manual add/remove)

## Dependencies

- EPIC-01 through EPIC-08 (all complete)
- LLM API keys for AI Director (stored in workspace secrets)
- S3/MinIO for artifacts (local filesystem fallback)
- AES-256-GCM encryption for secrets (already exists)
- Docker Swarm cluster for container execution method (optional)
- Docker SDK for Go (`github.com/docker/docker/client`)

## Acceptance Criteria

- [ ] `orchestras` table with full lifecycle (active/waiting/paused/completed/cancelled/failed)
- [ ] `workspace_secrets` with AES-256-GCM encryption, CRUD API, never expose values
- [ ] `run_artifacts` with S3/MinIO + local backends, isolated per orchestra/run
- [ ] `result JSONB` on runs — set by AgentNode, read by director/next AgentNode
- [ ] `waiting_for_choice` state with N choices, each linked to a process
- [ ] Chat system: text, results, help requests, actions, files, system status
- [ ] Chat actions: programmable buttons that trigger handlers
- [ ] Director auto-trigger after each movement completes/fails
- [ ] AI Director: multi-model (Claude/GPT/Gemini) with tool_use and fallback
- [ ] Sub-orchestras: launch with wait or parallel, isolated chat/storage, shared budget
- [ ] Timeout: configurable per orchestra, kills running movements, fails orchestra
- [ ] Budget: max movements, max AI calls, max duration — warnings at 80%, fail at 100%
- [ ] Pause/resume: context persisted in DB, queued events replayed on resume
- [ ] Storage: isolated S3 prefixes per orchestra/run, share via chat messages
- [ ] Real-time dashboard: SSE stream, chat panel, score timeline, status bar
- [ ] Container execution method: Docker Swarm, ephemeral services, resource limits
- [ ] Container registry auth via workspace secrets
- [ ] Container pool visibility in dashboard (nodes, CPU/memory usage)
- [ ] All 5 SDKs: full orchestra methods including chat, actions, artifacts, sub-orchestras
- [ ] AgentNodes remain pure — no orchestration awareness required
- [ ] No process stays running while waiting for human decision
