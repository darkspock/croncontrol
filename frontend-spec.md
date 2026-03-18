> **LEGACY DOCUMENT** — This file is superseded by the canonical documentation in `docs/`. When there is a conflict, `docs/` wins. See [docs/README.md](docs/README.md).

# CronControl — Frontend Specification

## 1. Technology Stack

| Layer | Technology | Version |
|---|---|---|
| Framework | React | 19+ |
| Build | Vite | 6+ |
| Language | TypeScript | 5.x |
| UI Components | shadcn/ui (Radix UI + Tailwind) | Latest |
| CSS | Tailwind CSS | 4+ |
| Routing | TanStack Router | Latest |
| Tables | TanStack Table | v8 |
| Forms | React Hook Form + Zod | Latest |
| HTTP Client | ky (or fetch wrapper) | Latest |
| State | TanStack Query (React Query) | v5 |
| Icons | Lucide React | Latest |
| Charts | Recharts | Latest |
| Date/Time | date-fns | Latest |
| Cron Preview | cronstrue | Latest |
| Code/Output Viewer | @monaco-editor/react or simple pre | Latest |

## 2. Project Structure

```
frontend/
├── src/
│   ├── main.tsx                    # Entry point
│   ├── App.tsx                     # Router + providers
│   ├── routeTree.gen.ts            # TanStack Router generated
│   │
│   ├── api/                        # API client layer
│   │   ├── client.ts               # Base HTTP client (ky instance with auth)
│   │   ├── types.ts                # Auto-generated from OpenAPI (or manual)
│   │   ├── auth.ts                 # Registration, login, logout
│   │   ├── tenants.ts              # Tenant info, usage, settings
│   │   ├── processes.ts            # Process CRUD + actions
│   │   ├── slots.ts                # Slot listing, cancel, kill, output
│   │   ├── queues.ts               # Queue CRUD
│   │   ├── jobs.ts                 # Job enqueue, list, replay, cancel
│   │   ├── dashboard.ts            # Dashboard aggregations
│   │   └── admin.ts                # Users, API keys, SSH keys
│   │
│   ├── hooks/                      # Custom hooks
│   │   ├── use-auth.ts             # Auth state + Google OAuth
│   │   ├── use-tenant.ts           # Current tenant context
│   │   ├── use-polling.ts          # Auto-refresh for running executions
│   │   └── use-plan.ts             # Plan limits + usage
│   │
│   ├── components/                 # Shared components
│   │   ├── ui/                     # shadcn/ui components (owned, customizable)
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── badge.tsx
│   │   │   ├── dialog.tsx
│   │   │   ├── dropdown-menu.tsx
│   │   │   ├── input.tsx
│   │   │   ├── select.tsx
│   │   │   ├── table.tsx
│   │   │   ├── tabs.tsx
│   │   │   ├── toast.tsx
│   │   │   ├── tooltip.tsx
│   │   │   ├── sheet.tsx
│   │   │   ├── skeleton.tsx
│   │   │   ├── progress.tsx
│   │   │   ├── separator.tsx
│   │   │   ├── command.tsx          # Command palette
│   │   │   └── ...
│   │   │
│   │   ├── layout/
│   │   │   ├── app-layout.tsx       # Sidebar + header + main content
│   │   │   ├── sidebar.tsx          # Navigation sidebar
│   │   │   ├── header.tsx           # Top bar with user menu + tenant info
│   │   │   └── breadcrumbs.tsx
│   │   │
│   │   ├── data/
│   │   │   ├── data-table.tsx       # Generic TanStack Table wrapper
│   │   │   ├── data-table-toolbar.tsx
│   │   │   ├── data-table-pagination.tsx
│   │   │   ├── data-table-faceted-filter.tsx
│   │   │   └── data-table-column-header.tsx
│   │   │
│   │   ├── domain/
│   │   │   ├── state-badge.tsx      # Colored badge for slot/job states
│   │   │   ├── origin-badge.tsx     # Badge for planner/manual/one_time/recovery
│   │   │   ├── target-icon.tsx      # Icon for execution target (local/http/ssh/ssm/k8s)
│   │   │   ├── progress-bar.tsx     # Heartbeat progress (total/current/%)
│   │   │   ├── cron-preview.tsx     # Human-readable cron expression
│   │   │   ├── duration-display.tsx # Formatted duration
│   │   │   ├── time-ago.tsx         # Relative time display
│   │   │   ├── output-viewer.tsx    # stdout/stderr viewer (monospace, tabs)
│   │   │   ├── heartbeat-timeline.tsx # Visual timeline of heartbeat events
│   │   │   ├── usage-meter.tsx      # Plan usage bar (executions/limit)
│   │   │   ├── plan-badge.tsx       # Free/Pro/Enterprise badge
│   │   │   └── upgrade-prompt.tsx   # Inline upgrade CTA for free tier
│   │   │
│   │   └── forms/
│   │       ├── process-form.tsx     # Create/edit process form
│   │       ├── queue-form.tsx       # Create/edit queue form
│   │       ├── target-config-form.tsx # Dynamic form per execution target
│   │       ├── cron-input.tsx       # Cron expression input with preview
│   │       ├── env-var-editor.tsx   # Key-value pair editor
│   │       ├── tag-input.tsx        # Tag/label input
│   │       └── json-editor.tsx      # JSON payload editor
│   │
│   ├── routes/                     # TanStack Router file-based routes
│   │   ├── __root.tsx              # Root layout
│   │   ├── login.tsx               # Login page (public)
│   │   ├── register.tsx            # Registration page (public)
│   │   ├── verify-email.tsx        # Email verification (public)
│   │   ├── _authenticated.tsx      # Auth guard layout
│   │   ├── _authenticated/
│   │   │   ├── index.tsx           # Dashboard (home)
│   │   │   ├── processes/
│   │   │   │   ├── index.tsx       # Process list
│   │   │   │   ├── new.tsx         # Create process
│   │   │   │   └── $processId.tsx  # Process detail
│   │   │   ├── executions/
│   │   │   │   ├── index.tsx       # Execution history
│   │   │   │   ├── upcoming.tsx    # Upcoming slots
│   │   │   │   ├── timeline.tsx    # Timeline view
│   │   │   │   └── $slotId.tsx     # Execution detail
│   │   │   ├── queues/
│   │   │   │   ├── index.tsx       # Queue overview
│   │   │   │   ├── new.tsx         # Create queue
│   │   │   │   └── $queueId.tsx    # Queue detail + job list
│   │   │   ├── jobs/
│   │   │   │   ├── index.tsx       # All jobs
│   │   │   │   ├── failed.tsx      # Failed jobs view
│   │   │   │   └── $jobId.tsx      # Job detail
│   │   │   └── settings/
│   │   │       ├── index.tsx       # Tenant settings
│   │   │       ├── billing.tsx     # Plan + usage
│   │   │       ├── users.tsx       # User management
│   │   │       ├── api-keys.tsx    # API key management
│   │   │       └── ssh-keys.tsx    # SSH key management
│   │
│   ├── lib/                        # Utilities
│   │   ├── utils.ts                # cn() helper, formatters
│   │   ├── constants.ts            # State colors, target icons, plan limits
│   │   └── validators.ts           # Zod schemas for forms
│   │
│   └── styles/
│       └── globals.css             # Tailwind base + shadcn theme
│
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
├── components.json                 # shadcn/ui config
└── package.json
```

