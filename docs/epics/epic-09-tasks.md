# EPIC-09 Tasks: Orchestras — Dynamic Workflow Orchestration

> Status: ~95% — backend complete, dashboard complete, 2 minor items remaining (shared space, chat file sharing) — updated 2026-03-19

## Phase 1: Foundation (Result + Secrets + Artifacts)

### T09.1 Run Result (JSONB)
- [x] Add `result JSONB` column to runs table (migration)
- [x] `PATCH /runs/{id}/result` — set result (max 1MB, validates JSON)
- [x] `GET /runs/{id}/result` — get result
- [x] SDK: `set_result(run_id, data)` and `get_result(run_id)` in all 5 SDKs
- [x] `CRONCONTROL_TRIGGERED_BY` env var injected when run is triggered by another run

### T09.2 Workspace Secrets Vault
- [x] `workspace_secrets` table: id, workspace_id, name, value_enc (AES-256-GCM), created_at
- [x] `GET /secrets` — list names only, never values
- [x] `POST /secrets` — create (name + value, encrypted at rest)
- [x] `PUT /secrets/{name}` — update value
- [x] `DELETE /secrets/{name}` — delete
- [x] Dashboard: Settings > Secrets tab (list, create, update, reveal/hide, delete)
- [x] SDK: `create_secret`, `list_secrets`, `delete_secret` in all 5 SDKs

### T09.3 Run Artifacts (S3/MinIO + Local)
- [x] `run_artifacts` table: id, run_id, workspace_id, name, content_type, size_bytes, storage_key
- [x] Artifact storage backend interface: `Upload`, `Download`, `Delete`, `GetURL`
- [x] S3/MinIO backend implementation
- [x] Local filesystem backend (fallback)
- [x] `POST /runs/{id}/artifacts` — upload (multipart)
- [x] `GET /runs/{id}/artifacts` — list
- [x] `GET /runs/{id}/artifacts/{name}` — download
- [x] Config: `CC_ARTIFACTS_BACKEND`, `CC_ARTIFACTS_S3_ENDPOINT`, `CC_ARTIFACTS_S3_BUCKET`, etc.
- [x] SDK: `upload_artifact`, `get_artifact_url`, `list_artifacts` in all 5 SDKs

## Phase 2: Orchestra Core

### T09.4 Orchestra Table & Lifecycle
- [x] `orchestras` table: id, workspace_id, name, director_type, director_process_id, ai_config, state, movement_count, secrets, summary, parent_orchestra_id, wait_for_parent, budget, budget_used, timeout, timeout_at, created_at, updated_at
- [x] Orchestra states: active, waiting_for_choice, paused, completed, cancelled, failed
- [x] `POST /orchestras` — create (name, director, first_agent_node, secrets, timeout, budget)
- [x] `GET /orchestras` — list (filter by state)
- [x] `GET /orchestras/{id}` — get metadata
- [x] `GET /orchestras/{id}/score` — full score (all movements with results)
- [x] `POST /orchestras/{id}/cancel` — cancel active orchestra
- [x] Add `orchestra_id`, `orchestra_step` columns to runs table (migration)
- [x] SDK: `create_orchestra`, `get_score`, `cancel_orchestra` in all 5 SDKs

### T09.5 Next Movement (Dynamic Trigger)
- [x] `POST /runs/{id}/next` — trigger next movement (process_id, payload, message)
- [x] Creates new run with: orchestra_id, orchestra_step+1, triggered_by_run_id
- [x] Inherits orchestra secrets as env vars
- [x] Injects: `CRONCONTROL_ORCHESTRA_ID`, `CRONCONTROL_ORCHESTRA_STEP`, `CRONCONTROL_TRIGGERED_BY`
- [x] Increments orchestra movement_count
- [x] SDK: `next_movement(process_id, payload, message)` in all 5 SDKs

### T09.6 Director Auto-Trigger
- [x] After movement completes/fails: if orchestra has a director, trigger it automatically
- [x] Inject event env vars: `CRONCONTROL_EVENT_TYPE`, `CRONCONTROL_EVENT_RUN_ID`, `CRONCONTROL_EVENT_RESULT`
- [x] Director process runs, reads event via `cc.get_event()`, decides next action
- [x] SDK: `get_event()` (reads env vars, returns structured event) in all 5 SDKs
- [x] If director_type=none: no auto-trigger (AgentNodes chain directly)

