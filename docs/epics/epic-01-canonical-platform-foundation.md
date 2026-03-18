# EPIC-01: Canonical Platform Foundation

## Outcome

Create a stable, contradiction-free foundation for the product model, terminology, APIs, schema conventions, and implementation contracts.

## Why This Epic Exists

The current documentation mixes business intent, technical design, and draft implementation detail. Several cross-cutting contracts are inconsistent, which makes downstream work risky.

This epic exists to make later engineering work cheaper and safer.

## Goals

- Establish `docs/` as the canonical documentation set.
- Unify core terminology across product, API, schema, and UI.
- Define the control plane versus execution plane model.
- Replace the old `local execution` language with the `worker runtime` model.
- Standardize response envelopes and error structure.
- Define core schema and API conventions before feature growth.

## In Scope

- Documentation migration rules
- Core glossary and product terminology
- OpenAPI conventions
- SQL schema conventions
- Workspace-scoped resource rules
- Secret classification and redaction policy
- Core error model
- Configuration surface required for local development and production

## Out of Scope

- Full scheduling behavior
- Execution method implementation
- Queue processing
- Dashboard feature completeness

## Canonical Constraints

- Public product terms are `workspace`, `process`, `run`, and `queue`
- `tenant` and `scheduled_slot` are internal terms only
- CronControl is hosted SaaS only
- `worker` is a runtime and gateway, not an execution method
- Job idempotency is workspace-scoped
- Plan exhaustion uses `429` with structured error payloads
- Secrets are encrypted at rest and redacted in product surfaces
- Run origins are explicit first-class values in the model

## Acceptance Criteria

- `docs/` is the canonical documentation root.
- The public execution model uses the worker runtime instead of any `local execution` concept.
- API responses follow one consistent envelope pattern.
- Core cross-cutting contradictions are written down and resolved.
- Security guidance explicitly covers secrets outside SSH keys.
- The data model and API language use the same resource names.

## Dependencies

- None

## Follow-on Impact

Every later epic depends on this one. If this epic stays fuzzy, contradictions will spread into the schema, OpenAPI contract, dashboard copy, CLI behavior, and billing rules.
