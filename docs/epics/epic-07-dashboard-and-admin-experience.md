# EPIC-07: Dashboard and Admin Experience

## Outcome

Ship the product surface for humans: onboarding, visibility, administration, and day-to-day operations.

## Why This Epic Exists

Even in an API-first product, teams need a clear dashboard for setup, troubleshooting, and collaboration. The UI should expose the same core model as the API without inventing a second product vocabulary.

## Goals

- Provide onboarding for first-time users.
- Expose process, run, queue, and job visibility.
- Support admin workflows for users, API keys, workers, and billing.
- Surface plan usage, warnings, and operational issues clearly.

## In Scope

- Authentication pages
- Onboarding flow
- Dashboard overview
- Process pages
- Run detail views
- Queue pages
- User and key management
- Worker management views
- Billing and plan pages
- Settings and workspace configuration

## Out of Scope

- Advanced visual workflow editing
- White-labeling

## Canonical Constraints

- The UI uses public terms such as `workspace`, `process`, `run`, and `queue`
- Polling is the default real-time mechanism in the first version
- Admin-only resources stay admin-only in the UI
- Operators get operational actions but not sensitive configuration
- Worker enrollment and diagnostics are first-class admin workflows

## Acceptance Criteria

- A new workspace can sign up, configure the product, and launch a first workload.
- Operators can inspect executions, jobs, logs, and heartbeats from the dashboard.
- Admins can manage users, API keys, workers, and plan settings.
- The UI language matches the canonical docs and API contracts.
- Plan limits, warnings, and upgrade paths are understandable.

## Dependencies

- [EPIC-05 Multi-tenant SaaS, Identity, and Billing](epic-05-multi-tenant-saas-identity-and-billing.md)
- [EPIC-06 Observability, Alerts, and Data Lifecycle](epic-06-observability-alerts-and-data-lifecycle.md)

## Follow-on Impact

This epic determines how customers actually perceive the product. If the UI drifts from the core model, support and onboarding costs will rise quickly.
