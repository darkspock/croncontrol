# EPIC-02: Scheduling and Orchestration Core

## Outcome

Deliver the durable scheduling engine that plans future work, recovers missed work, manages overlap, and triggers dependent processes.

## Why This Epic Exists

CronControl only becomes valuable when scheduling is explicit, inspectable, and recoverable. This epic builds the product's operational core.

## Goals

- Support `cron`, `fixed_delay`, `on_demand`, and one-time execution models.
- Materialize future work as durable internal scheduled slots and public runs.
- Recover missed runs safely after downtime.
- Enforce overlap and parallelism rules.
- Trigger dependent processes after upstream completion.
- Make scheduling behavior visible and testable.

## In Scope

- Planner
- Scheduled slot lifecycle
- Recovery behavior
- Parallelism and overlap rules
- Dependency triggering
- Slot cancellation, pause, and resume semantics
- Timezone handling
- DST behavior definition

## Out of Scope

- Execution target internals
- Billing and identity
- Dashboard polish beyond what is required for operability

## Canonical Constraints

- Public schedule types are `cron`, `fixed_delay`, and `on_demand`
- Run origins are separate from schedule types
- DST rules are fixed: missing local times skip, duplicated times execute once on the first occurrence
- `cancelled` does not stop a fixed-delay chain; `paused` does
- Retrying runs remain logically active for overlap and parallelism rules
- Dependency checks use the final terminal outcome of the upstream run

## Acceptance Criteria

- Cron schedules plan future slots idempotently.
- Interval schedules create the next slot from terminal execution state.
- Missed-run recovery has explicit caps and deterministic behavior.
- Dependencies are validated for cycles and execute predictably.
- Slot lifecycle states and transitions are unambiguous.
- Timezone and DST behavior are documented and covered by tests.

## Dependencies

- [EPIC-01 Canonical Platform Foundation](epic-01-canonical-platform-foundation.md)

## Follow-on Impact

Execution, alerting, dashboard views, and agentic tooling all rely on the scheduling engine being stable and queryable.
