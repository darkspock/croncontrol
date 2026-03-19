# EPIC-09 Tasks: Orchestras — Dynamic Workflow Orchestration

> Status: NOT STARTED — created 2026-03-19

## Phase 1: Foundation (Result + Secrets + Artifacts)

### T09.1 Run Result (JSONB)
- [ ] Add `result JSONB` column to runs table (migration)
- [ ] `PATCH /runs/{id}/result` — set result (max 1MB, validates JSON)
- [ ] `GET /runs/{id}/result` — get result
- [ ] SDK: `set_result(run_id, data)` and `get_result(run_id)` in all 5 SDKs
- [ ] `CRONCONTROL_TRIGGERED_BY` env var injected when run is triggered by another run

### T09.2 Workspace Secrets Vault
- [ ] `workspace_secrets` table: id, workspace_id, name, value_enc (AES-256-GCM), created_at
- [ ] `GET /secrets` — list names only, never values
- [ ] `POST /secrets` — create (name + value, encrypted at rest)
- [ ] `PUT /secrets/{name}` — update value
- [ ] `DELETE /secrets/{name}` — delete
- [ ] Dashboard: Settings > Secrets tab (list, create, delete, warning about one-time visibility)
- [ ] SDK: `create_secret`, `list_secrets`, `delete_secret` in all 5 SDKs

### T09.3 Run Artifacts (S3/MinIO + Local)
- [ ] `run_artifacts` table: id, run_id, workspace_id, name, content_type, size_bytes, storage_key
- [ ] Artifact storage backend interface: `Upload`, `Download`, `Delete`, `GetURL`
- [ ] S3/MinIO backend implementation
- [ ] Local filesystem backend (fallback)
- [ ] `POST /runs/{id}/artifacts` — upload (multipart)
- [ ] `GET /runs/{id}/artifacts` — list
- [ ] `GET /runs/{id}/artifacts/{name}` — download
- [ ] Config: `CC_ARTIFACTS_BACKEND`, `CC_ARTIFACTS_S3_ENDPOINT`, `CC_ARTIFACTS_S3_BUCKET`, etc.
- [ ] SDK: `upload_artifact`, `get_artifact_url`, `list_artifacts` in all 5 SDKs

## Phase 2: Orchestra Core

### T09.4 Orchestra Table & Lifecycle
- [ ] `orchestras` table: id, workspace_id, name, director_type, director_process_id, ai_config, state, movement_count, secrets, summary, parent_orchestra_id, wait_for_parent, budget, budget_used, timeout, timeout_at, created_at, updated_at
- [ ] Orchestra states: active, waiting_for_choice, paused, completed, cancelled, failed
- [ ] `POST /orchestras` — create (name, director, first_musician, secrets, timeout, budget)
- [ ] `GET /orchestras` — list (filter by state)
- [ ] `GET /orchestras/{id}` — get metadata
- [ ] `GET /orchestras/{id}/score` — full score (all movements with results)
- [ ] `POST /orchestras/{id}/cancel` — cancel active orchestra
- [ ] Add `orchestra_id`, `orchestra_step` columns to runs table (migration)
- [ ] SDK: `create_orchestra`, `get_score`, `cancel_orchestra` in all 5 SDKs

### T09.5 Next Movement (Dynamic Trigger)
- [ ] `POST /runs/{id}/next` — trigger next movement (process_id, payload, message)
- [ ] Creates new run with: orchestra_id, orchestra_step+1, triggered_by_run_id
- [ ] Inherits orchestra secrets as env vars
- [ ] Injects: `CRONCONTROL_ORCHESTRA_ID`, `CRONCONTROL_ORCHESTRA_STEP`, `CRONCONTROL_TRIGGERED_BY`
- [ ] Increments orchestra movement_count
- [ ] SDK: `next_movement(process_id, payload, message)` in all 5 SDKs

### T09.6 Director Auto-Trigger
- [ ] After movement completes/fails: if orchestra has a director, trigger it automatically
- [ ] Inject event env vars: `CRONCONTROL_EVENT_TYPE`, `CRONCONTROL_EVENT_RUN_ID`, `CRONCONTROL_EVENT_RESULT`
- [ ] Director process runs, reads event via `cc.get_event()`, decides next action
- [ ] SDK: `get_event()` (reads env vars, returns structured event) in all 5 SDKs
- [ ] If director_type=none: no auto-trigger (musicians chain directly)