## 3. Design System

### 3.1 Theme
Built on shadcn/ui default theme with CronControl customizations:
- **Primary**: Indigo/Blue (action buttons, active states)
- **Destructive**: Red (kill, delete, failed states)
- **Warning**: Amber (hung, retrying, approaching limits)
- **Success**: Green (completed, healthy)
- **Muted**: Gray (disabled, skipped, paused)

Dark mode supported via shadcn theme toggle.

### 3.2 State Colors

| State | Color | Badge variant |
|---|---|---|
| `pending` | Blue | `outline` |
| `queued` | Blue | `secondary` |
| `running` | Indigo | `default` (pulsing dot) |
| `completed` | Green | `success` |
| `failed` | Red | `destructive` |
| `hung` | Amber | `warning` |
| `killed` | Red | `destructive` (outline) |
| `skipped` | Gray | `secondary` |
| `cancelled` | Gray | `outline` |
| `paused` | Amber | `warning` (outline) |
| `retrying` | Amber | `warning` |

### 3.3 Execution Target Icons
| Target | Icon (Lucide) |
|---|---|
| `local` | `Terminal` |
| `http` | `Globe` |
| `ssh` | `KeyRound` |
| `ssm` | `Cloud` |
| `k8s` | `Container` |

### 3.4 Layout
- **Sidebar**: Fixed left, collapsible, 240px wide
  - Logo + tenant name
  - Navigation groups: Scheduler (Dashboard, Processes, Executions, Timeline), Queue (Queues, Jobs, Failed), Settings
  - Plan badge at bottom (Free/Pro/Enterprise)
- **Header**: Top bar with breadcrumbs, search (Command+K), user menu (avatar, role, logout)
- **Content**: Scrollable main area with page-level padding

