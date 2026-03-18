package runstate

import "testing"

func TestValidTransitions(t *testing.T) {
	valid := []struct {
		from, to State
	}{
		{Pending, Running},
		{Pending, Queued},
		{Pending, WaitingForWorker},
		{Pending, Skipped},
		{Pending, Cancelled},
		{Pending, Paused},
		{Queued, Running},
		{Queued, Cancelled},
		{Queued, Paused},
		{WaitingForWorker, Running},
		{WaitingForWorker, Cancelled},
		{Running, Completed},
		{Running, Failed},
		{Running, Hung},
		{Running, KillRequested},
		{Running, Retrying},
		{KillRequested, Killed},
		{Retrying, Running},
		{Retrying, Failed},
		{Retrying, Cancelled},
		{Retrying, Killed},
		{Hung, Retrying},
		{Hung, Failed},
		{Paused, Pending},
	}

	for _, tt := range valid {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Errorf("expected valid transition %s→%s: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestInvalidTransitions(t *testing.T) {
	invalid := []struct {
		from, to State
	}{
		{Completed, Running},
		{Failed, Running},
		{Killed, Running},
		{Skipped, Running},
		{Cancelled, Running},
		{Running, Pending},
		{Pending, Completed},
		{Queued, Completed},
		{Paused, Running},
	}

	for _, tt := range invalid {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err == nil {
				t.Errorf("expected invalid transition %s→%s to be rejected", tt.from, tt.to)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []State{Completed, Failed, Killed, Skipped, Cancelled}
	for _, s := range terminal {
		if !IsTerminal(s) {
			t.Errorf("expected %s to be terminal", s)
		}
	}

	nonTerminal := []State{Pending, Running, Retrying, Queued, WaitingForWorker, KillRequested, Hung, Paused}
	for _, s := range nonTerminal {
		if IsTerminal(s) {
			t.Errorf("expected %s to NOT be terminal", s)
		}
	}
}

func TestBlocksOverlap(t *testing.T) {
	blocks := []State{Running, Retrying, KillRequested, Queued}
	for _, s := range blocks {
		if !BlocksOverlap(s) {
			t.Errorf("expected %s to block overlap", s)
		}
	}

	// WaitingForWorker does NOT consume concurrency
	if BlocksOverlap(WaitingForWorker) {
		t.Error("WaitingForWorker should NOT block overlap")
	}
}

func TestContinuesFixedDelayChain(t *testing.T) {
	continues := []State{Completed, Failed, Hung, Killed, Cancelled}
	for _, s := range continues {
		if !ContinuesFixedDelayChain(s) {
			t.Errorf("expected %s to continue fixed-delay chain", s)
		}
	}

	if ContinuesFixedDelayChain(Paused) {
		t.Error("paused should NOT continue fixed-delay chain")
	}
}
