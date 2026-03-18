// Package mcp implements a Model Context Protocol server for CronControl.
//
// This allows AI agents (Claude, GPT, etc.) to interact with CronControl
// as a tool — creating processes, triggering runs, inspecting jobs, etc.
//
// The MCP server communicates via stdin/stdout using JSON-RPC 2.0.
// Configuration: CRONCONTROL_URL + CRONCONTROL_API_KEY environment variables.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Server is the MCP server that bridges AI agents to CronControl API.
type Server struct {
	client *APIClient
	tools  map[string]Tool
}

// Tool represents an MCP tool that agents can invoke.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Handler     func(ctx context.Context, input map[string]any) (any, error)
}

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates a new MCP server.
func NewServer(baseURL, apiKey string) *Server {
	client := NewAPIClient(baseURL, apiKey)
	s := &Server{
		client: client,
		tools:  make(map[string]Tool),
	}
	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	s.tools["list_processes"] = Tool{
		Name:        "list_processes",
		Description: "List all configured processes in the workspace. Returns name, schedule type, execution method, state, and tags.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			return s.client.Get(ctx, "/processes")
		},
	}

	s.tools["create_process"] = Tool{
		Name:        "create_process",
		Description: "Create a new scheduled process. Requires name, schedule_type (cron/fixed_delay/on_demand), and execution_method (http/ssh/ssm/k8s).",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"name", "schedule_type", "execution_method"},
			"properties": map[string]any{
				"name":             map[string]any{"type": "string", "description": "Unique process name"},
				"schedule_type":    map[string]any{"type": "string", "enum": []string{"cron", "fixed_delay", "on_demand"}},
				"schedule":         map[string]any{"type": "string", "description": "Cron expression (for cron type)"},
				"delay_duration":   map[string]any{"type": "string", "description": "Delay duration like 5m, 1h (for fixed_delay type)"},
				"execution_method": map[string]any{"type": "string", "enum": []string{"http", "ssh", "ssm", "k8s"}},
				"method_config":    map[string]any{"type": "object", "description": "Method-specific config (url, headers, command, etc.)"},
				"enabled":          map[string]any{"type": "boolean", "default": true},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			return s.client.Post(ctx, "/processes", input)
		},
	}

	s.tools["trigger_process"] = Tool{
		Name:        "trigger_process",
		Description: "Manually trigger a process to create an immediate run.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"process_id"},
			"properties": map[string]any{
				"process_id": map[string]any{"type": "string", "description": "Process ID (prc_...)"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["process_id"].(string)
			return s.client.Post(ctx, fmt.Sprintf("/processes/%s/trigger", id), nil)
		},
	}

	s.tools["pause_process"] = Tool{
		Name:        "pause_process",
		Description: "Pause scheduling for a process. Pending runs can optionally be cancelled.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"process_id"},
			"properties": map[string]any{
				"process_id":     map[string]any{"type": "string"},
				"cancel_pending": map[string]any{"type": "boolean", "default": false},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["process_id"].(string)
			cancel := ""
			if cp, ok := input["cancel_pending"].(bool); ok && cp {
				cancel = "?cancel_pending=true"
			}
			return s.client.Post(ctx, fmt.Sprintf("/processes/%s/pause%s", id, cancel), nil)
		},
	}

	s.tools["resume_process"] = Tool{
		Name:        "resume_process",
		Description: "Resume scheduling for a paused process.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"process_id"},
			"properties": map[string]any{
				"process_id": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["process_id"].(string)
			return s.client.Post(ctx, fmt.Sprintf("/processes/%s/resume", id), nil)
		},
	}

	s.tools["list_runs"] = Tool{
		Name:        "list_runs",
		Description: "List recent runs. Can filter by process_id, state, or origin.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"process_id": map[string]any{"type": "string", "description": "Filter by process"},
				"state":      map[string]any{"type": "string", "description": "Filter by state (pending, running, completed, failed, hung, etc.)"},
				"limit":      map[string]any{"type": "integer", "default": 10},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			var params []string
			if pid, ok := input["process_id"].(string); ok {
				params = append(params, "process_id="+pid)
			}
			if state, ok := input["state"].(string); ok {
				params = append(params, "state="+state)
			}
			query := ""
			if len(params) > 0 {
				query = "?" + strings.Join(params, "&")
			}
			return s.client.Get(ctx, "/runs"+query)
		},
	}

	s.tools["get_run"] = Tool{
		Name:        "get_run",
		Description: "Get detailed information about a specific run, including attempts and progress.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"run_id"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Run ID (run_...)"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["run_id"].(string)
			return s.client.Get(ctx, fmt.Sprintf("/runs/%s", id))
		},
	}

	s.tools["kill_run"] = Tool{
		Name:        "kill_run",
		Description: "Request to kill a running execution.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"run_id"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["run_id"].(string)
			return s.client.Post(ctx, fmt.Sprintf("/runs/%s/kill", id), nil)
		},
	}

	s.tools["enqueue_job"] = Tool{
		Name:        "enqueue_job",
		Description: "Enqueue a job into a queue for processing.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"queue_id", "payload"},
			"properties": map[string]any{
				"queue_id":  map[string]any{"type": "string", "description": "Queue ID (que_...)"},
				"payload":   map[string]any{"type": "object", "description": "Job payload data"},
				"priority":  map[string]any{"type": "integer", "default": 0},
				"reference": map[string]any{"type": "string", "description": "External reference (e.g., order ID)"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			return s.client.Post(ctx, "/jobs", input)
		},
	}

	s.tools["get_job"] = Tool{
		Name:        "get_job",
		Description: "Get detailed information about a job including attempt history.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"job_id"},
			"properties": map[string]any{
				"job_id": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["job_id"].(string)
			return s.client.Get(ctx, fmt.Sprintf("/jobs/%s", id))
		},
	}

	s.tools["replay_job"] = Tool{
		Name:        "replay_job",
		Description: "Replay a failed or terminal job. Optionally override payload or priority.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"job_id"},
			"properties": map[string]any{
				"job_id":   map[string]any{"type": "string"},
				"payload":  map[string]any{"type": "object", "description": "Override payload"},
				"priority": map[string]any{"type": "integer", "description": "Override priority"},
			},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			id := input["job_id"].(string)
			overrides := map[string]any{}
			if p, ok := input["payload"]; ok {
				overrides["payload"] = p
			}
			if p, ok := input["priority"]; ok {
				overrides["priority"] = p
			}
			return s.client.Post(ctx, fmt.Sprintf("/jobs/%s/replay", id), overrides)
		},
	}

	s.tools["get_health"] = Tool{
		Name:        "get_health",
		Description: "Get system health status including component status, run counts, and queue depths.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, input map[string]any) (any, error) {
			return s.client.GetRaw(ctx, "/workspace/health")
		},
	}
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *Server) Run() error {
	slog.Info("mcp server starting")
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stdin: %w", err)
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			encoder.Encode(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &Error{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := s.handleRequest(req)
		encoder.Encode(resp)
	}
}

func (s *Server) handleRequest(req JSONRPCRequest) JSONRPCResponse {
	ctx := context.Background()

	switch req.Method {
	case "initialize":
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "croncontrol",
					"version": "1.0.0",
				},
			},
		}

	case "tools/list":
		var toolList []map[string]any
		for _, t := range s.tools {
			toolList = append(toolList, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": toolList},
		}

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: -32602, Message: "invalid params"}}
		}

		tool, ok := s.tools[params.Name]
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", params.Name)}}
		}

		result, err := tool.Handler(ctx, params.Arguments)
		if err != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": fmt.Sprintf("Error: %s", err.Error())},
					},
					"isError": true,
				},
			}
		}

		text, _ := json.MarshalIndent(result, "", "  ")
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}

	case "notifications/initialized":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: -32601, Message: "method not found"}}
	}
}