### 3.5 Responsive
- Desktop-first (primary use case is monitoring dashboard)
- Sidebar collapses to icons on medium screens
- Tables switch to card layout on mobile
- Dialogs → full-screen sheets on mobile

## 4. Pages

### 4.1 Public Pages

#### Login (`/login`)
- Google OAuth button (primary)
- Email + password form (secondary)
- Link to registration
- Clean, centered layout (no sidebar)

#### Register (`/register`)
- Email, name, password fields
- Google OAuth as alternative
- On success: redirect to dashboard with toast showing API key
- "Your API key: `cc_live_...` — save it now, it won't be shown again"

#### Verify Email (`/verify-email?token=...`)
- Auto-verify on load
- Success: redirect to dashboard
- Error: show message + resend option

### 4.2 Dashboard (`/`)
```
┌──────────────────────────────────────────────────────┐
│  Summary Cards (4)                                    │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                │
│  │Procs │ │Runnin│ │Failed│ │Queue │                │
│  │  12  │ │  3   │ │  2   │ │  45  │                │
│  └──────┘ └──────┘ └──────┘ └──────┘                │
│                                                       │
│  ┌─────────────────────┐ ┌──────────────────────────┐│
│  │ Recent Executions   │ │ Process Status           ││
│  │ (table, last 10)    │ │ (list with status dots)  ││
│  │                     │ │                          ││
│  │ name | state | time │ │ daily-report    ● ok     ││
│  │ ...                 │ │ sync-users      ● running││
│  │                     │ │ cleanup         ✕ failed ││
│  └─────────────────────┘ └──────────────────────────┘│
│                                                       │
│  ┌──────────────────────────────────────────────────┐│
│  │ Usage This Month                                  ││
│  │ ████████████░░░░░░ 680/1000 executions (68%)     ││
│  │ ██████░░░░░░░░░░░░ 120/500 queue jobs (24%)      ││
│  └──────────────────────────────────────────────────┘│
│  Auto-refresh: 10s                                    │
└──────────────────────────────────────────────────────┘
```

### 4.3 Process List (`/processes`)
- TanStack Table with columns: name, schedule (cron preview), target (icon), state (last execution), next run, tags, actions
- Toolbar: search, filter by tag/schedule_type/target/enabled, create button
- Row actions: trigger, pause/resume, edit, delete
- Free tier: "Create Process" shows upgrade prompt if at limit (5)

### 4.4 Process Detail (`/processes/:id`)
- **Header**: Name, schedule badge, target icon, enabled/paused toggle, trigger/edit/delete buttons
- **Tabs**:
  - **Overview**: Config summary, dependency chain visualization, stats (success rate, avg duration, last 7d)
  - **Executions**: Data table of execution history (filterable by state, date)
  - **Upcoming**: Pending/queued slots with cancel action
  - **Audit**: Configuration change log

### 4.5 Process Form (`/processes/new`, edit dialog)
- React Hook Form + Zod validation
- Step 1: Basic info (name, schedule type, cron/interval/manual)
  - Cron input with real-time preview ("Every day at 13:00 UTC")
  - Interval input with human-readable display
- Step 2: Execution target (select target → dynamic config form)
  - HTTP: URL, method, headers, body template
  - SSH: host/discovery URL, user, key (select from uploaded keys), command
  - SSM: region, tag key/value, command
  - K8s: namespace, image, command, resources
  - Local: command, working directory (disabled on SaaS with tooltip)
  - **Free tier**: Only HTTP selectable, others greyed out with "Pro plan" badge
- Step 3: Behavior (timeout, heartbeat, parallelism, overlap, miss policy)
- Step 4: Advanced (env vars, tags, dependencies)
- Preview before submit

