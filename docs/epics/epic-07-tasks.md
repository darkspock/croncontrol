# EPIC-07 Tasks: Dashboard and Admin Experience

> Status: DONE (95%) — updated 2026-03-18

## T07.1 Frontend Project Setup
- [x] Vite + React 19 + TypeScript.
- [x] shadcn/ui + Tailwind CSS 4.
- [x] TanStack Router (file-based routing).
- [x] TanStack Table v8.
- [x] TanStack Query v5 (React Query).
- [x] React Hook Form + Zod.
- [x] Lucide icons, Recharts, date-fns, cronstrue.
- [x] `components.json` for shadcn/ui.
- [x] Vite proxy to backend in dev.
- [x] `go:embed` integration for production build.

## T07.2 Authentication Pages
- [x] `/login`: Email/password form.
- [x] `/register`: name, email, password.
  - On success: show API key once ("Save this now"), redirect to dashboard.
- [x] Google OAuth button: Google SVG icon + "Sign in with Google" link to `/api/v1/auth/google/login` in login page. Forgot password link added.
- [x] `/verify-email?token=...`: auto-verifies on load, shows success/error, redirects to dashboard.
- [x] `/forgot-password`, `/reset-password?token=...`: request reset form, set new password form with confirmation. Both pages implemented.
- [x] No sidebar layout on auth pages (clean, centered).

## T07.3 Onboarding Flow
- [x] Onboarding banner on dashboard with 3 progressive steps:
  1. "Create your first process" — links to /processes/new.
  2. "Trigger a test run" — links to /processes.
  3. "Set up a queue" — links to /queues/new.
- [x] Progress bar showing completion percentage.
- [x] Dismissible (persists in localStorage).
- [x] Auto-dismisses when all steps completed.

## T07.4 App Layout
- [x] Sidebar: collapsible, with workspace name.
  - Navigation groups:
    - Scheduler: Dashboard, Processes, Runs, Timeline.
    - Queue: Queues, Jobs.
    - Settings (admin): Workspace, API Keys, Workers, Members.
  - Uses canonical terms: "Runs" not "Executions", "Workspace" not "Tenant".
- [x] Header: breadcrumbs, user menu.
- [x] Dark mode toggle.
- [x] Command palette (Cmd+K): searchable command list, arrow key navigation, 11 commands covering all pages. Search button in header.

## T07.5 Dashboard Page (`/`)
- [x] Summary cards: total processes, running runs, failed (24h), queue depth.
- [x] Recent runs table (last 10).
- [x] Process status list with state dots.
- [x] Auto-refresh.

## T07.6 Process Pages
- [x] **List** (`/processes`): Table with columns: name, schedule type, method, last run state, next run, tags, actions (trigger, pause/resume, delete).
- [x] **Detail** (`/processes/:id`): Stats, runs history, full configuration, dependency info.
- [x] **Create** (`/processes/new`):
  - Schedule type selector (cron/fixed_delay/on_demand).
  - Cron input with human-readable preview (cronstrue).
  - Execution method selector with dynamic config form per method.
  - Retry config: max_attempts, backoff list.
  - Dependency picker.
  - Environment variables editor.
  - Tags input.

## T07.7 Run Pages
- [x] **History** (`/runs`): filterable table. Filters: process, state, origin, date range.
- [x] **Detail** (`/runs/:id`):
  - Header: process name, method icon, runtime, origin badge, state badge.
  - Progress bar (non-HTTP only, polling while running).
  - Output viewer: stdout/stderr, monospace, auto-scroll while running.
  - Kill button (confirmation dialog).
  - Replay button (for terminal runs).
- [x] **Upcoming** (`/runs/upcoming`): pending/queued runs sorted by scheduled time, cancel action per run.
- [x] **Timeline** (`/runs/timeline`): horizontal timeline, processes on Y axis, time on X axis, color-coded bars, date range picker.

## T07.8 Queue Pages
- [x] **Overview** (`/queues`): cards per queue. Name, method icon, stats (pending/running/failed), health indicator.
- [x] **Create** (`/queues/new`): method, runtime, config, concurrency, retry, timeout.
- [x] **Detail** (`/queues/:id`): queue config header (method, concurrency, max attempts), state filter tabs, job data table with ID/reference/state/priority/attempts/created.

## T07.9 Job Pages
- [x] **All Jobs** (`/jobs`): filterable table across all queues.
- [x] **Detail** (`/jobs/:id`):
  - Metadata: queue name, state, reference, priority, idempotency key.
  - Attempt history: collapsible sections with full request/response.
  - Actions: replay, cancel.
- [x] **Failed Jobs** (`/jobs/failed`): grouped by queue with count badges, per-job replay button, bulk "Replay all" per queue.

## T07.10 Worker Management (Admin)
- [x] **List** (`/settings` workers tab): name, status, labels, capabilities, last heartbeat.
- [x] **Enroll**: create worker → show enrollment token with CLI command in copiable code block. Already in workers tab.
- [x] **Detail**: worker list shows status dot (online/offline/unhealthy), name, max concurrency, last heartbeat, delete action.

## T07.11 Settings Pages (Admin)
- [x] **Workspace** (`/settings`): workspace info.
- [x] **API Keys** (`/settings` keys tab): list (prefix, role, last used, created by), create (show key once in copiable box), revoke.
- [x] **Members** (`/settings` members tab): list, change role, remove.
- [x] **Credentials**: Settings > Credentials tab with 3 sections (SSH, SSM, K8s). List with name, metadata, delete action.
- [x] **Webhooks**: Settings > Webhooks tab. Create (URL/secret/events), list, test delivery, active/disabled status.
- [x] ~~**Billing**~~: removed — not applicable (open source).

## T07.12 Free Tier UX
> Not applicable to open-source version.

## T07.13 Workspace Switcher
- [x] Users with multiple workspaces: dropdown in header showing workspace name + role.
- [x] Switching calls `POST /workspaces/{id}/switch`, updates `active_workspace_id`, reloads page.
- [x] Backend: `GET /workspaces` (list user memberships), `POST /workspaces/{id}/switch` (set active).
- [x] Frontend: workspace name in breadcrumb header, dropdown with all workspaces.

## Acceptance Checklist
- [x] New workspace can sign up, configure, and launch first workload.
- [x] Operators can inspect runs, jobs, logs, heartbeats from dashboard.
- [x] Admins can manage users, API keys, workers from dashboard.
- [x] UI language matches canonical docs and API contracts.
- [x] Onboarding banner and workspace switcher implemented.
