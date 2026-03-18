# EPIC-04: Durable Queue and Replay

## Outcome

Deliver the queue module for event-driven workloads with retries, replay, attempt history, and durable traceability.

## Why This Epic Exists

Teams need more than scheduling. They also need a durable queue that preserves request and response history, supports replay, and does not lose operational context after dispatch.

## Goals

- Provide a database-backed queue with explicit job states.
- Support delayed jobs, retries, expiration, replay, and batch enqueue.
- Record every attempt with full operational context.
- Share execution primitives with the main scheduler.
- Make idempotency a first-class behavior.

## In Scope

- Queue configuration
- Job intake
- Batch enqueue
- Retry with backoff
- Replay and replay lineage
- Attempt history
- Expiration handling
- Queue-level concurrency
- Idempotency contract

## Out of Scope

- Billing implementation details
- Advanced worker fleet scheduling
- Non-core integrations

## Canonical Constraints

- Job idempotency is workspace-scoped
- Duplicate idempotency returns `409 Conflict` with `existing_job_id`
- Batch enqueue is atomic
- Replay is only allowed for terminal jobs
- Replay starts from the original effective snapshot and allows controlled overrides
- Attempt history stores redacted snapshots rather than raw sensitive values

## Acceptance Criteria

- Jobs move through a clear durable lifecycle.
- Retry behavior is deterministic and configurable.
- Failed jobs can be replayed with lineage back to the original job.
- Attempt history is queryable and safe to expose in product surfaces.
- Queue processing respects concurrency and plan constraints.
- The idempotency model is explicit and consistent with workspace isolation.

## Dependencies

- [EPIC-01 Canonical Platform Foundation](epic-01-canonical-platform-foundation.md)
- [EPIC-03 Execution Plane and Worker Runtime](epic-03-execution-plane-and-worker-runtime.md)

## Follow-on Impact

Queue observability, dashboard detail views, billing limits, and agentic replay tools all build directly on this epic.