### T09.7 Ask Choice (Human-in-the-Loop)
- [x] Add `choice_config JSONB`, `chosen_index INTEGER` columns to runs table
- [x] Add `waiting_for_choice` run state + state machine transitions
- [x] `POST /runs/{id}/choice` — set choice config (message + choices array)
- [x] `POST /runs/{id}/choose` — human selects a choice (choice_index)
- [x] On choose: if choice has process_id → trigger next movement; if null → finish/cancel
- [x] Update orchestra state: active ↔ waiting_for_choice
- [x] SDK: `ask_choice(run_id, message, choices)` and `ask_confirm(run_id, message, on_approve, on_reject)` in all 5 SDKs
- [x] Dashboard RunDetail: show choice buttons when waiting_for_choice

### T09.8 Finish Orchestra
- [x] `POST /orchestras/{id}/finish` — set state=completed, summary
- [x] SDK: `finish_orchestra(orchestra_id, summary)` in all 5 SDKs
- [x] Webhook event: `orchestra.completed`

## Phase 3: Chat System

### T09.9 Orchestra Chat Table & API
- [x] `orchestra_chat` table: id, orchestra_id, sender_type (system/director/agent_node/human), sender_id, message_type (text/result/request_help/action/choice/choice_response/file/status/warning), content, data JSONB, created_at
- [x] `POST /orchestras/{id}/chat` — post message
- [x] `GET /orchestras/{id}/chat` — list messages (pagination, since=timestamp)
- [x] System messages auto-posted: movement started/completed/failed, budget warnings
- [x] SDK: `post_chat(orchestra_id, message, type, data)` and `get_chat(orchestra_id, since)` in all 5 SDKs

### T09.10 Chat Actions
- [x] Actions field in chat messages: array of {label, action_name, params, style}
- [x] `POST /orchestras/{id}/chat/{message_id}/action` — trigger an action
- [x] When action triggered: creates a new run for the action handler OR triggers director with event
- [x] SDK: `register_action(name, handler)` and `post_chat(..., actions=[...])` in all 5 SDKs

### T09.11 Real-Time Chat (SSE)
- [x] `GET /orchestras/{id}/stream` — SSE endpoint
- [x] Events: chat, movement, choice, state, budget_warning
- [x] Dashboard: real-time chat panel on Score page
- [x] Human input box: post text messages and click action buttons

## Phase 4: AI Director

### T09.12 Multi-Model AI Director
- [x] Built-in Go component: `internal/orchestra/ai_director.go`
- [x] Provider interface: `CallLLM(model, system, messages, tools) → response`
- [x] Anthropic provider (Claude API with tool_use)
- [x] OpenAI provider (GPT API with function_calling)
- [x] Google provider (Gemini API with function_calling)
- [x] Tools: next_movement, ask_choice, post_chat, launch_sub_orchestra, finish_orchestra, get_score, list_agent_nodes
- [x] System prompt includes: orchestra name, available AgentNodes, budget remaining
- [x] Context includes: full score, last event, chat history
- [x] Fallback: 3 retries, then ask_choice to human with available options

### T09.13 AI Director Budget Tracking
- [x] Count AI calls per orchestra in budget_used
- [x] Warning at 80% in chat
- [x] Fail orchestra at 100%
- [x] Webhook events: `orchestra.budget_warning`, `orchestra.budget_exceeded`

## ~~Phase 5: Sub-Orchestras~~ DEFERRED
> Moved to future epic. Orchestras are flat for now — no nesting.

## Phase 5: Container Execution (Docker Swarm)


### T09.16 Container Executor
- [x] `internal/executor/container/container.go`: implements Method interface
- [x] Docker SDK client (`github.com/docker/docker/client`)
- [x] `Execute()`: create Swarm service → poll tasks → collect logs → remove service
- [x] `Kill()`: remove service
- [x] Resource limits: cpu, memory from method_config
- [x] Env var injection: all standard vars + orchestra secrets
- [x] Config: `CC_CONTAINER_SWARM_ENDPOINT`, `CC_CONTAINER_DEFAULT_CPU`, `CC_CONTAINER_DEFAULT_MEMORY`

