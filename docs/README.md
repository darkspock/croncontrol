# Documentation

This directory is the canonical product documentation set for CronControl.

Legacy research notes and draft requirements still exist at the repository root. They remain useful as source material, but `docs/` wins whenever there is a conflict.

## Core Documents

- [Business Overview](business-overview.md)
- [Product Specification](product-specification.md)
- [Roadmap](roadmap.md)
- [Glossary](glossary.md)

## Epic Documents

- [EPIC-01 Canonical Platform Foundation](epics/epic-01-canonical-platform-foundation.md)
- [EPIC-02 Scheduling and Orchestration Core](epics/epic-02-scheduling-and-orchestration-core.md)
- [EPIC-03 Execution Plane and Worker Runtime](epics/epic-03-execution-plane-and-worker-runtime.md)
- [EPIC-04 Durable Queue and Replay](epics/epic-04-durable-queue-and-replay.md)
- [EPIC-05 Multi-workspace Identity and Access](epics/epic-05-multi-tenant-saas-identity-and-billing.md)
- [EPIC-06 Observability, Alerts, and Data Lifecycle](epics/epic-06-observability-alerts-and-data-lifecycle.md)
- [EPIC-07 Dashboard and Admin Experience](epics/epic-07-dashboard-and-admin-experience.md)
- [EPIC-08 Agentic API, MCP, CLI, and SDKs](epics/epic-08-agentic-api-mcp-cli-and-sdks.md)

## Documentation Rules

- Product and roadmap documents are written in English.
- Use business language first, then implementation details.
- `workspace`, `process`, `run`, and `queue` are public product terms.
- `tenant` and `scheduled_slot` are internal implementation terms only.
- CronControl is open source (MIT license).
- `worker` is an execution runtime and network gateway, not a first-class execution method.
- Decisions that change product language or core contracts must be reflected in `docs/` first.
