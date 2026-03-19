package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/id"
)

// Monitor checks active orchestras for timeout and budget limits.
type Monitor struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	interval time.Duration
	stop     chan struct{}
}

// NewMonitor creates a new orchestra monitor.
func NewMonitor(pool *pgxpool.Pool, interval time.Duration) *Monitor {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &Monitor{
		pool:     pool,
		queries:  db.New(pool),
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins the monitoring loop.
func (m *Monitor) Start(ctx context.Context) {
	go m.loop(ctx)
	slog.Info("orchestra monitor started", "interval", m.interval)
}

// Stop signals the monitor to stop.
func (m *Monitor) Stop() {
	close(m.stop)
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkTimeouts(ctx)
			m.checkBudgets(ctx)
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) checkTimeouts(ctx context.Context) {
	// Find active orchestras with timeout_at in the past
	rows, err := m.pool.Query(ctx,
		`SELECT id, workspace_id, name FROM orchestras
		 WHERE state IN ('active', 'waiting_for_choice')
		 AND timeout_at IS NOT NULL
		 AND timeout_at < now()`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var orchID, wsID, name string
		rows.Scan(&orchID, &wsID, &name)

		slog.Warn("orchestra: timeout", "orchestra", name, "id", orchID)

		// Fail the orchestra
		m.queries.UpdateOrchestraState(ctx, orchID, "failed")

		// Kill running movements
		m.pool.Exec(ctx,
			`UPDATE runs SET state = 'killed', updated_at = now()
			 WHERE orchestra_id = $1 AND state IN ('running', 'pending', 'queued')`, orchID)

		// Post chat message
		m.postSystemChat(ctx, orchID, "Orchestra timed out. All running movements have been killed.", "warning")
	}
}

func (m *Monitor) checkBudgets(ctx context.Context) {
	// Find active orchestras with budget defined
	rows, err := m.pool.Query(ctx,
		`SELECT id, workspace_id, name, budget, budget_used FROM orchestras
		 WHERE state IN ('active', 'waiting_for_choice')
		 AND budget IS NOT NULL
		 AND budget != '{}'`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var orchID, wsID, name string
		var budgetRaw, usedRaw []byte
		rows.Scan(&orchID, &wsID, &name, &budgetRaw, &usedRaw)

		var budget, used map[string]float64
		json.Unmarshal(budgetRaw, &budget)
		json.Unmarshal(usedRaw, &used)

		exceeded := false
		warning := false

		for key, limit := range budget {
			current := used[key]
			ratio := current / limit

			if ratio >= 1.0 {
				exceeded = true
				slog.Warn("orchestra: budget exceeded", "orchestra", name, "key", key, "limit", limit, "used", current)
			} else if ratio >= 0.8 && !m.hasRecentWarning(ctx, orchID, key) {
				warning = true
				m.postSystemChat(ctx, orchID,
					fmt.Sprintf("Budget warning: %s at %.0f%% (%.0f/%.0f)", key, ratio*100, current, limit), "warning")
			}
		}

		if exceeded {
			m.queries.UpdateOrchestraState(ctx, orchID, "failed")
			m.pool.Exec(ctx,
				`UPDATE runs SET state = 'killed', updated_at = now()
				 WHERE orchestra_id = $1 AND state IN ('running', 'pending', 'queued')`, orchID)
			m.postSystemChat(ctx, orchID, "Orchestra failed: budget exceeded.", "warning")
		}

		_ = warning
	}
}

func (m *Monitor) hasRecentWarning(ctx context.Context, orchID, key string) bool {
	var count int64
	m.pool.QueryRow(ctx,
		`SELECT count(*) FROM orchestra_chat
		 WHERE orchestra_id = $1 AND message_type = 'warning'
		 AND content LIKE $2 AND created_at > now() - interval '10 minutes'`,
		orchID, "%"+key+"%").Scan(&count)
	return count > 0
}

func (m *Monitor) postSystemChat(ctx context.Context, orchID, content, msgType string) {
	systemType := "system"
	m.queries.CreateChatMessage(ctx, db.CreateChatMessageParams{
		ID:          id.New("msg_"),
		OrchestraID: orchID,
		SenderType:  systemType,
		MessageType: msgType,
		Content:     content,
	})
}