### T09.7 Ask Choice (Human-in-the-Loop)
- [ ] Add `choice_config JSONB`, `chosen_index INTEGER` columns to runs table
- [ ] Add `waiting_for_choice` run state + state machine transitions
- [ ] `POST /runs/{id}/choice` — set choice config (message + choices array)
- [ ] `POST /runs/{id}/choose` — human selects a choice (choice_index)
- [ ] On choose: if choice has process_id → trigger next movement; if null → finish/cancel
- [ ] Update orchestra state: active ↔ waiting_for_choice
- [ ] SDK: `ask_choice(run_id, message, choices)` and `ask_confirm(run_id, message, on_approve, on_reject)` in all 5 SDKs
- [ ] Dashboard RunDetail: show choice buttons when waiting_for_choice

### T09.8 Finish Orchestra
- [ ] `POST /orchestras/{id}/finish` — set state=completed, summary
- [ ] SDK: `finish_orchestra(orchestra_id, summary)` in all 5 SDKs
- [ ] Webhook event: `orchestra.completed`

## Phase 3: Chat System

### T09.9 Orchestra Chat Table & API
- [ ] `orchestra_chat` table: id, orchestra_id, sender_type (system/director/musician/human), sender_id, message_type (text/result/request_help/action/choice/choice_response/file/status/warning), content, data JSONB, created_at
- [ ] `POST /orchestras/{id}/chat` — post message
- [ ] `GET /orchestras/{id}/chat` — list messages (pagination, since=timestamp)
- [ ] System messages auto-posted: movement started/completed/failed, budget warnings
- [ ] SDK: `post_chat(orchestra_id, message, type, data)` and `get_chat(orchestra_id, since)` in all 5 SDKs

### T09.10 Chat Actions
- [ ] Actions field in chat messages: array of {label, action_name, params, style}
- [ ] `POST /orchestras/{id}/chat/{message_id}/action` — trigger an action
- [ ] When action triggered: creates a new run for the action handler OR triggers director with event
- [ ] SDK: `register_action(name, handler)` and `post_chat(..., actions=[...])` in all 5 SDKs

### T09.11 Real-Time Chat (SSE)
- [ ] `GET /orchestras/{id}/stream` — SSE endpoint
- [ ] Events: chat, movement, choice, state, budget_warning
- [ ] Dashboard: real-time chat panel on Score page
- [ ] Human input box: post text messages and click action buttons

## Phase 4: AI Director

### T09.12 Multi-Model AI Director
- [ ] Built-in Go component: `internal/orchestra/ai_director.go`
- [ ] Provider interface: `CallLLM(model, system, messages, tools) → response`
- [ ] Anthropic provider (Claude API with tool_use)
- [ ] OpenAI provider (GPT API with function_calling)
- [ ] Google provider (Gemini API with function_calling)
- [ ] Tools: next_movement, ask_choice, post_chat, launch_sub_orchestra, finish_orchestra, get_score, list_musicians
- [ ] System prompt includes: orchestra name, available musicians, budget remaining
- [ ] Context includes: full score, last event, chat history
- [ ] Fallback: 3 retries, then ask_choice to human with available options

### T09.13 AI Director Budget Tracking
- [ ] Count AI calls per orchestra in budget_used
- [ ] Warning at 80% in chat
- [ ] Fail orchestra at 100%
- [ ] Webhook events: `orchestra.budget_warning`, `orchestra.budget_exceeded`

## Phase 5: Sub-Orchestras

### T09.14 Launch Sub-Orchestra
- [ ] `POST /orchestras` with `parent_orchestra_id` and `wait_for_parent`
- [ ] SDK: `launch_sub_orchestra(parent_id, name, director, first_musician, wait)`
- [ ] Sub-orchestra inherits parent secrets (can add more)
- [ ] Sub-orchestra has its own chat (isolated)
- [ ] Sub-orchestra has its own S3 prefix (isolated)
- [ ] Budget shared: sub-orchestra costs count against parent

### T09.15 Sub-Orchestra Completion
- [ ] When sub-orchestra completes: event sent to parent director
- [ ] If wait=True: parent director receives `sub_orchestra_completed` event with summary
- [ ] If wait=False: event posted to parent chat only
- [ ] If parent cancelled/failed: cascade cancel to active sub-orchestras
- [ ] Max depth: configurable (default 5)

## Phase 6: Container Execution (Docker Swarm)

