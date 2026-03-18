# EPIC-05 Tasks: Multi-workspace Identity and Access

> **Note**: Billing tasks (T05.6 Plan Tiers, T05.7 Billing Integration) are not applicable to the open-source version. They are retained for historical context.

> Status: DONE (98%) — updated 2026-03-18

## T05.1 Workspace Lifecycle
- [x] `internal/workspace/workspace.go` (logic in handler layer):
  - Create workspace: generates `wsp_` ID, slug from name, state `active`.
  - Workspace states defined: active, suspended, archived.
  - State transitions validated.
- [x] Workspace isolation enforced at data access layer (all queries scoped by workspace_id).
- [x] Tests: creation, state transitions, isolation.

## T05.2 User Identity
- [x] One global user identity per email (`users.email` UNIQUE globally).
- [x] A user may belong to multiple workspaces (via `workspace_memberships`).
- [x] `users.active_workspace_id` persisted in DB.
- [x] Google OAuth: full flow (login URL, callback, user creation/lookup, session via API key). Config: `CC_AUTH_GOOGLE_CLIENT_ID`, `CC_AUTH_GOOGLE_CLIENT_SECRET`.
- [x] Password policy: minimum 12 characters.
- [x] Password reset: `POST /auth/forgot-password` creates token (1h TTL), `POST /auth/reset-password` validates and updates. Tokens single-use, prior tokens invalidated.
- [x] Email verification: `POST /register/verify` validates token, `POST /register/resend` creates new 24h token. Migration 00003 adds `user_tokens` table.
- [x] Tests: multi-workspace, password validation.

## T05.3 Authentication
- [x] `internal/auth/google.go`: Google OAuth 2.0 flow with LoginURL, Exchange (userinfo endpoint), CSRF state cookie.
- [x] `internal/auth/email.go`: Email + password flow.
  - Registration: create user + workspace + admin membership + default API key.
  - Login: validate bcrypt hash.
- [x] `internal/auth/session.go`: Session cookie support.
- [x] `internal/auth/apikey.go`: API key authentication.
  - Workspace-scoped. Single model (no separate machine key type).
  - SHA-256 hashed in DB, shown once at creation.
  - Optional expiration, no expiry by default.
  - Track: last_used_at, last_ip, last_user_agent.
  - Initial workspace bootstrap key is `admin` role.

## T05.4 Roles and Permissions
- [x] `internal/auth/rbac.go` (logic in middleware and handlers):
  - Three roles: `admin`, `operator`, `viewer`.
  - One role per workspace_membership.
  - Permission matrix:
    - viewer: read all, no mutations.
    - operator: trigger, replay, kill, pause/resume, enqueue. No CRUD on processes/queues/workers/credentials.
    - admin: full authority. Manages users, memberships, workers, API keys, webhooks, credentials.
  - Sensitive resources admin-only: workers, webhook subscriptions, ssh_credentials, ssm_profiles, k8s_clusters.
- [x] Middleware enforces permissions per endpoint.
- [x] At least one admin must remain per workspace (cannot remove last admin).
- [x] Tests: all role combinations, last admin protection.

## T05.5 Invitations and Team Management
- [x] Admin can add members to workspace.
- [x] Admin can change roles, remove members.
- [x] Email invitation: `POST /api/v1/users/invite` — if user exists adds membership directly, if not creates invitation token (7-day TTL). Admin-only.
- [x] Tests: role change, removal.

## ~~T05.6 Plan Tiers and Quotas~~ REMOVED
> Removed — not applicable to open-source version.

## ~~T05.7 Billing Integration~~ REMOVED
> Removed — not applicable to open-source version.

## T05.8 Registration API
- [x] `POST /api/v1/register` (no auth):
  - Create workspace + user + admin membership + default admin API key.
  - Return: workspace info, user info, API key (shown once).
- [x] Rate limiting on registration.
- [x] `POST /api/v1/register/verify` — email verification with token hash lookup.
- [x] `POST /api/v1/register/resend` — resend verification email (token regeneration).
- [x] Google OAuth registration flow — full: login, callback, auto-create user+workspace, redirect with API key.
- [x] Disposable email detection: `IsDisposableEmail()` in `internal/auth/disposable.go` with 30+ common domains. Checked at registration.
- [x] Tests: registration, rate limiting.

## Acceptance Checklist
- [x] Workspace boundaries enforced consistently at every layer.
- [x] API keys, users, and roles follow one permission model.
- [x] Verification flows are explicit and operable (email verify + password reset).
- [x] Quota warnings and hard-limits consistent across API and dashboard (N/A for open-source).
- [x] Packaging understandable to users and implementable.
