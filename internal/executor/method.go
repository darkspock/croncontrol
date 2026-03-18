// Package executor defines the execution method interface and orchestrates run dispatch.
package executor

import (
	"context"
	"time"
)

// Method is the interface that all execution methods must implement.
type Method interface {
	// Execute dispatches work and blocks until completion.
	Execute(ctx context.Context, params ExecuteParams) (Result, error)

	// Kill attempts to stop a running execution.
	Kill(ctx context.Context, handle Handle) error

	// SupportsKill returns true if this method can kill running work.
	SupportsKill() bool

	// SupportsHeartbeat returns true if heartbeat/progress applies.
	SupportsHeartbeat() bool
}

// ExecuteParams contains everything needed to dispatch work.
type ExecuteParams struct {
	RunID        string
	WorkspaceID  string
	MethodConfig map[string]any
	Environment  map[string]string
	APIBaseURL   string
}

// Result captures the outcome of an execution.
type Result struct {
	ExitCode     *int
	ResponseCode *int
	Stdout       string
	Stderr       string
	ResponseBody string
	DurationMs   int64
	Error        error
}

// Handle contains method-specific data needed to kill a running execution.
type Handle struct {
	MethodName string
	RunID      string
	Data       map[string]any // method-specific (PID, command ID, job name, etc.)
}

// IsSuccess returns true if the execution succeeded (exit code 0 or HTTP 2xx).
func (r Result) IsSuccess() bool {
	if r.ExitCode != nil {
		return *r.ExitCode == 0
	}
	if r.ResponseCode != nil {
		return *r.ResponseCode >= 200 && *r.ResponseCode < 300
	}
	return r.Error == nil
}

// Registry maps method names to implementations.
type Registry struct {
	methods map[string]Method
}

// NewRegistry creates a new method registry.
func NewRegistry() *Registry {
	return &Registry{methods: make(map[string]Method)}
}

// Register adds a method implementation.
func (r *Registry) Register(name string, m Method) {
	r.methods[name] = m
}

// Get returns a method by name.
func (r *Registry) Get(name string) (Method, bool) {
	m, ok := r.methods[name]
	return m, ok
}

// ParseDuration parses a duration string like "5m", "1h", "30s" into time.Duration.
func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