### T09.16 Container Executor
- [ ] `internal/executor/container/container.go`: implements Method interface
- [ ] Docker SDK client (`github.com/docker/docker/client`)
- [ ] `Execute()`: create Swarm service → poll tasks → collect logs → remove service
- [ ] `Kill()`: remove service
- [ ] Resource limits: cpu, memory from method_config
- [ ] Env var injection: all standard vars + orchestra secrets
- [ ] Config: `CC_CONTAINER_SWARM_ENDPOINT`, `CC_CONTAINER_DEFAULT_CPU`, `CC_CONTAINER_DEFAULT_MEMORY`

### T09.17 Registry Authentication
- [ ] method_config.registry_auth references a workspace secret
- [ ] Container executor decrypts and passes to `docker service create --with-registry-auth`
- [ ] Support: Docker Hub, ghcr.io, AWS ECR, custom registries

### T09.18 Container Pool Dashboard
- [ ] `GET /admin/container-pool` — list Swarm nodes (platform admin)
- [ ] Dashboard: Settings > Container Pool (node list, CPU/memory usage, containers running)
- [ ] Health status per node

## Phase 7: Timeout, Budget & Pause

### T09.19 Orchestra Timeout
- [ ] `timeout` and `timeout_at` fields on orchestras table
- [ ] Monitor goroutine checks active orchestras for timeout
- [ ] On timeout: kill running movements, state=failed, chat message, webhook `orchestra.timeout`

### T09.20 Orchestra Budget
- [ ] `budget` and `budget_used` JSONB fields on orchestras table
- [ ] Counters: movements, ai_calls, duration_minutes
- [ ] Check before each movement/AI call
- [ ] 80% warning: chat + webhook
- [ ] 100% exceeded: fail orchestra + webhook

### T09.21 Pause / Resume
- [ ] `POST /orchestras/{id}/pause` — state=paused, no new movements triggered
- [ ] `POST /orchestras/{id}/resume` — state=active, process queued events
- [ ] Running movements continue to completion during pause
- [ ] Completed events queued (stored in orchestra_events table or chat)
- [ ] On resume: director triggered with accumulated events
- [ ] SDK: `pause_orchestra(id)` and `resume_orchestra(id)` in all 5 SDKs

## Phase 8: Dashboard

### T09.22 Orchestra List Page
- [ ] New sidebar item: "Orchestras" (between Queue and Infrastructure sections)
- [ ] List: name, director type badge, state badge, movement count, duration, budget used
- [ ] Filters: state, date range
- [ ] Click → Score view

### T09.23 Score View Page
- [ ] `/orchestras/{id}` route
- [ ] Vertical timeline of movements: process name, state, duration, result preview
- [ ] Choice buttons inline when waiting_for_choice
- [ ] Orchestra metadata header: name, director, state, budget, timeout
- [ ] Sub-orchestras expandable tree

### T09.24 Chat Panel
- [ ] Real-time chat panel (SSE) on Score page
- [ ] Message types rendered differently: text, status (gray), warning (amber), file (link), action (buttons)
- [ ] Human input box at bottom
- [ ] Action buttons clickable

### T09.25 RunDetail Orchestra Integration
- [ ] Show result JSON (collapsible section)
- [ ] Show artifacts with download links
- [ ] Orchestra breadcrumb: Orchestra "name" → Movement N
- [ ] Choice buttons when waiting_for_choice

## Phase 9: Storage Isolation

### T09.26 Isolated S3 Prefixes
- [ ] S3 key format: `{workspace_id}/{orchestra_id}/{run_id}/{artifact_name}`
- [ ] Orchestra-level shared space: `{workspace_id}/{orchestra_id}/shared/`
- [ ] Sub-orchestras get their own prefix under parent
- [ ] File sharing via chat: message contains reference (orchestra_id + run_id + name), not copy
- [ ] Access control: only runs within same orchestra can read each other's artifacts

## Acceptance Checklist
- [ ] Phase 1: Result, secrets, artifacts all working with SDK support
- [ ] Phase 2: Orchestra lifecycle, movements, director trigger, human choices
- [ ] Phase 3: Chat with actions, real-time SSE
- [ ] Phase 4: AI Director multi-model with fallback
- [ ] Phase 5: Sub-orchestras with wait/parallel
- [ ] Phase 6: Container executor on Docker Swarm
- [ ] Phase 7: Timeout, budget, pause/resume
- [ ] Phase 8: Full dashboard with score view and chat
- [ ] Phase 9: Storage isolation per orchestra/run