### 4.6 Execution Detail (`/executions/:slotId`)
```
┌──────────────────────────────────────────────────────┐
│  Process: daily-report    Target: HTTP    Origin: planner │
│  State: ● running         Started: 2min ago              │
│                                                           │
│  ┌──────────────────────────────────────────────────┐    │
│  │ Progress                                          │    │
│  │ ████████████████░░░░░░░░░░ 450/1000 (45%)        │    │
│  │ "Processing batch 5 of 10"                        │    │
│  └──────────────────────────────────────────────────┘    │
│                                                           │
│  ┌──────────────────────────────────────────────────┐    │
│  │ Heartbeat Timeline                                │    │
│  │ ●────●────●────●────●────●                       │    │
│  │ 0%  10%  20%  30%  40%  45%                      │    │
│  └──────────────────────────────────────────────────┘    │
│                                                           │
│  ┌──────────────────────────────────────────────────┐    │
│  │ Output  [stdout] [stderr]                         │    │
│  │ ┌────────────────────────────────────────────┐   │    │
│  │ │ > Loading configuration...                  │   │    │
│  │ │ > Connecting to database...                 │   │    │
│  │ │ > Processing batch 1 of 10...               │   │    │
│  │ │ > Processing batch 2 of 10...               │   │    │
│  │ │ █                                           │   │    │
│  │ └────────────────────────────────────────────┘   │    │
│  │ Auto-scroll: ON                                   │    │
│  └──────────────────────────────────────────────────┘    │
│                                                           │
│  [Kill Execution]                                         │
│  Polling: 5s                                              │
└──────────────────────────────────────────────────────────┘
```
- Progress bar: animated, shows total/current/% + message
- Heartbeat timeline: horizontal dots with timestamps on hover
- Output viewer: monospace, tabs for stdout/stderr, auto-scroll while running, line numbers
- Kill button: confirmation dialog
- Polling every 5s while running, stops when terminal state

### 4.7 Upcoming (`/executions/upcoming`)
- Table of pending/queued slots ordered by scheduled_at
- Columns: process name, scheduled_at, state (pending/queued), origin
- Actions: cancel, view process
- "Add one-time execution" button (opens dialog: select process, pick datetime)

### 4.8 Timeline (`/executions/timeline`)
- Horizontal timeline (Recharts or custom)
- Y axis: processes, X axis: time
- Bars colored by state
- Click bar → go to execution detail
- Date range picker (last 1h, 6h, 24h, 7d, custom)

### 4.9 Queue Overview (`/queues`)
- Cards per queue (shadcn Card):
  - Name, target icon, enabled/paused badge
  - Stats: pending | running | failed (24h) | completed (24h)
  - Health indicator (green/yellow/red based on failure rate)
- Create queue button
- Click card → queue detail

### 4.10 Queue Detail (`/queues/:id`)
- Queue config + stats header
- Job data table: state, priority, reference, created_at, attempts, actions
- Filters: state, date range, reference search
- Bulk actions: cancel all pending, replay all failed
- Pause/resume queue

### 4.11 Job Detail (`/jobs/:id`)
```
┌──────────────────────────────────────────────────────┐
│  Queue: emails    State: ● failed    Attempts: 3/3   │
│  Reference: order-12345                               │
│                                                       │
│  Attempt History                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ #1  ✕ failed    2s    HTTP 500                 │  │
│  │ ┌ Request ─────────────────────────────────┐   │  │
│  │ │ POST https://api.example.com/send-email  │   │  │
│  │ │ Content-Type: application/json           │   │  │
│  │ │ {"to": "user@example.com", ...}          │   │  │
│  │ └─────────────────────────────────────────┘   │  │
│  │ ┌ Response ────────────────────────────────┐   │  │
│  │ │ 500 Internal Server Error                │   │  │
│  │ │ {"error": "SMTP connection failed"}      │   │  │
│  │ └─────────────────────────────────────────┘   │  │
│  ├────────────────────────────────────────────────┤  │
│  │ #2  ✕ failed    1s    HTTP 500  (retry +5m)   │  │
│  │ ...                                            │  │
│  ├────────────────────────────────────────────────┤  │
│  │ #3  ✕ failed    1s    HTTP 500  (retry +15m)  │  │
│  │ ...                                            │  │
│  └────────────────────────────────────────────────┘  │
│                                                       │
│  [Replay Job]  (opens form to override fields)        │
└──────────────────────────────────────────────────────┘
```
- Collapsible attempt sections (expand to see full request/response)
- JSON syntax highlighting for bodies
- Retry timeline: visual representation of attempts and wait times
- Replay button: dialog with pre-filled fields, user can override

### 4.12 Failed Jobs (`/jobs/failed`)
- Grouped by queue with error distribution:
  - "emails: 15 timeout, 3 connection refused, 7 HTTP 500"
  - "invoices: 2 HTTP 400"
- One-click replay per job
- Bulk replay per queue or selection
- Date range filter

### 4.13 Settings (`/settings`)

#### Tenant Settings (`/settings`)
- Tenant name, timezone selector
- Webhook URL
- Allowed domains (URL allowlist) — tag input

