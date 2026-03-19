# EPIC-10 Tasks: Serverless Infrastructure

**Status**: ~65% — Backend mostly complete (Hetzner client, provisioner, SQLC, config, Swarm constraints, admin API). Frontend dashboard done. Remaining: container executor full integration, lifecycle automation, platform admin frontend.

**Created**: 2026-03-19

---

## Phase 1: Database & Hetzner Client

### T10.1: workspace_servers Table
- [x] Create migration `00010_workspace_servers.sql`
- [x] Table with: id, workspace_id, hetzner_id, name, ip_address, state, server_type, datacenter, swarm_token, containers_running, max_containers, last_activity_at, monthly_cost, created_at, destroyed_at, updated_at
- [x] State CHECK constraint: provisioning, ready, active, idle, destroying, destroyed
- [x] Index on (workspace_id, state)
- [x] SQLC queries: CreateServer, GetServer, ListServersByWorkspace, UpdateServerState, IncrementContainers, DecrementContainers, ListIdleServers, MarkServerDestroyed, FindServerWithCapacity, CountServersByWorkspace, DestroyServersByWorkspace

### T10.2: Hetzner API Client
- [x] `internal/infra/hetzner.go` — HetznerClient struct
- [x] CreateServer(name, sshKeyName, cloudInit) → (serverID, ip, error)
- [x] DeleteServer(serverID) → error
- [x] GetServer(serverID) → (*ServerInfo, error)
- [ ] Unit tests with mock HTTP server
- [ ] Error handling: rate limits, API errors, timeouts

### T10.3: Cloud-Init Script
- [ ] Template with placeholders: SWARM_TOKEN, MANAGER_IP, WORKSPACE_ID, CRONCONTROL_URL, SERVER_ID, INFRA_SECRET
- [ ] Install Docker CE
- [ ] `docker swarm join` with worker token
- [ ] Label node with `workspace=<ID>`
- [ ] POST to `/api/v1/infra/servers/{id}/ready` on completion
- [ ] Error reporting on cloud-init failure

---

## Phase 2: Provisioner

### T10.4: Provisioner Core
- [x] `internal/infra/provisioner.go` — Provisioner struct
- [x] EnsureCapacity(ctx, workspaceID, needed) → error
- [x] MarkServerReady(ctx, serverID) → error
- [x] DestroyServer(ctx, serverID) → error
- [x] checkIdleServers loop (every 5 min) — with idle state marking before destroy
- [x] Grace period: configurable idle timeout (default 1h)
- [x] Server naming: `cc-{workspace_short}-{ulid}`

### T10.5: Container Executor Integration
- [ ] Container executor checks workspace servers before dispatching
- [ ] If no server with capacity → call EnsureCapacity, wait up to 3 min
- [x] Swarm placement constraint: `node.labels.workspace == <ID>`
- [ ] Increment/decrement container count on server record
- [ ] Handle server becoming unavailable mid-execution

---

## Phase 3: API Endpoints

### T10.6: Infrastructure API
- [x] GET `/infra/servers` — list workspace servers (admin)
- [x] POST `/infra/servers` — manually provision a server (admin)
- [x] DELETE `/infra/servers/{id}` — manually destroy a server (admin)
- [x] POST `/infra/servers/{id}/ready` — server ready callback (infra-secret auth)
- [ ] GET `/infra/pool` — pool overview: servers, capacity, cost (admin)
- [x] Wire routes in main.go

---

## Phase 4: Configuration

### T10.7: Config
- [x] Add infra section to config.yaml: enabled, provider, hetzner_api_token, datacenter, server_type, ssh_key_name, swarm_manager_ip, swarm_join_token, grace_period, max_servers_per_workspace, infra_secret
- [x] Environment variable overrides (CC_INFRA_*) — via viper AutomaticEnv
- [x] Validate config on startup when infra.enabled = true

---

## Phase 5: Dashboard

### T10.8: Settings > Infrastructure Tab
- [x] Server list: name, IP, state badge, containers (2/4), cost, created date
- [x] Provision button (manual)
- [x] Destroy button with confirmation dialog
- [x] Total cost display
- [x] Empty state when infra not enabled

### T10.9: Platform Admin > Infrastructure
- [x] All servers across all workspaces — GET /admin/infra/servers
- [ ] Total cost, total capacity (frontend)
- [ ] Utilization percentage (frontend)
- [ ] Filter by workspace (frontend)

---

## Phase 6: Lifecycle & Billing

### T10.10: Server Lifecycle
- [ ] Auto-provisioning on first container orchestra
- [ ] Idle detection: no containers for grace_period → mark idle
- [ ] Idle → destroying: call Hetzner delete
- [ ] Re-activate idle server if new orchestra starts
- [ ] Destroy servers on workspace deletion
- [ ] Handle Hetzner API failures gracefully (retry, mark error)

### T10.11: Billing Display
- [ ] Monthly cost per workspace = count(active servers) * monthly_cost * 2
- [x] 2x multiplier applied in display, raw cost stored
- [x] Cost breakdown in Settings > Infrastructure

---

## Acceptance Criteria

- [x] Hetzner API client: create, delete, get server
- [ ] Cloud-init script: Docker install + Swarm join + ready callback
- [x] `workspace_servers` table with full lifecycle
- [ ] Auto-provisioning: create server when workspace needs capacity
- [ ] Idle destruction: destroy server after grace period with no containers
- [ ] Swarm placement constraints: containers only run on workspace nodes
- [x] Server ready callback: `/infra/servers/{id}/ready`
- [x] Dashboard: server list with state, capacity, cost
- [ ] Platform admin: cross-workspace infrastructure view
- [ ] Configuration: Hetzner token, datacenter, server type, grace period
- [x] 2x pricing model reflected in dashboard
- [ ] Servers destroyed on workspace deletion
