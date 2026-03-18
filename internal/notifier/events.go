package notifier

// Canonical event types for the webhook system.
const (
	// Run events
	EventRunCompleted = "run.completed"
	EventRunFailed    = "run.failed"
	EventRunHung      = "run.hung"
	EventRunKilled    = "run.killed"

	// Job events
	EventJobCompleted = "job.completed"
	EventJobFailed    = "job.failed"
	EventJobKilled    = "job.killed"

	// Usage events
	EventUsageWarning = "usage.warning"

	// Webhook lifecycle
	EventWebhookDisabled = "webhook.disabled"

	// Worker events
	EventWorkerOffline   = "worker.offline"
	EventWorkerUnhealthy = "worker.unhealthy"

	// Workspace events
	EventWorkspaceRestricted = "workspace.restricted"
	EventWorkspaceArchived   = "workspace.archived"

	// Test
	EventWebhookTest = "webhook.test"
)
