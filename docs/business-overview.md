# CronControl Business Overview

## Summary

CronControl is a control plane for scheduled and event-driven operational workloads. It helps teams run recurring jobs, trigger infrastructure actions, process durable background work, and inspect every execution attempt with clear state, logs, heartbeats, and replay support.

The product is built for operators, small engineering teams, and AI-assisted builders who need something simpler than a full workflow platform but more reliable than plain cron plus ad hoc scripts.

## The Problem

Most teams start with a mix of cron, queue workers, shell scripts, and webhook calls spread across servers and services. That setup usually breaks down in the same ways:

- Scheduled jobs are hard to audit.
- Missed runs are not recovered consistently.
- Background queues lose traceability after dispatch.
- Long-running tasks have no heartbeat or progress model.
- Infrastructure actions are tied to a single environment or tool.
- AI agents and automation tools have no safe, structured API surface to work with.

CronControl exists to make those operational workflows durable, observable, and programmable.

## Who It Serves

CronControl is designed for:

- SaaS companies that need reliable operational automation.
- Agencies and small product teams managing recurring backend work.
- DevOps and platform teams that want a lighter alternative to workflow engines.
- AI-first builders who want an API they can safely call from agents, MCP clients, and internal tooling.

It is not intended to be a visual no-code workflow builder or a replacement for large-scale data pipeline systems.

## Product Shape

CronControl combines five product layers:

1. Scheduling
   - Cron schedules
   - Interval schedules
   - Manual and one-time runs
   - Missed-run recovery
   - Dependency-aware triggering

2. Execution
   - HTTP
   - SSH
   - AWS SSM
   - Kubernetes Jobs
   - Direct or worker-based runtime selection

3. Durable queue
   - Database-backed jobs
   - Retry with backoff
   - Replay
   - Full attempt history
   - Idempotency support

4. Observability
   - State transitions
   - Heartbeats and progress
   - Output and response capture
   - Alert webhooks
   - Health views and cleanup lifecycle

5. Agentic surface
   - API-first product model
   - OpenAPI contract
   - MCP server
   - CLI and lightweight SDKs

## Control Plane and Execution Plane

CronControl should be understood as two connected planes:

- Control plane: the hosted CronControl application that stores configuration, plans work, enforces limits, and exposes the API and dashboard.
- Execution plane: the systems where work actually runs, such as external HTTP services, SSH hosts, AWS-managed instances, Kubernetes clusters, or CronControl Workers.

This separation is central to the product. It keeps the SaaS platform multi-tenant and safe while still allowing execution inside customer infrastructure.

## Why the Worker Exists

`Local execution` is not part of the canonical product model. A hosted SaaS control plane cannot safely execute arbitrary customer workloads on its own hosts.

CronControl therefore uses a `worker` model:

- A CronControl Worker runs on a customer-owned server or VM.
- The worker authenticates to exactly one workspace.
- The worker acts as an execution runtime and network gateway inside the customer's environment.
- CronControl can route supported execution methods through that worker when private connectivity is required.
- Logs, heartbeats, status, and kill signals flow back through the control plane.

The worker is not a separate execution method. It is the way CronControl reaches private or internal targets when direct SaaS connectivity is not appropriate.

## Delivery Model

CronControl is open-source software (MIT license).

- Multi-workspace by design
- Workloads execute directly from the control plane or through workspace-owned workers
- Self-hosted: deploy anywhere with Go + PostgreSQL

## Business Value

CronControl creates value in four ways:

- Reliability: planned runs, recovery, retries, and stateful execution history.
- Operational visibility: a durable record of what ran, where, when, and why it failed.
- Safer infrastructure automation: controlled execution across HTTP, servers, cloud services, and Kubernetes, with private access available through workers.
- AI compatibility: a product surface that can be consumed by agents without scraping a UI or relying on brittle conventions.

## Positioning

CronControl sits between simple cron-as-a-service tools and heavy workflow engines.

Compared with lightweight schedulers, it adds durable state, replay, queueing, and infrastructure execution.

Compared with workflow orchestration platforms, it stays focused on operational jobs, explicit execution methods and runtimes, and auditability instead of becoming a full application runtime.

## Distribution

CronControl is distributed as:

- Open-source code (MIT license)
- Single Go binary with embedded frontend
- Docker image
- Worker binary for private network execution

## Product Principles

- Durable by default
- API-first, not UI-first
- Multi-tenant control plane with explicit isolation
- Customer-owned private runtime when workloads need internal connectivity
- Observable long-running work
- Clear audit trails and predictable recovery behavior

## Non-goals

CronControl is not trying to be:

- A general DAG authoring studio
- A low-code automation builder
- A replacement for application-specific job frameworks
- A compute platform that runs arbitrary customer code inside the hosted control plane
