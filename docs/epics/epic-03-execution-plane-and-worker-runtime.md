# EPIC-03: Execution Plane and Worker Runtime

## Outcome

Build the execution layer that can dispatch supported methods and introduce the CronControl Worker as a private runtime and network gateway.

## Why This Epic Exists

Scheduling without execution is not a product. The hosted control plane also needs a safe way to run commands inside customer infrastructure without pretending that arbitrary shell execution is "local".

## Goals

- Support HTTP execution first.
- Support infrastructure methods that fit the product: HTTP, SSH, AWS SSM, and Kubernetes Jobs.
- Define a secure worker registration and authentication model.
- Capture logs, output, heartbeats, and kill behavior across methods.
- Keep a consistent execution contract regardless of method or runtime.

## In Scope

- Execution method interface
- HTTP target
- SSH target
- AWS SSM target
- Kubernetes Job target
- CronControl Worker runtime
- Worker enrollment and identity
- Worker task dispatch and status reporting
- Per-method and per-worker concurrency controls

## Out of Scope

- Rich billing behavior
- Dashboard feature completeness
- Queue semantics beyond shared execution primitives

## Worker Product Model

The worker is a lightweight runtime installed on a customer-owned host.

- It authenticates to exactly one workspace.
- It advertises host identity, labels, and capabilities.
- It keeps an outbound connection pattern to the CronControl control plane.
- It receives work for supported execution methods when a process or queue uses `runtime = worker`.
- It acts as a network-local execution gateway for private HTTP, SSH, SSM, or Kubernetes access.
- It reports status, heartbeats, logs, and final result back to CronControl.

The worker is not a separate execution method. It is the runtime used when the control plane must execute from inside the customer's network.

## Canonical Constraints

- Worker communication is outbound only.
- The first implementation uses long polling.
- A worker belongs to one workspace only.
- Worker credentials are separate from normal API keys.
- Workers declare capabilities automatically; labels are admin-managed.
- Selection supports explicit `worker_id` and label matching, with automatic least-loaded routing when no selector is provided.
- `direct` and `worker` runtimes are available for all execution methods, subject to plan and connectivity rules.

## Acceptance Criteria

- HTTP execution is production-ready for the first hosted release.
- Worker-routed execution is available as a first-class runtime option.
- Supported methods share one execution result model.
- Output, heartbeats, and kill behavior are captured consistently.
- The product does not expose direct "local execution" as a concept.
- Worker connectivity, authentication, and failure modes are documented.

## Dependencies

- [EPIC-01 Canonical Platform Foundation](epic-01-canonical-platform-foundation.md)
- [EPIC-02 Scheduling and Orchestration Core](epic-02-scheduling-and-orchestration-core.md)

## Follow-on Impact

The durable queue, monitoring, billing limits, and dashboard all depend on this epic because they consume target execution behavior.
