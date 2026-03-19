package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/id"
)

// ProvisionerConfig holds provisioner settings.
type ProvisionerConfig struct {
	Enabled        bool
	HetznerToken   string
	Datacenter     string // fsn1, nbg1, hel1
	ServerType     string // cx22
	SSHKeyName     string
	SwarmManagerIP string
	SwarmJoinToken string
	GracePeriod    time.Duration // idle before destroy
	MaxServers     int
	InfraSecret    string // for ready callback auth
	CronControlURL string
}

// Provisioner manages workspace server lifecycle.
type Provisioner struct {
	hetzner *HetznerClient
	queries *db.Queries
	pool    *pgxpool.Pool
	config  ProvisionerConfig
	stop    chan struct{}
}

// NewProvisioner creates a new infrastructure provisioner.
func NewProvisioner(pool *pgxpool.Pool, cfg ProvisionerConfig) *Provisioner {
	return &Provisioner{
		hetzner: NewHetznerClient(cfg.HetznerToken),
		queries: db.New(pool),
		pool:    pool,
		config:  cfg,
		stop:    make(chan struct{}),
	}
}

// Start begins the idle server check loop.
func (p *Provisioner) Start(ctx context.Context) {
	if !p.config.Enabled {
		slog.Info("infra provisioner disabled")
		return
	}
	go p.loop(ctx)
	slog.Info("infra provisioner started", "datacenter", p.config.Datacenter, "grace_period", p.config.GracePeriod)
}

// Stop signals the provisioner to stop.
func (p *Provisioner) Stop() {
	close(p.stop)
}

// EnsureCapacity ensures a workspace has at least `needed` container slots available.
// Creates a new server if necessary.
func (p *Provisioner) EnsureCapacity(ctx context.Context, workspaceID string, needed int) (string, error) {
	// Check existing capacity
	var available int
	p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(max_containers - containers_running), 0)
		 FROM workspace_servers WHERE workspace_id = $1 AND state IN ('ready', 'active')`,
		workspaceID).Scan(&available)

	if available >= needed {
		// Find a server with capacity
		var serverIP string
		p.pool.QueryRow(ctx,
			`SELECT ip_address FROM workspace_servers
			 WHERE workspace_id = $1 AND state IN ('ready', 'active')
			 AND containers_running < max_containers
			 ORDER BY containers_running LIMIT 1`,
			workspaceID).Scan(&serverIP)
		return serverIP, nil
	}

	// Check max servers limit
	var serverCount int
	p.pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace_servers WHERE workspace_id = $1 AND state NOT IN ('destroyed')`,
		workspaceID).Scan(&serverCount)
	if serverCount >= p.config.MaxServers {
		return "", fmt.Errorf("max servers reached (%d)", p.config.MaxServers)
	}

	// Provision new server
	return p.provisionServer(ctx, workspaceID)
}

func (p *Provisioner) provisionServer(ctx context.Context, workspaceID string) (string, error) {
	serverID := id.New("srv_")
	serverName := fmt.Sprintf("cc-%s-%s", workspaceID[:8], serverID[4:12])

	cloudInit := fmt.Sprintf(`#!/bin/bash
set -e
apt-get update -qq
curl -fsSL https://get.docker.com | sh
docker swarm join --token %s %s:2377
docker node update --label-add workspace=%s $(hostname)
curl -sf -X POST %s/api/v1/infra/servers/%s/ready -H "Authorization: Bearer %s"
`, p.config.SwarmJoinToken, p.config.SwarmManagerIP, workspaceID,
		p.config.CronControlURL, serverID, p.config.InfraSecret)

	slog.Info("infra: provisioning server", "workspace", workspaceID, "name", serverName)

	info, err := p.hetzner.CreateServer(ctx, serverName, p.config.ServerType, p.config.Datacenter, p.config.SSHKeyName, cloudInit)
	if err != nil {
		return "", fmt.Errorf("provision server: %w", err)
	}

	// Save to DB
	p.pool.Exec(ctx,
		`INSERT INTO workspace_servers (id, workspace_id, hetzner_id, name, ip_address, state, server_type, datacenter)
		 VALUES ($1, $2, $3, $4, $5, 'provisioning', $6, $7)`,
		serverID, workspaceID, info.ID, serverName, info.PublicIP, p.config.ServerType, p.config.Datacenter)

	slog.Info("infra: server provisioning started", "server", serverName, "hetzner_id", info.ID, "ip", info.PublicIP)

	return info.PublicIP, nil
}

// MarkServerReady is called by the cloud-init ready callback.
func (p *Provisioner) MarkServerReady(ctx context.Context, serverID string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE workspace_servers SET state = 'ready', updated_at = now() WHERE id = $1 AND state = 'provisioning'`,
		serverID)
	if err == nil {
		slog.Info("infra: server ready", "server", serverID)
	}
	return err
}

// DestroyServer destroys a workspace server.
func (p *Provisioner) DestroyServer(ctx context.Context, serverID string) error {
	var hetznerID int64
	err := p.pool.QueryRow(ctx,
		`SELECT hetzner_id FROM workspace_servers WHERE id = $1`, serverID).Scan(&hetznerID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	slog.Info("infra: destroying server", "server", serverID, "hetzner_id", hetznerID)

	if err := p.hetzner.DeleteServer(ctx, hetznerID); err != nil {
		slog.Error("infra: failed to destroy server", "error", err)
		return err
	}

	p.pool.Exec(ctx,
		`UPDATE workspace_servers SET state = 'destroyed', destroyed_at = now(), updated_at = now() WHERE id = $1`,
		serverID)

	return nil
}

func (p *Provisioner) loop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkIdleServers(ctx)
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *Provisioner) checkIdleServers(ctx context.Context) {
	cutoff := time.Now().Add(-p.config.GracePeriod)

	rows, err := p.pool.Query(ctx,
		`SELECT id, name FROM workspace_servers
		 WHERE state IN ('ready', 'idle')
		 AND containers_running = 0
		 AND (last_activity_at IS NULL OR last_activity_at < $1)
		 AND created_at < $1`, cutoff)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serverID, name string
		rows.Scan(&serverID, &name)
		slog.Info("infra: idle server detected, destroying", "server", name)
		p.DestroyServer(ctx, serverID)
	}
}

// ListServers returns all servers for a workspace.
func (p *Provisioner) ListServers(ctx context.Context, workspaceID string) ([]map[string]any, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, name, ip_address, state, server_type, containers_running, max_containers, monthly_cost, created_at
		 FROM workspace_servers WHERE workspace_id = $1 AND state != 'destroyed' ORDER BY created_at`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []map[string]any
	for rows.Next() {
		var id, name, state, serverType string
		var ip *string
		var running, max int32
		var cost float64
		var created time.Time
		rows.Scan(&id, &name, &ip, &state, &serverType, &running, &max, &cost, &created)
		servers = append(servers, map[string]any{
			"id": id, "name": name, "ip_address": ip, "state": state,
			"server_type": serverType, "containers_running": running,
			"max_containers": max, "monthly_cost": cost, "workspace_cost": cost * 2,
			"created_at": created,
		})
	}
	return servers, nil
}
