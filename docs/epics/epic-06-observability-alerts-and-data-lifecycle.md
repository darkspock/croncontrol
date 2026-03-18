# EPIC-06: Observability, Alerts, and Data Lifecycle

## Outcome

Provide the operational visibility layer: heartbeats, progress, logging, alerts, health checks, and retention policies.

## Why This Epic Exists

CronControl's promise is not only to run work but to explain what happened. That requires strong observability and a clear lifecycle for operational data.

## Goals

- Support heartbeat and progress reporting for long-running work.
- Capture execution output and queue attempt history safely.
- Provide alert webhooks and health endpoints.
- Support multiple logging backends where appropriate.
- Define cleanup and retention behavior.

## In Scope

- Heartbeat API
- Progress model
- Execution output storage
- Attempt storage
- Alert notifier
- Health endpoint
- Retention configuration
- Cleanup jobs
- Log backend abstraction
- Redaction and masking behavior

## Out of Scope

- Full BI or analytics product features
- Complex event-routing products

## Canonical Constraints

- Webhooks use one unified event system
- Deliveries are HMAC-signed and at-least-once
- Subscriptions are disabled after repeated consecutive failures
- Public health is basic and unauthenticated; detailed health is authenticated per workspace
- Retention is simple by plan in the first version

## Acceptance Criteria

- Running work can report liveness and progress without ambiguity.
- Alerts have a stable payload contract and delivery policy.
- Health endpoints expose actionable operational indicators.
- Sensitive data is masked or excluded according to policy.
- Retention and cleanup behavior are visible, configurable, and safe.
- The product can explain both recent failures and historical trends.

## Dependencies

- [EPIC-02 Scheduling and Orchestration Core](epic-02-scheduling-and-orchestration-core.md)
- [EPIC-03 Execution Plane and Worker Runtime](epic-03-execution-plane-and-worker-runtime.md)
- [EPIC-04 Durable Queue and Replay](epic-04-durable-queue-and-replay.md)

## Follow-on Impact

The dashboard, support workflows, billing enforcement visibility, and AI-assisted debugging all depend on this epic.
