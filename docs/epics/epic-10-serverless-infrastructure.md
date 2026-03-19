# EPIC-10: Serverless Infrastructure — Auto-Provisioned Dedicated Servers

## Outcome

CronControl automatically provisions and manages dedicated Hetzner servers per workspace. When a workspace needs container execution, servers are created on-demand, Docker is installed, and they join the workspace's Swarm cluster. Servers are destroyed when no longer needed.

## Why This Epic Exists

EPIC-09 introduced the container execution method using Docker Swarm. But Swarm nodes must exist before containers can run. This epic automates the infrastructure layer: CronControl provisions servers via the Hetzner API, installs Docker, joins them to Swarm, and tears them down — all transparently.

## Pricing Model

- **Server**: Hetzner CX22 (2 vCPU, 4GB RAM) = €4.5/month
- **Capacity**: 4 containers per server (0.5 CPU each)
- **Pricing to workspace**: 2x server cost = **€9/month per server**
- **Billing unit**: per server, not per container

| Containers needed | Servers | Cost to workspace |
|------------------|---------|------------------|
| 1-4 | 1 | €9/mo |
| 5-8 | 2 | €18/mo |
| 9-12 | 3 | €27/mo |
| 13-16 | 4 | €36/mo |

## How It Works

### Lifecycle

```
1. Workspace creates first orchestra with container execution
2. CronControl checks: workspace has servers? No → provision
3. Hetzner API: create CX22 server
4. Cloud-init: install Docker, join Swarm, report ready
5. Server registered in DB as workspace infrastructure
6. Container executor uses this server for musicians
7. When no active orchestras for X hours → server marked idle
8. Idle server destroyed after grace period (configurable, default 1h)
```

### Provisioning Flow

```
CronControl                    Hetzner API              New Server
    |                              |                        |
    |-- POST /servers ------------>|                        |
    |<-- server_id, ip ------------|                        |
    |                              |                        |
    |                              |-- cloud-init --------->|
    |                              |   apt install docker   |
    |                              |   docker swarm join    |
    |                              |   curl /ready -------->|
    |<------- heartbeat -----------------------------------|
    |                                                       |
    | Server ready. Dispatch containers.                    |
```

### Server States

```
provisioning → ready → active → idle → destroying → destroyed
                                  ↓
                               active  (new orchestra started)
```

- **provisioning**: Hetzner API called, waiting for cloud-init
- **ready**: Docker installed, Swarm joined, waiting for work
- **active**: Running containers
- **idle**: No containers running for grace_period
- **destroying**: Hetzner API delete called
- **destroyed**: Server gone, record kept for billing

## Data Model

### New table: `workspace_servers`

```sql
CREATE TABLE workspace_servers (
    id              TEXT PRIMARY KEY,       -- srv_ + ULID
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    hetzner_id      BIGINT NOT NULL,        -- Hetzner server ID
    name            VARCHAR(100) NOT NULL,
    ip_address      VARCHAR(45),
    state           VARCHAR(20) NOT NULL DEFAULT 'provisioning'
                    CHECK (state IN ('provisioning', 'ready', 'active', 'idle', 'destroying', 'destroyed')),
    server_type     VARCHAR(20) NOT NULL DEFAULT 'cx22',
    datacenter      VARCHAR(20) NOT NULL DEFAULT 'fsn1',
    swarm_token     TEXT,
    containers_running INTEGER NOT NULL DEFAULT 0,
    max_containers  INTEGER NOT NULL DEFAULT 4,
    last_activity_at TIMESTAMPTZ,
    monthly_cost    NUMERIC(10,2) NOT NULL DEFAULT 4.50,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    destroyed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_servers_workspace ON workspace_servers(workspace_id, state);
```

## Hetzner API Integration

### `internal/infra/hetzner.go`

```go
type HetznerClient struct {
    apiToken string
    client   *http.Client
}

// CreateServer provisions a new CX22 in the configured datacenter.
func (h *HetznerClient) CreateServer(name, sshKeyName, cloudInit string) (serverID int64, ip string, err error)

// DeleteServer destroys a server by Hetzner ID.
func (h *HetznerClient) DeleteServer(serverID int64) error

// GetServer returns server status.
func (h *HetznerClient) GetServer(serverID int64) (*ServerInfo, error)
```

### Cloud-Init Script

Injected as `user_data` when creating the server:

