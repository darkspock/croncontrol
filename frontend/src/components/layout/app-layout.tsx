import { useState, useEffect } from 'react'
import { LogOut, ChevronDown, Building2, Search } from 'lucide-react'
import { Sidebar } from './sidebar'
import { CommandPalette } from '@/components/domain/command-palette'
import { api } from '@/api/client'

interface Workspace {
  workspace_id: string
  workspace_name: string
  workspace_slug: string
  role: string
}

interface AppLayoutProps {
  children: React.ReactNode
}

export function AppLayout({ children }: AppLayoutProps) {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [currentWorkspace, setCurrentWorkspace] = useState<any>(null)
  const [showWsSwitcher, setShowWsSwitcher] = useState(false)

  useEffect(() => {
    api.getWorkspace().then(r => setCurrentWorkspace(r.data)).catch(() => {})
    api.listWorkspaces().then(r => setWorkspaces(r.data || [])).catch(() => {})
  }, [])

  const handleNavigate = (path: string) => {
    setCurrentPath(path)
    window.history.pushState(null, '', path)
  }

  const handleLogout = () => {
    localStorage.removeItem('cc_api_key')
    window.location.href = '/'
  }

  const handleSwitchWorkspace = async (wsId: string) => {
    try {
      await api.switchWorkspace(wsId)
      setShowWsSwitcher(false)
      window.location.reload()
    } catch {
      // ignore
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <CommandPalette />
      <Sidebar currentPath={currentPath} onNavigate={handleNavigate} />
      <main className="ml-60 min-h-screen">
        <header className="sticky top-0 z-30 h-14 flex items-center justify-between px-6 border-b border-border bg-background/80 backdrop-blur-sm">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="text-foreground font-medium">CronControl</span>
            <span>/</span>
            {currentWorkspace && (
              <>
                <div className="relative">
                  <button
                    onClick={() => setShowWsSwitcher(!showWsSwitcher)}
                    className="flex items-center gap-1 px-1.5 py-0.5 rounded text-foreground hover:bg-muted/50 transition-colors"
                  >
                    <Building2 size={12} />
                    <span className="font-medium">{currentWorkspace.name}</span>
                    {workspaces.length > 1 && <ChevronDown size={12} />}
                  </button>
                  {showWsSwitcher && workspaces.length > 1 && (
                    <div className="absolute top-full left-0 mt-1 w-56 bg-popover border border-border rounded-md shadow-lg py-1 z-50">
                      {workspaces.map((ws) => (
                        <button
                          key={ws.workspace_id}
                          onClick={() => handleSwitchWorkspace(ws.workspace_id)}
                          className={`w-full text-left px-3 py-2 text-sm hover:bg-muted/50 transition-colors flex items-center justify-between ${
                            currentWorkspace.id === ws.workspace_id ? 'text-indigo-400' : 'text-foreground'
                          }`}
                        >
                          <span>{ws.workspace_name}</span>
                          <span className="text-xs text-muted-foreground">{ws.role}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
                <span>/</span>
              </>
            )}
            <span className="capitalize">{currentPath === '/' ? 'Dashboard' : currentPath.split('/').filter(Boolean)[0]}</span>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              title="Command palette (Cmd+K)"
            >
              <Search size={13} />
              <kbd className="text-[10px] px-1 py-0.5 rounded border border-border bg-muted">K</kbd>
            </button>
            <button
              onClick={handleLogout}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              title="Sign out"
            >
              <LogOut size={13} />
              Sign out
            </button>
          </div>
        </header>
        <div className="p-6">
          {children}
        </div>
      </main>
    </div>
  )
}
