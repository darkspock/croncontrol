package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/runstate"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DirectorTools are the tools available to the AI Director.
var DirectorTools = []Tool{
	{
		Name:        "next_movement",
		Description: "Trigger the next musician process in the orchestra",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"process_id": map[string]any{"type": "string", "description": "ID or name of the process to run next"},
				"message":    map[string]any{"type": "string", "description": "Why this musician was chosen"},
			},
			"required": []string{"process_id"},
		},
	},
	{
		Name:        "ask_choice",
		Description: "Present choices to a human operator. The orchestra pauses until they decide.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "Question or context for the human"},
				"choices": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"label":      map[string]any{"type": "string"},
							"process_id": map[string]any{"type": "string", "description": "Process to trigger if chosen, or null to end"},
							"style":      map[string]any{"type": "string", "enum": []string{"primary", "danger", "default"}},
						},
					},
				},
			},
			"required": []string{"message", "choices"},
		},
	},
	{
		Name:        "post_chat",
		Description: "Send a message to the orchestra chat visible to all participants",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "Message to post"},
			},
			"required": []string{"message"},
		},
	},
	{
		Name:        "finish_orchestra",
		Description: "End the orchestra with a summary. No more movements will be triggered.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{"type": "string", "description": "Summary of what the orchestra accomplished"},
			},
			"required": []string{"summary"},
		},
	},
	{
		Name:        "list_musicians",
		Description: "List all available processes (musicians) in the workspace",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	},
}

// AIDirector coordinates an orchestra using an LLM provider.
type AIDirector struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	provider Provider
}

// NewAIDirector creates a new AI Director.
func NewAIDirector(pool *pgxpool.Pool, provider Provider) *AIDirector {
	return &AIDirector{pool: pool, queries: db.New(pool), provider: provider}
}

// Event represents what just happened in the orchestra.
type Event struct {
	Type        string `json:"type"`         // movement_completed, movement_failed, choice_made
	OrchestraID string `json:"orchestra_id"`
	RunID       string `json:"run_id"`
	ProcessName string `json:"process_name"`
	Step        int    `json:"step"`
	Result      any    `json:"result"`
}

// HandleEvent processes an orchestra event and decides the next action.
func (d *AIDirector) HandleEvent(ctx context.Context, orch db.Orchestra, event Event) error {
	log := slog.With("orchestra", orch.Name, "provider", d.provider.Name())

	// Build context for the AI
	movements, _ := d.queries.ListMovementsByOrchestra(ctx, &orch.ID)
	processes, _ := d.queries.ListProcesses(ctx, db.ListProcessesParams{WorkspaceID: orch.WorkspaceID, Limit: 100, Offset: 0})
	chat, _ := d.queries.ListChatMessagesAll(ctx, db.ListChatMessagesAllParams{
		OrchestraID: orch.ID,
		Limit:       50,
		Offset:      0,
	})

	scoreJSON, _ := json.Marshal(map[string]any{
		"orchestra":   orch.Name,
		"state":       orch.State,
		"movements":   movements,
		"event":       event,
		"chat_recent": chat,
	})

	var processNames []string
	for _, p := range processes {
		processNames = append(processNames, fmt.Sprintf("%s (%s, %s)", p.Name, p.ExecutionMethod, p.ScheduleType))
	}

	systemPrompt := fmt.Sprintf(`You are the AI Director of the "%s" orchestra in CronControl.
Your job is to coordinate musicians (processes) to achieve the orchestra's goal.

Available musicians:
%s

Based on the score (history) and the latest event, decide what to do next.
Use the tools provided. Be decisive and concise.`, orch.Name, fmt.Sprintf("%v", processNames))

	messages := []Message{{
		Role:    "user",
		Content: string(scoreJSON),
	}}

	// Call LLM with retry
	var response *ProviderResponse
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		response, err = d.provider.Call(ctx, systemPrompt, messages, DirectorTools)
		if err == nil {
			break
		}
		log.Warn("ai director: LLM call failed", "attempt", attempt, "error", err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}

	if err != nil {
		// Fallback: ask human
		log.Error("ai director: all retries failed, falling back to human")
		d.postChat(ctx, orch.ID, "AI Director unavailable after 3 attempts. Please decide manually.", "warning")
		d.fallbackToHuman(ctx, orch, event)
		return err
	}

	// Track AI call in budget
	d.incrementAICalls(ctx, orch.ID)

	// Execute tool calls
	for _, tc := range response.ToolCalls {
		log.Info("ai director: executing tool", "tool", tc.Name)
		if err := d.executeTool(ctx, orch, event, tc); err != nil {
			log.Error("ai director: tool execution failed", "tool", tc.Name, "error", err)
		}
	}

	// If AI returned text but no tools, post it as chat
	if len(response.ToolCalls) == 0 && response.Text != "" {
		d.postChat(ctx, orch.ID, response.Text, "text")
	}

	return nil
}