#### Billing (`/settings/billing`)
- Current plan card (Free/Pro/Enterprise)
- Usage meters: executions, queue jobs, processes, queues
- Feature comparison table (what's included in each plan)
- Upgrade button (→ Stripe checkout)

#### Users (`/settings/users`)
- Table: email, name, role, last login, actions
- Invite user dialog (email + role)
- Change role dropdown
- Remove user (confirmation)

#### API Keys (`/settings/api-keys`)
- Table: name, prefix (`cc_live_abc1...`), role, last used, created by
- Create key dialog: name + role → shows full key ONCE in a copiable box
- Revoke key (confirmation)

#### SSH Keys (`/settings/ssh-keys`)
- Table: name, fingerprint, created at
- Upload key dialog: name + paste private key (file upload or textarea)
- Delete key (confirmation)
- **Hidden on free tier** (greyed out with upgrade prompt)

## 5. Data Fetching Pattern

### 5.1 TanStack Query
All API calls use TanStack Query for caching, deduplication, and background refetching:

```typescript
// Example: process list
const { data, isLoading } = useQuery({
  queryKey: ['processes', { page, filters }],
  queryFn: () => api.processes.list({ page, ...filters }),
})

// Example: mutation with cache invalidation
const createProcess = useMutation({
  mutationFn: api.processes.create,
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['processes'] })
    toast.success('Process created')
  },
  onError: (error) => {
    toast.error(error.message, { description: error.hint })
  },
})
```

### 5.2 Polling for Running Executions
```typescript
const { data } = useQuery({
  queryKey: ['slot', slotId],
  queryFn: () => api.slots.get(slotId),
  refetchInterval: (query) => {
    const state = query.state.data?.data.state
    return state === 'running' ? 5000 : false // poll every 5s while running
  },
})
```

### 5.3 Error Handling
All API errors return `{ error: { code, message, hint } }`. The HTTP client intercepts errors and creates typed error objects:

```typescript
class ApiError extends Error {
  code: string
  hint?: string
}

// Toast errors automatically show hint
toast.error(error.message, { description: error.hint })
```

Plan limit errors (HTTP 402) show an inline upgrade prompt component instead of a toast.

## 6. Auth Flow

### 6.1 Google OAuth
1. User clicks "Sign in with Google"
2. Redirect to `/auth/google/login`
3. Google OAuth flow
4. Backend creates session cookie
5. Redirect to `/` (dashboard)
6. Frontend detects session cookie, fetches `/api/v1/tenant` to get user + tenant info

### 6.2 Email/Password
1. User fills login form → POST `/auth/login`
2. Backend validates, creates session cookie
3. Frontend redirects to dashboard

### 6.3 Session Management
- Session cookie is HTTP-only (not accessible from JS)
- Frontend calls `GET /api/v1/tenant` on page load to check if authenticated
- If 401 → redirect to `/login`
- Logout: POST `/auth/logout` → clear cookie → redirect to `/login`

### 6.4 Route Guard
```typescript
// _authenticated.tsx (TanStack Router layout)
export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({ to: '/login' })
    }
  },
  component: AuthenticatedLayout,
})
```

## 7. Key Interactions

### 7.1 Command Palette (Cmd+K)
shadcn `Command` component. Actions:
- Search processes by name
- Search jobs by reference
- Quick navigate to any page
- Quick actions: trigger process, create queue, view failed jobs

### 7.2 Toasts
shadcn `Sonner` toast. Events:
- Success: "Process created", "Execution triggered", "Job replayed"
- Error: API error message + hint
- Warning: "Approaching plan limit (80%)"
- Info: "Email verification sent"

### 7.3 Confirmation Dialogs
shadcn `AlertDialog`. For destructive actions:
- Kill execution
- Delete process/queue
- Cancel slot
- Revoke API key
- Bulk replay (shows count)

### 7.4 Real-Time Indicators
- Running executions: pulsing dot on state badge
- Dashboard: auto-refresh indicator ("Updated 3s ago")
- Execution detail: live progress bar + auto-scrolling output

## 8. Free Tier UX

Free tier users see the full UI but with clear limitations:

| Feature | Free tier behavior |
|---|---|
| SSH/SSM/K8s targets | Target selector shows lock icon + "Pro" badge, tooltip: "Upgrade to Pro to use SSH targets" |
| Dependencies | Dependency picker disabled + upgrade prompt |
| Webhook alerts | Settings shows "Upgrade to Pro" instead of webhook URL field |
| Users | "Invite user" disabled + upgrade prompt |
| SSH keys | Entire section hidden / shows upgrade card |
| Usage meters | Always visible, turns amber at 80%, red at 95% |
| Creating beyond limit | Dialog shows: "You've reached the free plan limit of 5 processes. Upgrade to Pro for up to 50." with upgrade button |

The upgrade CTA is never blocking — users can always see and navigate the full UI.
