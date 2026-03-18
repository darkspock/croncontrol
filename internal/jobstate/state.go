// Package jobstate defines the canonical job state machine for CronControl queues.
//
// Job states follow docs/product-specification.md.
// 9 states with validated transitions.
package jobstate

import "fmt"

type State string

const (
	Pending          State = "pending"
	WaitingForWorker State = "waiting_for_worker"
	Running          State = "running"
	Retrying         State = "retrying"
	KillRequested    State = "kill_requested"
	Completed        State = "completed"
	Failed           State = "failed"
	Killed           State = "killed"
	Cancelled        State = "cancelled"
)

var validTransitions = map[State][]State{
	Pending:          {WaitingForWorker, Running, Cancelled},
	WaitingForWorker: {Running, Cancelled},
	Running:          {Completed, Failed, Retrying, KillRequested},
	Retrying:         {Running, Failed, Cancelled, Killed},
	KillRequested:    {Killed},
	Completed:        {},
	Failed:           {},
	Killed:           {},
	Cancelled:        {},
}

func ValidateTransition(from, to State) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown job state %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid job transition from %q to %q", from, to)
}

func IsTerminal(s State) bool {
	switch s {
	case Completed, Failed, Killed, Cancelled:
		return true
	default:
		return false
	}
}

func IsReplayable(s State) bool {
	return IsTerminal(s)
}
