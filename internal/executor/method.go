// Package executor defines the execution method interface and orchestrates run dispatch.
package executor

import (
	"context"
	"fmt"
	"time"
)

// Method is the async-capable contract for all execution methods.
type Method interface {
	// Start dispatches work. Blocking methods may return a terminal Result immediately.
	Start(ctx context.Context, params StartParams) (StartResult, error)

	// Poll checks an async execution. Blocking methods can return ErrPollUnsupported.
	Poll(ctx context.Context, handle Handle, cursor PollCursor) (PollResult, error)

	// Kill attempts to stop a running execution.
	Kill(ctx context.Context, handle Handle) error

	// SupportsKill returns true if this method can kill running work.
	SupportsKill() bool

	// SupportsHeartbeat returns true if heartbeat/progress applies.
	SupportsHeartbeat() bool

	// IsAsync returns true when the method requires follow-up polling.
	IsAsync() bool
}

// BlockingMethod is the legacy blocking executor contract.
type BlockingMethod interface {
	Execute(ctx context.Context, params ExecuteParams) (Result, error)
	Kill(ctx context.Context, handle Handle) error
	SupportsKill() bool
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

// StartParams contains everything needed to start an execution.
type StartParams struct {
	RunID        string
	WorkspaceID  string
	MethodConfig map[string]any
	Environment  map[string]string
	APIBaseURL   string
	Timeout      time.Duration
}

// StartResult captures the outcome of a Start call.
type StartResult struct {
	Handle     Handle
	AcceptedAt time.Time

	// Result is non-nil for blocking methods that complete inside Start.
	Result *Result
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

// PollCursor tracks incremental poll progress for async executions.
type PollCursor struct {
	StdoutOffset int64
	StderrOffset int64
}

// RemoteState describes the remote execution state returned by Poll.
type RemoteState string

const (
	RemoteRunning   RemoteState = "running"
	RemoteCompleted RemoteState = "completed"
	RemoteFailed    RemoteState = "failed"
	RemoteKilled    RemoteState = "killed"
)

// PollResult captures the state returned by Poll on async executions.
type PollResult struct {
	State             RemoteState
	ExitCode          *int
	StdoutChunk       string
	StderrChunk       string
	ResponseBodyChunk string
	Cursor            PollCursor
	Error             error
}

// ErrPollUnsupported indicates that a method does not support polling because it is blocking.
var ErrPollUnsupported = fmt.Errorf("executor: poll unsupported for blocking method")

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

// RegisterBlocking adapts and registers a blocking method under the async contract.
func (r *Registry) RegisterBlocking(name string, m BlockingMethod) {
	r.methods[name] = AdaptBlocking(name, m)
}

// Get returns a method by name.
func (r *Registry) Get(name string) (Method, bool) {
	m, ok := r.methods[name]
	return m, ok
}

type blockingAdapter struct {
	methodName string
	method     BlockingMethod
}

// AdaptBlocking wraps a blocking method in the async-capable Method interface.
func AdaptBlocking(name string, m BlockingMethod) Method {
	return &blockingAdapter{methodName: name, method: m}
}

func (a *blockingAdapter) Start(ctx context.Context, params StartParams) (StartResult, error) {
	result, err := a.method.Execute(ctx, ExecuteParams{
		RunID:        params.RunID,
		WorkspaceID:  params.WorkspaceID,
		MethodConfig: params.MethodConfig,
		Environment:  params.Environment,
		APIBaseURL:   params.APIBaseURL,
	})

	handle := Handle{
		MethodName: a.methodName,
		RunID:      params.RunID,
		Data:       make(map[string]any),
	}

	return StartResult{
		Handle:     handle,
		AcceptedAt: time.Now().UTC(),
		Result:     &result,
	}, err
}

func (a *blockingAdapter) Poll(context.Context, Handle, PollCursor) (PollResult, error) {
	return PollResult{}, ErrPollUnsupported
}

func (a *blockingAdapter) Kill(ctx context.Context, handle Handle) error {
	return a.method.Kill(ctx, handle)
}

func (a *blockingAdapter) SupportsKill() bool {
	return a.method.SupportsKill()
}

func (a *blockingAdapter) SupportsHeartbeat() bool {
	return a.method.SupportsHeartbeat()
}

func (a *blockingAdapter) IsAsync() bool {
	return false
}

// ParseDuration parses a duration string like "5m", "1h", "30s" into time.Duration.
func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