```bash
#!/bin/bash
set -e

# Install Docker
curl -fsSL https://get.docker.com | sh

# Join Swarm (manager IP and token from CronControl)
docker swarm join --token {{SWARM_TOKEN}} {{MANAGER_IP}}:2377

# Label node with workspace ID
docker node update --label-add workspace={{WORKSPACE_ID}} $(hostname)

# Report ready to CronControl
curl -X POST {{CRONCONTROL_URL}}/api/v1/infra/servers/{{SERVER_ID}}/ready \
  -H "Authorization: Bearer {{INFRA_SECRET}}"
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/infra/servers` | admin | List workspace servers |
| POST | `/infra/servers` | admin | Manually provision a server |
| DELETE | `/infra/servers/{id}` | admin | Manually destroy a server |
| POST | `/infra/servers/{id}/ready` | infra-secret | Server reports ready (called by cloud-init) |
| GET | `/infra/pool` | admin | Pool overview: servers, capacity, cost |

## Auto-Provisioning

### `internal/infra/provisioner.go`

Background component that:

1. **On container needed**: checks if workspace has capacity
   - If yes: use existing server
   - If no: create new server via Hetzner API
2. **Server ready callback**: marks server as ready, starts dispatching
3. **Idle check** (every 5 min): servers with no containers for `grace_period` → destroy
4. **Capacity planning**: always keep 1 server with free slots (optional warm pool)

```go
type Provisioner struct {
    hetzner     *HetznerClient
    queries     *db.Queries
    pool        *pgxpool.Pool
    managerIP   string
    swarmToken  string
    gracePeriod time.Duration  // default 1h
}

func (p *Provisioner) EnsureCapacity(ctx context.Context, workspaceID string, needed int) error
func (p *Provisioner) CheckIdleServers(ctx context.Context) error
func (p *Provisioner) DestroyServer(ctx context.Context, serverID string) error
```

## Container Executor Integration

The container executor checks workspace servers before dispatching:

```go
func (m *Method) Execute(ctx context.Context, params ExecuteParams) (Result, error) {
    // 1. Find a workspace server with capacity
    server := findServerWithCapacity(params.WorkspaceID)

    // 2. If no server: request provisioning, wait
    if server == nil {
        server = provisioner.EnsureCapacity(ctx, params.WorkspaceID, 1)
        // Wait for server to be ready (polling, max 3 min)
    }

    // 3. Create Swarm service constrained to workspace node
    spec.TaskTemplate.Placement = &swarm.Placement{
        Constraints: []string{
            fmt.Sprintf("node.labels.workspace == %s", params.WorkspaceID),
        },
    }

    // 4. Execute as before
}
```

## Configuration

```yaml
# config.yaml
infra:
  enabled: false                    # enable auto-provisioning
  provider: hetzner                 # only hetzner for now
  hetzner_api_token: ""             # CC_INFRA_HETZNER_TOKEN
  datacenter: fsn1                  # fsn1, nbg1, hel1
  server_type: cx22                 # cx22 = 2 vCPU, 4GB
  ssh_key_name: croncontrol         # pre-created in Hetzner
  swarm_manager_ip: ""              # IP of the Swarm manager
  swarm_join_token: ""              # docker swarm join-token worker
  grace_period: 1h                  # idle before destroy
  max_servers_per_workspace: 10
  infra_secret: ""                  # for server ready callback auth
```

## Dashboard

### Settings > Infrastructure (new tab, admin only)

- Server list: name, IP, state badge, containers (2/4), cost, created date
- Provision button (manual)
- Destroy button (with confirmation)
- Total cost display

### Platform Admin > Infrastructure

- All servers across all workspaces
- Total cost, total capacity
- Utilization percentage

## Billing

### Monthly cost tracking

```sql
-- Add to workspace_servers
monthly_cost NUMERIC(10,2) NOT NULL DEFAULT 4.50

-- View: monthly cost per workspace
SELECT workspace_id,
       count(*) as servers,
       sum(monthly_cost) * 2 as workspace_cost
FROM workspace_servers
WHERE state NOT IN ('destroyed')
GROUP BY workspace_id;
```

The 2x multiplier is applied in the billing display, not stored. Raw cost is Hetzner cost.

## Security

- Servers are in the same Hetzner project (private network possible)
- SSH key pre-created in Hetzner, CronControl has the private key
- Cloud-init uses an `infra_secret` to authenticate the ready callback
- Swarm join token rotated periodically
- Servers labeled by workspace — Swarm placement constraints prevent cross-workspace access
- Servers destroyed on workspace deletion

## Out of Scope

- Multi-provider support (AWS, GCP, Azure) — Hetzner only for now
- Auto-scaling based on queue depth (manual provisioning + idle destruction only)
- Spot/preemptible instances
- Custom server types per workspace (cx22 only)
- Network isolation between workspaces (Swarm labels, not VLANs)

## Dependencies

- EPIC-09 (Orchestras + Container executor)
- Hetzner API token
- Pre-created SSH key in Hetzner
- Swarm manager node (CronControl server or dedicated)

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
