package jobstate

import "testing"

func TestValidTransitions(t *testing.T) {
	valid := []struct{ from, to State }{
		{Pending, Running},
		{Pending, WaitingForWorker},
		{Pending, Cancelled},
		{WaitingForWorker, Running},
		{Running, Completed},
		{Running, Failed},
		{Running, Retrying},
		{Running, KillRequested},
		{KillRequested, Killed},
		{Retrying, Running},
		{Retrying, Failed},
		{Retrying, Cancelled},
		{Retrying, Killed},
	}
	for _, tt := range valid {
		if err := ValidateTransition(tt.from, tt.to); err != nil {
			t.Errorf("expected valid: %s→%s: %v", tt.from, tt.to, err)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	invalid := []struct{ from, to State }{
		{Completed, Running},
		{Failed, Running},
		{Killed, Running},
		{Cancelled, Running},
		{Running, Pending},
		{Pending, Completed},
	}
	for _, tt := range invalid {
		if err := ValidateTransition(tt.from, tt.to); err == nil {
			t.Errorf("expected invalid: %s→%s", tt.from, tt.to)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	for _, s := range []State{Completed, Failed, Killed, Cancelled} {
		if !IsTerminal(s) {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range []State{Pending, Running, Retrying, WaitingForWorker, KillRequested} {
		if IsTerminal(s) {
			t.Errorf("%s should NOT be terminal", s)
		}
	}
}
