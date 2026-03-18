package croncontrol_test

import (
	"context"
	"fmt"

	croncontrol "github.com/croncontrol/croncontrol/sdk/go"
)

func Example() {
	ctx := context.Background()
	cc := croncontrol.New("http://localhost:8080", "cc_live_abc123...")

	// List processes
	processes, _ := cc.ListProcesses(ctx, nil)
	fmt.Printf("Found %d processes\n", len(processes.Data))

	// Trigger a process
	run, _ := cc.TriggerProcess(ctx, "prc_01HYX...")
	fmt.Printf("Run: %s\n", string(run.Data))

	// Enqueue a job
	job, _ := cc.EnqueueJob(ctx, map[string]any{
		"queue_id":  "que_01HYX...",
		"payload":   map[string]any{"to": "user@example.com"},
		"reference": "order-12345",
	})
	fmt.Printf("Job: %s\n", string(job.Data))

	// Report heartbeat
	cc.Heartbeat(ctx, "run_01HYX...", 100, 50, "Halfway")

	// Health check
	health, _ := cc.Health(ctx)
	fmt.Printf("Status: %s\n", health["status"])
}
