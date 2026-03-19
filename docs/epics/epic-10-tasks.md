# EPIC-10 Tasks: Serverless Infrastructure

**Status**: ~30% — Design complete, Hetzner client + provisioner + migration created, not yet functional end-to-end.

**Created**: 2026-03-19

---

## Phase 1: Database & Hetzner Client

### T10.1: workspace_servers Table
- [x] Create migration `00010_workspace_servers.sql`
- [x] Table with: id, workspace_id, hetzner_id, name, ip_address, state, server_type, datacenter, swarm_token, containers_running, max_containers, last_activity_at, monthly_cost, created_at, destroyed_at, updated_at
- [x] State CHECK constraint: provisioning, ready, active, idle, destroying, destroyed
- [x] Index on (workspace_id, state)
- [ ] SQLC queries: CreateServer, GetServer, ListServersByWorkspace, UpdateServerState, IncrementContainers, DecrementContainers, ListIdleServers, MarkServerDestroyed

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
- [ ] checkIdleServers loop (every 5 min)
- [ ] Grace period: configurable idle timeout (default 1h)
- [ ] Server naming: `cc-{workspace_short}-{ulid}`

### T10.5: Container Executor Integration
- [ ] Container executor checks workspace servers before dispatching
- [ ] If no server with capacity → call EnsureCapacity, wait up to 3 min
- [ ] Swarm placement constraint: `node.labels.workspace == <ID>`
- [ ] Increment/decrement container count on server record
- [ ] Handle server becoming unavailable mid-execution

---

## Phase 3: API Endpoints

### T10.6: Infrastructure API
- [ ] GET `/infra/servers` — list workspace servers (admin)
- [ ] POST `/infra/servers` — manually provision a server (admin)
- [ ] DELETE `/infra/servers/{id}` — manually destroy a server (admin)
- [ ] POST `/infra/servers/{id}/ready` — server ready callback (infra-secret auth)
- [ ] GET `/infra/pool` — pool overview: servers, capacity, cost (admin)
- [ ] Wire routes in main.go

---

## Phase 4: Configuration

### T10.7: Config
- [ ] Add infra section to config.yaml: enabled, provider, hetzner_api_token, datacenter, server_type, ssh_key_name, swarm_manager_ip, swarm_join_token, grace_period, max_servers_per_workspace, infra_secret
- [ ] Environment variable overrides (CC_INFRA_*)
- [ ] Validate config on startup when infra.enabled = true

---

## Phase 5: Dashboard

### T10.8: Settings > Infrastructure Tab
- [ ] Server list: name, IP, state badge, containers (2/4), cost, created date
- [ ] Provision button (manual)
- [ ] Destroy button with confirmation dialog
- [ ] Total cost display
- [ ] Empty state when infra not enabled

### T10.9: Platform Admin > Infrastructure
- [ ] All servers across all workspaces
- [ ] Total cost, total capacity
- [ ] Utilization percentage
- [ ] Filter by workspace

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
- [ ] 2x multiplier applied in display, raw cost stored
- [ ] Cost breakdown in Settings > Infrastructure

---

## Acceptance Criteria

- [ ] Hetzner API client: create, delete, get server
- [ ] Cloud-init script: Docker install + Swarm join + ready callback
- [ ] `workspace_servers` table with full lifecycle
- [ ] Auto-provisioning: create server when workspace needs capacity
- [ ] Idle destruction: destroy server after grace period with no containers
- [ ] Swarm placement constraints: containers only run on workspace nodes
- [ ] Server ready callback: `/infra/servers/{id}/ready`
- [ ] Dashboard: server list with state, capacity, cost
- [ ] Platform admin: cross-workspace infrastructure view
- [ ] Configuration: Hetzner token, datacenter, server type, grace period
- [ ] 2x pricing model reflected in dashboard
- [ ] Servers destroyed on workspace deletion
