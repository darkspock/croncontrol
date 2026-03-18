# EPIC-05: Multi-workspace Identity and Access

> **Note**: This epic was originally designed for a commercial SaaS model. CronControl is now open source. Billing, pricing tiers, and Stripe integration references are historical. The identity, workspace isolation, and RBAC portions remain relevant.

## Outcome

Provide workspace isolation, identity, and role-based access control.

## Why This Epic Exists

The product already assumes plans, quotas, roles, and tenant isolation. Those ideas need a coherent commercial and operational model or they will leak contradictions into the API and user experience.

## Goals

- Support workspace creation and isolation.
- Provide dashboard and API authentication.
- Support roles for admins, operators, and viewers.
- Define plan tiers, quotas, and limit enforcement.
- Implement the billing and subscription lifecycle.
- Support invitations and team management.

## In Scope

- Tenant model
- User model
- API key lifecycle
- Session and auth flows
- Invitations and role management
- Quotas and usage counters
- Plan enforcement
- Subscription lifecycle
- Upgrade, downgrade, cancellation, and grace-period rules
- Billing provider integration

## Out of Scope

- Deep dashboard polish
- Execution engine internals
- Non-commercial enterprise custom work

## Canonical Constraints

- Public product language uses `workspace`; `tenant` is internal only.
- A user can belong to multiple workspaces.
- `users.active_workspace_id` is persisted in the database.
- Email is globally unique across identities.
- Dashboard auth supports Google OAuth and email/password.
- Email verification is required before any real execution or dispatch.
- Standard plan upgrades are immediate with proration.
- Downgrades apply at the end of the billing cycle.
- Failed payment gets a 7-day grace period, then the workspace becomes restricted and running work is interrupted.
- Enterprise remains commercially configured rather than self-serve.

## Acceptance Criteria

- Workspace boundaries are enforced consistently at every layer.
- API keys, users, and roles follow one permission model.
- Invitation and verification flows are explicit and operable.
- Billing is more than a pricing table: subscription states and transitions are defined.
- Quota warnings and hard-limit behavior are consistent across API and dashboard.
- Hosted SaaS packaging is understandable to customers and implementable by engineering.

## Dependencies

- [EPIC-01 Canonical Platform Foundation](epic-01-canonical-platform-foundation.md)

## Follow-on Impact

This epic unlocks a real hosted product. The dashboard, alerts, worker enrollment, and agentic onboarding all depend on the tenant and billing model being stable.
