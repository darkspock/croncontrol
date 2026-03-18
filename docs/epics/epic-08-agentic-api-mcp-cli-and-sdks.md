# EPIC-08: Agentic API, MCP, CLI, and SDKs

## Outcome

Make CronControl easy to adopt from code, terminals, and AI agents through a stable machine-friendly surface.

## Why This Epic Exists

The product is intentionally API-first. To make that real, the API, MCP server, CLI, and SDKs must all represent the same domain model and error semantics.

## Goals

- Publish a clean OpenAPI contract.
- Provide a stable API surface for all major product actions.
- Offer an MCP server for AI tools.
- Provide a CLI for terminal workflows.
- Ship lightweight SDKs where they reduce adoption friction.

## In Scope

- OpenAPI polish and publication
- Agent-friendly error model
- MCP tool surface
- CLI commands and auth flow
- Lightweight helper SDKs
- Documentation and examples

## Out of Scope

- A fully generated SDK strategy for every language
- Deep workflow composition inside MCP

## Canonical Constraints

- The public API follows the canonical resource model and response envelope
- MCP, CLI, and SDKs use the same public terminology as the dashboard
- Agent-facing webhook events come from the unified event system
- SDKs stay intentionally thin and aligned to the API rather than introducing alternate abstractions

## Acceptance Criteria

- The API exposes the same capabilities as the dashboard for core product flows.
- Error responses are structured and predictable for both humans and agents.
- MCP tools cover the most important operational workflows.
- The CLI is useful for real operator tasks, not only demos.
- SDKs stay thin and align with the canonical API contract.

## Dependencies

- [EPIC-01 Canonical Platform Foundation](epic-01-canonical-platform-foundation.md)
- [EPIC-02 Scheduling and Orchestration Core](epic-02-scheduling-and-orchestration-core.md)
- [EPIC-03 Execution Plane and Worker Runtime](epic-03-execution-plane-and-worker-runtime.md)
- [EPIC-04 Durable Queue and Replay](epic-04-durable-queue-and-replay.md)
- [EPIC-05 Multi-tenant SaaS, Identity, and Billing](epic-05-multi-tenant-saas-identity-and-billing.md)
- [EPIC-06 Observability, Alerts, and Data Lifecycle](epic-06-observability-alerts-and-data-lifecycle.md)
- [EPIC-07 Dashboard and Admin Experience](epic-07-dashboard-and-admin-experience.md)

## Follow-on Impact

This epic determines whether CronControl is merely scriptable or truly agent-ready.