func (d *AIDirector) executeTool(ctx context.Context, orch db.Orchestra, event Event, tc ToolCall) error {
	switch tc.Name {
	case "next_movement":
		processID, _ := tc.Input["process_id"].(string)
		message, _ := tc.Input["message"].(string)
		if message != "" {
			d.postChat(ctx, orch.ID, message, "text")
		}
		return d.triggerMovement(ctx, orch, event.RunID, processID)

	case "ask_choice":
		message, _ := tc.Input["message"].(string)
		choices, _ := json.Marshal(tc.Input["choices"])
		config, _ := json.Marshal(map[string]any{"message": message, "choices": json.RawMessage(choices)})
		d.queries.SetRunChoiceConfig(ctx, db.SetRunChoiceConfigParams{ID: event.RunID, ChoiceConfig: config})
		d.queries.UpdateOrchestraState(ctx, db.UpdateOrchestraStateParams{ID: orch.ID, State: "waiting_for_choice"})
		d.postChat(ctx, orch.ID, message, "choice")
		return nil

	case "post_chat":
		message, _ := tc.Input["message"].(string)
		d.postChat(ctx, orch.ID, message, "text")
		return nil

	case "finish_orchestra":
		summary, _ := tc.Input["summary"].(string)
		d.queries.FinishOrchestra(ctx, db.FinishOrchestraParams{ID: orch.ID, Summary: &summary})
		d.postChat(ctx, orch.ID, "Orchestra finished: "+summary, "status")
		return nil

	case "list_musicians":
		processes, _ := d.queries.ListProcesses(ctx, db.ListProcessesParams{WorkspaceID: orch.WorkspaceID, Limit: 100, Offset: 0})
		list, _ := json.Marshal(processes)
		d.postChat(ctx, orch.ID, "Available musicians: "+string(list), "text")
		return nil

	default:
		return fmt.Errorf("unknown tool: %s", tc.Name)
	}
}

func (d *AIDirector) triggerMovement(ctx context.Context, orch db.Orchestra, triggeredByRunID, processID string) error {
	step := orch.MovementCount + 1
	d.queries.IncrementMovementCount(ctx, orch.ID)

	systemType := "system"
	newRun, err := d.queries.CreateRun(ctx, db.CreateRunParams{
		ID:               id.NewRun(),
		WorkspaceID:      orch.WorkspaceID,
		ProcessID:        processID,
		ScheduledAt:      dbutil.Timestamptz(time.Now().UTC()),
		State:            string(runstate.Pending),
		Origin:           "orchestra",
		MaxAttempts:      1,
		ActorType:        &systemType,
		TriggeredByRunID: &triggeredByRunID,
	})
	if err != nil {
		return err
	}

	d.pool.Exec(ctx, "UPDATE runs SET orchestra_id=$1, orchestra_step=$2 WHERE id=$3", orch.ID, step, newRun.ID)
	d.postChat(ctx, orch.ID, fmt.Sprintf("Triggered movement %d: %s", step, processID), "status")
	return nil
}

func (d *AIDirector) fallbackToHuman(ctx context.Context, orch db.Orchestra, event Event) {
	processes, _ := d.queries.ListProcesses(ctx, db.ListProcessesParams{WorkspaceID: orch.WorkspaceID, Limit: 100, Offset: 0})
	var choices []map[string]any
	for _, p := range processes {
		if p.Enabled {
			choices = append(choices, map[string]any{"label": p.Name, "process_id": p.ID})
		}
	}
	choices = append(choices, map[string]any{"label": "Cancel orchestra", "process_id": nil, "style": "danger"})

	config, _ := json.Marshal(map[string]any{
		"message": "AI Director is unavailable. Please choose the next action.",
		"choices": choices,
	})
	d.queries.SetRunChoiceConfig(ctx, db.SetRunChoiceConfigParams{ID: event.RunID, ChoiceConfig: config})
	d.queries.UpdateOrchestraState(ctx, db.UpdateOrchestraStateParams{ID: orch.ID, State: "waiting_for_choice"})
}

func (d *AIDirector) postChat(ctx context.Context, orchestraID, content, msgType string) {
	directorType := "director"
	d.queries.CreateChatMessage(ctx, db.CreateChatMessageParams{
		ID:          id.New("msg_"),
		OrchestraID: orchestraID,
		SenderType:  directorType,
		MessageType: msgType,
		Content:     content,
	})
}

func (d *AIDirector) incrementAICalls(ctx context.Context, orchestraID string) {
	d.pool.Exec(ctx,
		"UPDATE orchestras SET budget_used = jsonb_set(COALESCE(budget_used, '{}'), '{ai_calls}', to_jsonb(COALESCE((budget_used->>'ai_calls')::int, 0) + 1)) WHERE id = $1",
		orchestraID)
}
