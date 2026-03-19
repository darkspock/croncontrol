import { useState } from 'react'
import { cn } from '@/lib/utils'
import { useTheme } from '@/hooks/use-theme'
import {
  LayoutDashboard, Cpu, Play, Clock, Layers, Inbox, AlertTriangle, Music,
  Settings, ChevronLeft, ChevronRight, Zap, Server, Sun, Moon, Shield,
  HelpCircle, X
} from 'lucide-react'

interface SidebarProps {
  currentPath: string
  onNavigate: (path: string) => void
}

const navGroups = [
  {
    label: 'Scheduler',
    items: [
      { icon: LayoutDashboard, label: 'Dashboard', path: '/' },
      { icon: Cpu, label: 'Processes', path: '/processes' },
      { icon: Play, label: 'Runs', path: '/runs' },
      { icon: Clock, label: 'Timeline', path: '/runs/timeline' },
    ],
  },
  {
    label: 'Queue',
    items: [
      { icon: Layers, label: 'Queues', path: '/queues' },
      { icon: Inbox, label: 'Jobs', path: '/jobs' },
      { icon: AlertTriangle, label: 'Failed Jobs', path: '/jobs/failed' },
    ],
  },
  {
    label: 'Orchestras',
    items: [
      { icon: Music, label: 'Orchestras', path: '/orchestras' },
    ],
  },
  {
    label: 'Infrastructure',
    items: [
      { icon: Server, label: 'Workers', path: '/settings/workers' },
      { icon: Settings, label: 'Settings', path: '/settings' },
      { icon: Shield, label: 'Admin', path: '/admin' },
    ],
  },
]

const helpTexts: Record<string, { title: string; text: string }> = {
  '/': {
    title: 'Dashboard',
    text: 'Overview of your workspace. See how many processes are running, recent executions, and overall health. Use the onboarding steps to get started quickly.',
  },
  '/processes': {
    title: 'Processes',
    text: 'Processes are your scheduled or on-demand tasks. Create a new process with a cron expression, fixed delay, or manual trigger. Each process defines what to execute (HTTP, SSH, SSM, K8s) and how (retries, timeout, dependencies).',
  },
  '/processes/new': {
    title: 'Create Process',
    text: 'Define the schedule type (cron, fixed delay, or on demand), the execution method and target URL/command, and optional retry and timeout settings. The cron preview shows a human-readable description of your schedule.',
  },
  '/runs': {
    title: 'Runs',
    text: 'Every execution of a process creates a run. Filter by state (pending, running, completed, failed) or origin (cron, manual, replay). Click a run to see output, error details, and attempt history.',
  },
  '/runs/timeline': {
    title: 'Timeline',
    text: 'Visual timeline of all executions across processes. Each bar represents a run, color-coded by state. Use the time range buttons (1h, 6h, 24h, 7d) to zoom in or out.',
  },
  '/runs/upcoming': {
    title: 'Upcoming Runs',
    text: 'Shows all pending and queued runs sorted by scheduled time. You can cancel individual runs before they execute.',
  },
  '/queues': {
    title: 'Queues',
    text: 'Queues process event-driven background jobs. Create a queue with an execution method, then enqueue jobs via the API or SDK. Jobs are retried automatically on failure with configurable backoff.',
  },
  '/jobs': {
    title: 'Jobs',
    text: 'All jobs across all queues. Filter by state to find pending, running, or failed jobs. Click a job to see its attempt history with full request/response details.',
  },
  '/jobs/failed': {
    title: 'Failed Jobs',
    text: 'Jobs grouped by queue that have exhausted all retry attempts. Use the Replay button to retry individual jobs or "Replay all" to retry an entire queue.',
  },
  '/settings': {
    title: 'Settings',
    text: 'Manage API keys, workers, team members, webhook subscriptions, and credentials (SSH, SSM, K8s). API keys are shown once at creation — save them immediately.',
  },
  '/settings/workers': {
    title: 'Workers',
    text: 'Workers run tasks inside your private network. Create a worker to get an enrollment token, then install the worker binary on your server. Workers report heartbeats every 15 seconds.',
  },
  '/admin': {
    title: 'Platform Admin',
    text: 'Global administration across all workspaces. View platform stats, manage workspace states (active/suspended/archived), and promote or revoke platform admin access.',
  },
  '/orchestras': {
    title: 'Orchestras',
    text: 'Multi-step workflows with a Director that coordinates AgentNodes (tasks). Create orchestras via the SDK, view the score (execution timeline), and interact via the real-time chat.',
  },
}

