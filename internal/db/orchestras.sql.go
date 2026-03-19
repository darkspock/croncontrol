// Orchestra queries.
// source: orchestras.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Orchestra struct {
	ID                string             `json:"id"`
	WorkspaceID       string             `json:"workspace_id"`
	Name              string             `json:"name"`
	DirectorType      string             `json:"director_type"`
	DirectorProcessID *string            `json:"director_process_id"`
	AIConfig          []byte             `json:"ai_config"`
	State             string             `json:"state"`
	MovementCount     int32              `json:"movement_count"`
	Secrets           []string           `json:"secrets"`
	Summary           *string            `json:"summary"`
	Budget            []byte             `json:"budget"`
	BudgetUsed        []byte             `json:"budget_used"`
	Timeout           pgtype.Interval    `json:"timeout"`
	TimeoutAt         pgtype.Timestamptz `json:"timeout_at"`
	CreatedAt         pgtype.Timestamptz `json:"created_at"`
	UpdatedAt         pgtype.Timestamptz `json:"updated_at"`
}

const createOrchestra = `INSERT INTO orchestras (id, workspace_id, name, director_type, director_process_id, ai_config, secrets, budget, timeout, timeout_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, workspace_id, name, director_type, director_process_id, ai_config, state, movement_count, secrets, summary, budget, budget_used, timeout, timeout_at, created_at, updated_at`

type CreateOrchestraParams struct {
	ID                string             `json:"id"`
	WorkspaceID       string             `json:"workspace_id"`
	Name              string             `json:"name"`
	DirectorType      string             `json:"director_type"`
	DirectorProcessID *string            `json:"director_process_id"`
	AIConfig          []byte             `json:"ai_config"`
	Secrets           []string           `json:"secrets"`
	Budget            []byte             `json:"budget"`
	Timeout           pgtype.Interval    `json:"timeout"`
	TimeoutAt         pgtype.Timestamptz `json:"timeout_at"`
}

func (q *Queries) CreateOrchestra(ctx context.Context, arg CreateOrchestraParams) (Orchestra, error) {
	row := q.db.QueryRow(ctx, createOrchestra, arg.ID, arg.WorkspaceID, arg.Name, arg.DirectorType, arg.DirectorProcessID, arg.AIConfig, arg.Secrets, arg.Budget, arg.Timeout, arg.TimeoutAt)
	var o Orchestra
	err := row.Scan(&o.ID, &o.WorkspaceID, &o.Name, &o.DirectorType, &o.DirectorProcessID, &o.AIConfig, &o.State, &o.MovementCount, &o.Secrets, &o.Summary, &o.Budget, &o.BudgetUsed, &o.Timeout, &o.TimeoutAt, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

const getOrchestra = `SELECT * FROM orchestras WHERE id = $1 AND workspace_id = $2`

type GetOrchestraParams struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
}

func (q *Queries) GetOrchestra(ctx context.Context, arg GetOrchestraParams) (Orchestra, error) {
	row := q.db.QueryRow(ctx, getOrchestra, arg.ID, arg.WorkspaceID)
	var o Orchestra
	err := row.Scan(&o.ID, &o.WorkspaceID, &o.Name, &o.DirectorType, &o.DirectorProcessID, &o.AIConfig, &o.State, &o.MovementCount, &o.Secrets, &o.Summary, &o.Budget, &o.BudgetUsed, &o.Timeout, &o.TimeoutAt, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

const listOrchestrasByWorkspace = `SELECT * FROM orchestras WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

type ListOrchestrasParams struct {
	WorkspaceID string `json:"workspace_id"`
	Limit       int32  `json:"limit"`
	Offset      int32  `json:"offset"`
}

func (q *Queries) ListOrchestrasByWorkspace(ctx context.Context, arg ListOrchestrasParams) ([]Orchestra, error) {
	rows, err := q.db.Query(ctx, listOrchestrasByWorkspace, arg.WorkspaceID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Orchestra{}
	for rows.Next() {
		var o Orchestra
		if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.Name, &o.DirectorType, &o.DirectorProcessID, &o.AIConfig, &o.State, &o.MovementCount, &o.Secrets, &o.Summary, &o.Budget, &o.BudgetUsed, &o.Timeout, &o.TimeoutAt, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

const updateOrchestraState = `UPDATE orchestras SET state = $2, updated_at = now() WHERE id = $1`

func (q *Queries) UpdateOrchestraState(ctx context.Context, id, state string) error {
	_, err := q.db.Exec(ctx, updateOrchestraState, id, state)
	return err
}

const finishOrchestra = `UPDATE orchestras SET state = 'completed', summary = $2, updated_at = now() WHERE id = $1`

func (q *Queries) FinishOrchestra(ctx context.Context, id string, summary *string) error {
	_, err := q.db.Exec(ctx, finishOrchestra, id, summary)
	return err
}

const incrementMovementCount = `UPDATE orchestras SET movement_count = movement_count + 1, updated_at = now() WHERE id = $1`

func (q *Queries) IncrementMovementCount(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, incrementMovementCount, id)
	return err
}

const listMovementsByOrchestra = `SELECT id, workspace_id, process_id, scheduled_at, state, origin, attempt, max_attempts, orchestra_id, orchestra_step, result, choice_config, chosen_index, started_at, finished_at, duration_ms, exit_code, created_at FROM runs WHERE orchestra_id = $1 ORDER BY orchestra_step`

type MovementRow struct {
	ID            string             `json:"id"`
	WorkspaceID   string             `json:"workspace_id"`
	ProcessID     string             `json:"process_id"`
	ScheduledAt   pgtype.Timestamptz `json:"scheduled_at"`
	State         string             `json:"state"`
	Origin        string             `json:"origin"`
	Attempt       int32              `json:"attempt"`
	MaxAttempts   int32              `json:"max_attempts"`
	OrchestraID   *string            `json:"orchestra_id"`
	OrchestraStep *int32             `json:"orchestra_step"`
	Result        []byte             `json:"result"`
	ChoiceConfig  []byte             `json:"choice_config"`
	ChosenIndex   *int32             `json:"chosen_index"`
	StartedAt     pgtype.Timestamptz `json:"started_at"`
	FinishedAt    pgtype.Timestamptz `json:"finished_at"`
	DurationMs    *int64             `json:"duration_ms"`
	ExitCode      *int32             `json:"exit_code"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
}

func (q *Queries) ListMovementsByOrchestra(ctx context.Context, orchestraID string) ([]MovementRow, error) {
	rows, err := q.db.Query(ctx, listMovementsByOrchestra, orchestraID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MovementRow{}
	for rows.Next() {
		var m MovementRow
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.ProcessID, &m.ScheduledAt, &m.State, &m.Origin, &m.Attempt, &m.MaxAttempts, &m.OrchestraID, &m.OrchestraStep, &m.Result, &m.ChoiceConfig, &m.ChosenIndex, &m.StartedAt, &m.FinishedAt, &m.DurationMs, &m.ExitCode, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

const setRunChoiceConfig = `UPDATE runs SET choice_config = $2, state = 'waiting_for_choice', updated_at = now() WHERE id = $1`

func (q *Queries) SetRunChoiceConfig(ctx context.Context, id string, config []byte) error {
	_, err := q.db.Exec(ctx, setRunChoiceConfig, id, config)
	return err
}

const setRunChosenIndex = `UPDATE runs SET chosen_index = $2, state = 'completed', updated_at = now() WHERE id = $1`

func (q *Queries) SetRunChosenIndex(ctx context.Context, id string, index int32) error {
	_, err := q.db.Exec(ctx, setRunChosenIndex, id, index)
	return err
}
