import { useState } from 'react'
import { cn } from '@/lib/utils'
import { useTheme } from '@/hooks/use-theme'
import {
  LayoutDashboard, Cpu, Play, Clock, Layers, Inbox, AlertTriangle,
  Settings, ChevronLeft, ChevronRight, Zap, Server, Sun, Moon, Shield
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
    label: 'Infrastructure',
    items: [
      { icon: Server, label: 'Workers', path: '/settings/workers' },
      { icon: Settings, label: 'Settings', path: '/settings' },
      { icon: Shield, label: 'Admin', path: '/admin' },
    ],
  },
]

export function Sidebar({ currentPath, onNavigate }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)
  const { theme, toggle: toggleTheme } = useTheme()

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
                  (item.path !== '/' && currentPath.startsWith(item.path))

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

      {/* Footer: theme toggle + collapse */}
      <div className="border-t border-border p-2 space-y-1">
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
