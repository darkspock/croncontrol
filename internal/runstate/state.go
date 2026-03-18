// Package runstate defines the canonical run state machine for CronControl.
//
// Run states and transitions follow docs/product-specification.md.
// 13 states with validated transitions.
package runstate

import "fmt"

// State represents a run's current state.
type State string

const (
	Pending          State = "pending"
	WaitingForWorker State = "waiting_for_worker"
	Queued           State = "queued"
	Running          State = "running"
	Retrying         State = "retrying"
	KillRequested    State = "kill_requested"
	Completed        State = "completed"
	Failed           State = "failed"
	Hung             State = "hung"
	Killed           State = "killed"
	Skipped          State = "skipped"
	Cancelled        State = "cancelled"
	Paused           State = "paused"
)

// AllStates lists all valid states.
var AllStates = []State{
	Pending, WaitingForWorker, Queued, Running, Retrying, KillRequested,
	Completed, Failed, Hung, Killed, Skipped, Cancelled, Paused,
}

// validTransitions maps each state to its allowed next states.
var validTransitions = map[State][]State{
	Pending:          {WaitingForWorker, Queued, Running, Skipped, Cancelled, Paused},
	WaitingForWorker: {Running, Cancelled},
	Queued:           {Running, Cancelled, Paused},
	Running:          {Completed, Failed, Hung, KillRequested, Retrying},
	Retrying:         {Running, Failed, Cancelled, Killed},
	KillRequested:    {Killed},
	Completed:        {},
	Failed:           {},
	Hung:             {Retrying, Failed},
	Killed:           {},
	Skipped:          {},
	Cancelled:        {},
	Paused:           {Pending},
}

// ValidateTransition checks whether a state transition is allowed.
func ValidateTransition(from, to State) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown state %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", from, to)
}

// IsTerminal returns true if the state is a final state (no further transitions except hung→retrying).
func IsTerminal(s State) bool {
	switch s {
	case Completed, Failed, Killed, Skipped, Cancelled:
		return true
	default:
		return false
	}
}

// IsFinalTerminal returns true if the state is terminal AND no retries will happen.
// Used for: fixed-delay chain creation, dependency evaluation.
// Hung is NOT final terminal because it may retry.
func IsFinalTerminal(s State) bool {
	return IsTerminal(s)
}

// IsActive returns true if the run is consuming resources or blocking overlap.
func IsActive(s State) bool {
	switch s {
	case Running, Retrying, KillRequested:
		return true
	default:
		return false
	}
}

// BlocksOverlap returns true if the run should be considered for overlap/parallelism checks.
// WaitingForWorker does NOT consume concurrency per canonical spec.
func BlocksOverlap(s State) bool {
	switch s {
	case Running, Retrying, KillRequested, Queued:
		return true
	default:
		return false
	}
}

// ContinuesFixedDelayChain returns true if this terminal state should create the next
// fixed-delay run. Only `paused` stops the chain.
func ContinuesFixedDelayChain(s State) bool {
	switch s {
	case Completed, Failed, Hung, Killed, Cancelled:
		return true
	case Paused:
		return false
	default:
		return false
	}
}