function getHelpForPath(path: string) {
  if (helpTexts[path]) return helpTexts[path]
  // Match dynamic routes
  if (path.startsWith('/processes/') && path !== '/processes/new') return { title: 'Process Detail', text: 'View process configuration, execution history, and stats. Trigger a manual run, pause/resume scheduling, or delete the process.' }
  if (path.startsWith('/runs/') && !path.includes('timeline') && !path.includes('upcoming')) return { title: 'Run Detail', text: 'Full execution details: state, duration, exit code, attempt history with error messages, and stdout/stderr output. Use Replay to re-run with the same configuration.' }
  if (path.startsWith('/jobs/') && path !== '/jobs/failed') return { title: 'Job Detail', text: 'Job metadata, attempt history with request/response details, and error messages. Replay to retry the job or cancel if still pending.' }
  if (path.startsWith('/queues/') && path !== '/queues/new') return { title: 'Queue Detail', text: 'Queue configuration and all jobs in this queue. Filter by state and use bulk actions to replay failed jobs.' }
  if (path.startsWith('/settings')) return helpTexts['/settings']
  return helpTexts['/']
}

export function Sidebar({ currentPath, onNavigate }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const { theme, toggle: toggleTheme } = useTheme()

  const help = getHelpForPath(currentPath)

  return (
    <aside className={cn(
      'fixed left-0 top-0 h-screen flex flex-col border-r border-border bg-sidebar sidebar-texture transition-all duration-200 z-40',
      collapsed ? 'w-16' : 'w-60'
    )}>
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-4 h-14 border-b border-border">
        <div className="w-7 h-7 rounded-md bg-indigo-500 flex items-center justify-center flex-shrink-0">
          <Zap size={14} className="text-white" />
        </div>
        {!collapsed && (
          <div className="flex flex-col min-w-0">
            <span className="text-sm font-semibold text-foreground tracking-tight">CronControl</span>
            <span className="text-[10px] text-muted-foreground font-mono uppercase tracking-widest">open source</span>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-3 px-2 space-y-5">
        {navGroups.map((group) => (
          <div key={group.label}>
            {!collapsed && (
              <p className="px-2 mb-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-widest">
                {group.label}
              </p>
            )}
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const isActive = currentPath === item.path ||
                  (item.path !== '/' && item.path !== '/settings' && item.path !== '/jobs' && currentPath.startsWith(item.path)) ||
                  (item.path === '/settings' && currentPath === '/settings') ||
                  (item.path === '/settings/workers' && currentPath.startsWith('/settings/workers')) ||
                  (item.path === '/jobs' && currentPath === '/jobs') ||
                  (item.path === '/jobs/failed' && currentPath === '/jobs/failed')

                return (
                  <button
                    key={item.path}
                    onClick={() => onNavigate(item.path)}
                    className={cn(
                      'w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-sm transition-colors',
                      isActive
                        ? 'bg-indigo-500/10 text-indigo-400'
                        : 'text-sidebar-foreground hover:text-foreground hover:bg-muted/50'
                    )}
                    title={collapsed ? item.label : undefined}
                  >
                    <item.icon size={16} className="flex-shrink-0" />
                    {!collapsed && <span>{item.label}</span>}
                  </button>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* Help panel */}
      {showHelp && !collapsed && (
        <div className="mx-2 mb-2 p-3 rounded-lg border border-indigo-500/20 bg-indigo-500/5 relative">
          <button onClick={() => setShowHelp(false)} className="absolute top-2 right-2 text-muted-foreground hover:text-foreground">
            <X size={12} />
          </button>
          <p className="text-xs font-semibold text-indigo-400 mb-1">{help.title}</p>
          <p className="text-xs text-muted-foreground leading-relaxed">{help.text}</p>
        </div>
      )}

      {/* Footer: help + theme toggle + collapse */}
      <div className="border-t border-border p-2 space-y-1">
        <button
          onClick={() => setShowHelp(!showHelp)}
          className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
          title="Contextual help"
        >
          <HelpCircle size={16} className={showHelp ? 'text-indigo-400' : ''} />
          {!collapsed && <span className="text-sm">{showHelp ? 'Hide help' : 'Help'}</span>}
        </button>
        <button
          onClick={toggleTheme}
          className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
          title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
        >
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
          {!collapsed && <span className="text-sm">{theme === 'dark' ? 'Light mode' : 'Dark mode'}</span>}
        </button>
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="w-full flex items-center justify-center py-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
        >
          {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
        </button>
      </div>
    </aside>
  )
}
