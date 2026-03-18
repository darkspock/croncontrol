import { useState, useEffect, useRef } from 'react'
import { Search, Cpu, Play, Layers, Inbox, Settings, Clock, LayoutDashboard } from 'lucide-react'

interface CommandItem {
  id: string
  icon: React.ElementType
  label: string
  description?: string
  action: () => void
}

export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const navigate = (path: string) => {
    window.history.pushState(null, '', path)
    window.dispatchEvent(new PopStateEvent('popstate'))
    setOpen(false)
  }

  const commands: CommandItem[] = [
    { id: 'dashboard', icon: LayoutDashboard, label: 'Dashboard', description: 'Go to dashboard', action: () => navigate('/') },
    { id: 'processes', icon: Cpu, label: 'Processes', description: 'View all processes', action: () => navigate('/processes') },
    { id: 'new-process', icon: Cpu, label: 'Create Process', description: 'Create a new process', action: () => navigate('/processes/new') },
    { id: 'runs', icon: Play, label: 'Runs', description: 'View run history', action: () => navigate('/runs') },
    { id: 'upcoming', icon: Clock, label: 'Upcoming Runs', description: 'Pending and queued runs', action: () => navigate('/runs/upcoming') },
    { id: 'timeline', icon: Clock, label: 'Timeline', description: 'Visual execution timeline', action: () => navigate('/runs/timeline') },
    { id: 'queues', icon: Layers, label: 'Queues', description: 'View all queues', action: () => navigate('/queues') },
    { id: 'new-queue', icon: Layers, label: 'Create Queue', description: 'Create a new queue', action: () => navigate('/queues/new') },
    { id: 'jobs', icon: Inbox, label: 'Jobs', description: 'View all jobs', action: () => navigate('/jobs') },
    { id: 'failed', icon: Inbox, label: 'Failed Jobs', description: 'View failed jobs', action: () => navigate('/jobs/failed') },
    { id: 'settings', icon: Settings, label: 'Settings', description: 'Workspace settings', action: () => navigate('/settings') },
  ]

  const filtered = query
    ? commands.filter(c => c.label.toLowerCase().includes(query.toLowerCase()) || c.description?.toLowerCase().includes(query.toLowerCase()))
    : commands

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen(o => !o)
        setQuery('')
        setSelected(0)
      }
      if (e.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  // Arrow key navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelected(s => Math.min(s + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelected(s => Math.max(s - 1, 0))
    } else if (e.key === 'Enter' && filtered[selected]) {
      filtered[selected].action()
    }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh]">
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setOpen(false)} />

      {/* Palette */}
      <div className="relative w-full max-w-md rounded-xl border border-border bg-card shadow-2xl overflow-hidden">
        <div className="flex items-center gap-2 px-4 border-b border-border">
          <Search size={16} className="text-muted-foreground" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setSelected(0) }}
            onKeyDown={handleKeyDown}
            placeholder="Search commands..."
            className="flex-1 py-3 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
          <kbd className="text-[10px] text-muted-foreground px-1.5 py-0.5 rounded border border-border bg-muted">ESC</kbd>
        </div>

        <div className="max-h-64 overflow-y-auto p-1">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-sm text-muted-foreground">No results</div>
          ) : filtered.map((cmd, i) => (
            <button
              key={cmd.id}
              onClick={cmd.action}
              onMouseEnter={() => setSelected(i)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left transition-colors ${
                i === selected ? 'bg-indigo-500/10 text-indigo-400' : 'text-foreground hover:bg-muted/50'
              }`}
            >
              <cmd.icon size={16} className="flex-shrink-0" />
              <div>
                <p className="text-sm">{cmd.label}</p>
                {cmd.description && <p className="text-xs text-muted-foreground">{cmd.description}</p>}
              </div>
            </button>
          ))}
        </div>

        <div className="px-4 py-2 border-t border-border flex items-center gap-4 text-[10px] text-muted-foreground">
          <span><kbd className="px-1 py-0.5 rounded border border-border bg-muted">&uarr;&darr;</kbd> navigate</span>
          <span><kbd className="px-1 py-0.5 rounded border border-border bg-muted">Enter</kbd> select</span>
          <span><kbd className="px-1 py-0.5 rounded border border-border bg-muted">Esc</kbd> close</span>
        </div>
      </div>
    </div>
  )
}