### T09.17 Registry Authentication
- [x] method_config.registry_auth references a workspace secret
- [x] Container executor decrypts and passes to `docker service create --with-registry-auth`
- [x] Support: Docker Hub, ghcr.io, AWS ECR, custom registries

### T09.18 Container Pool Dashboard
- [x] `GET /admin/container-pool` — list Swarm nodes (platform admin)
- [x] Dashboard: Settings > Infrastructure tab (server list, state badges, containers, cost, provision/destroy)
- [x] Health status per node (state badges: provisioning/ready/active/idle/destroying)

## Phase 7: Timeout, Budget & Pause

### T09.19 Orchestra Timeout
- [x] `timeout` and `timeout_at` fields on orchestras table
- [x] Monitor goroutine checks active orchestras for timeout
- [x] On timeout: kill running movements, state=failed, chat message, webhook `orchestra.timeout`

### T09.20 Orchestra Budget
- [x] `budget` and `budget_used` JSONB fields on orchestras table
- [x] Counters: movements, ai_calls, duration_minutes
- [x] Check before each movement/AI call
- [x] 80% warning: chat + webhook
- [x] 100% exceeded: fail orchestra + webhook

### T09.21 Pause / Resume
- [x] `POST /orchestras/{id}/pause` — state=paused, no new movements triggered
- [x] `POST /orchestras/{id}/resume` — state=active, process queued events
- [x] Running movements continue to completion during pause
- [x] Completed events queued (stored in orchestra_events table or chat)
- [x] On resume: director triggered with accumulated events
- [x] SDK: `pause_orchestra(id)` and `resume_orchestra(id)` in all 5 SDKs

## Phase 8: Dashboard

### T09.22 Orchestra List Page
- [x] New sidebar item: "Orchestras" (between Queue and Infrastructure sections)
- [x] List: name, director type badge, state badge, movement count, duration, budget used
- [x] Filters: state, date range
- [x] Click → Score view

### T09.23 Score View Page
- [x] `/orchestras/{id}` route
- [x] Vertical timeline of movements: process name, state, duration, result preview
- [x] Choice buttons inline when waiting_for_choice
- [x] Orchestra metadata header: name, director, state, budget, timeout
- [x] ~~Sub-orchestras expandable tree~~ (descoped — no sub-orchestras)

### T09.24 Chat Panel
- [x] Real-time chat panel (SSE) on Score page
- [x] Message types rendered differently: text, status (gray), warning (amber), file (link), action (buttons)
- [x] Human input box at bottom
- [x] Action buttons clickable

### T09.25 RunDetail Orchestra Integration
- [x] Show result JSON (collapsible section)
- [x] Show artifacts with download links
- [x] Orchestra breadcrumb: Orchestra "name" → Movement N
- [x] Choice buttons when waiting_for_choice

## Phase 9: Storage Isolation

### T09.26 Isolated S3 Prefixes
- [x] S3 key format: `{workspace_id}/{orchestra_id}/{run_id}/{artifact_name}` (orchestra runs) or `{workspace_id}/{run_id}/{artifact_name}` (standalone runs)
- [ ] Orchestra-level shared space: `{workspace_id}/{orchestra_id}/shared/`
- [x] ~~Sub-orchestras get their own prefix under parent~~ (descoped — no sub-orchestras)
- [ ] File sharing via chat: message contains reference (orchestra_id + run_id + name), not copy
- [x] Access control: workspace-level isolation enforced at query layer

## Acceptance Checklist
- [x] Phase 1: Result, secrets, artifacts all working with SDK support
- [x] Phase 2: Orchestra lifecycle, movements, director trigger, human choices
- [x] Phase 3: Chat with actions, real-time SSE
- [x] Phase 4: AI Director multi-model with fallback
- [x] Phase 5: Container executor on Docker Swarm
- [x] Phase 7: Timeout, budget, pause/resume
- [x] Phase 8: Full dashboard with score view, chat, secrets, and infrastructure
- [x] Phase 9: Storage isolation per orchestra/run (S3 key includes orchestra_id)
