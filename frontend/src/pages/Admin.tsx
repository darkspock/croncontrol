import { useState, useEffect } from 'react'
import { Shield, Building2, Users, BarChart3, Play, AlertTriangle, Server, Copy, Check, HardDrive } from 'lucide-react'
import { formatTimeAgo, cn } from '@/lib/utils'

const apiKey = () => localStorage.getItem('cc_api_key') || ''
const headers = () => ({ 'Content-Type': 'application/json', 'X-API-Key': apiKey() })

async function adminFetch(path: string, options?: RequestInit) {
  const res = await fetch(`/api/v1/admin${path}`, { ...options, headers: headers() })
  if (!res.ok) return null
  return res.json()
}

export function Admin() {
  const [tab, setTab] = useState<'stats' | 'workspaces' | 'users' | 'infra'>('stats')

  const tabs = [
    { id: 'stats' as const, label: 'Dashboard', icon: BarChart3 },
    { id: 'workspaces' as const, label: 'Workspaces', icon: Building2 },
    { id: 'users' as const, label: 'Users', icon: Users },
    { id: 'infra' as const, label: 'Infrastructure', icon: HardDrive },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Shield size={20} className="text-amber-400" />
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Platform Admin</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Cross-workspace administration</p>
        </div>
      </div>

      <div className="flex gap-1 border-b border-border">
        {tabs.map((t) => (
          <button key={t.id} onClick={() => setTab(t.id)}
            className={`flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === t.id ? 'border-amber-500 text-amber-400' : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}>
            <t.icon size={14} /> {t.label}
          </button>
        ))}
      </div>

      {tab === 'stats' && <StatsTab />}
      {tab === 'workspaces' && <WorkspacesTab />}
      {tab === 'users' && <UsersTab />}
      {tab === 'infra' && <InfraAdminTab />}
    </div>
  )
}

function StatsTab() {
  const [stats, setStats] = useState<any>(null)

  useEffect(() => {
    adminFetch('/stats').then(d => setStats(d?.data))
  }, [])

  if (!stats) return <div className="text-sm text-muted-foreground">Loading...</div>

  const cards = [
    { label: 'Workspaces', value: stats.active_workspaces, total: stats.total_workspaces, icon: Building2, color: 'text-blue-400' },
    { label: 'Users', value: stats.total_users, total: null, icon: Users, color: 'text-indigo-400' },
    { label: 'Running Runs', value: stats.running_runs, total: null, icon: Play, color: 'text-emerald-400' },
    { label: 'Failed (24h)', value: stats.failed_runs_24h, total: null, icon: AlertTriangle, color: 'text-red-400' },
    { label: 'Online Workers', value: stats.online_workers, total: null, icon: Server, color: 'text-cyan-400' },
    { label: 'Platform Admins', value: stats.platform_admins, total: null, icon: Shield, color: 'text-amber-400' },
  ]

  return (
    <div className="grid grid-cols-3 gap-4">
      {cards.map((c) => (
        <div key={c.label} className="rounded-lg border border-border bg-card p-4">
          <div className="flex items-center gap-2 mb-2">
            <c.icon size={14} className={c.color} />
            <span className="text-xs text-muted-foreground">{c.label}</span>
          </div>
          <p className="text-2xl font-semibold">
            {c.value}
            {c.total != null && <span className="text-sm text-muted-foreground font-normal"> / {c.total}</span>}
          </p>
        </div>
      ))}
    </div>
  )
}

function WorkspacesTab() {
  const [workspaces, setWorkspaces] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [impersonated, setImpersonated] = useState<Record<string, string>>({})
  const [copied, setCopied] = useState('')

  useEffect(() => {
    adminFetch('/workspaces').then(d => { setWorkspaces(d?.data || []); setLoading(false) })
  }, [])

  const handleStateChange = async (wsId: string, state: string) => {
    await fetch(`/api/v1/admin/workspaces/${wsId}/state`, {
      method: 'POST', headers: headers(), body: JSON.stringify({ state }),
    })
    setWorkspaces(ws => ws.map(w => w.id === wsId ? { ...w, state } : w))
  }

  const handleImpersonate = async (wsId: string) => {
    const res = await fetch(`/api/v1/admin/workspaces/${wsId}/impersonate`, {
      method: 'POST', headers: headers(),
    })
    const data = await res.json()
    if (data?.data?.api_key) {
      setImpersonated(prev => ({ ...prev, [wsId]: data.data.api_key }))
    }
  }

  const handleCopy = (key: string) => {
    navigator.clipboard.writeText(key)
    setCopied(key)
    setTimeout(() => setCopied(''), 2000)
  }

  if (loading) return <div className="text-sm text-muted-foreground">Loading...</div>

  return (
    <div className="rounded-lg border border-border bg-card divide-y divide-border">
      {workspaces.map((ws) => (
        <div key={ws.id} className="px-4 py-3 space-y-2">
          <div className="flex items-center gap-4">
            <div className="flex-1">
              <p className="text-sm font-medium">{ws.name}</p>
              <p className="text-xs text-muted-foreground font-mono">{ws.slug} · {ws.id}</p>
            </div>
            <span className={`text-xs px-2 py-0.5 rounded-full ${
              ws.state === 'active' ? 'bg-emerald-500/10 text-emerald-400' :
              ws.state === 'suspended' ? 'bg-amber-500/10 text-amber-400' :
              'bg-red-500/10 text-red-400'
            }`}>{ws.state}</span>
            <select
              value={ws.state}
              onChange={(e) => handleStateChange(ws.id, e.target.value)}
              className="text-xs px-2 py-1 rounded border border-border bg-background"
            >
              <option value="active">active</option>
              <option value="suspended">suspended</option>
              <option value="archived">archived</option>
            </select>
            <button onClick={() => handleImpersonate(ws.id)}
              className="px-2 py-1 rounded text-xs text-amber-400 hover:bg-amber-500/10 transition-colors">
              Impersonate
            </button>
          </div>
          {impersonated[ws.id] && (
            <div className="flex items-center gap-2 bg-amber-500/5 border border-amber-500/20 rounded p-2">
              <code className="flex-1 text-xs font-mono text-amber-300 truncate">{impersonated[ws.id]}</code>
              <button onClick={() => handleCopy(impersonated[ws.id])} className="p-1">
                {copied === impersonated[ws.id] ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} className="text-muted-foreground" />}
              </button>
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

function UsersTab() {
  const [users, setUsers] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    adminFetch('/users').then(d => { setUsers(d?.data || []); setLoading(false) })
  }, [])

  const togglePlatformAdmin = async (userId: string, current: boolean) => {
    await fetch(`/api/v1/admin/users/${userId}/platform-admin`, {
      method: 'POST', headers: headers(), body: JSON.stringify({ is_platform_admin: !current }),
    })
    setUsers(us => us.map(u => u.id === userId ? { ...u, is_platform_admin: !current } : u))
  }

  if (loading) return <div className="text-sm text-muted-foreground">Loading...</div>

  return (
    <div className="rounded-lg border border-border bg-card divide-y divide-border">
      {users.map((u) => (
        <div key={u.id} className="flex items-center gap-4 px-4 py-3">
          <div className="w-8 h-8 rounded-full bg-indigo-500/20 flex items-center justify-center">
            <span className="text-xs font-medium text-indigo-400">{(u.name || u.email)[0].toUpperCase()}</span>
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium">{u.name}</p>
            <p className="text-xs text-muted-foreground">{u.email} · {u.auth_provider}</p>
          </div>
          {u.is_platform_admin && (
            <span className="text-xs px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400">Platform Admin</span>
          )}
          <span className="text-xs text-muted-foreground">{u.last_login_at ? formatTimeAgo(u.last_login_at) : 'never'}</span>
          <button
            onClick={() => togglePlatformAdmin(u.id, u.is_platform_admin)}
            className={`px-2 py-1 rounded text-xs transition-colors ${
              u.is_platform_admin
                ? 'text-red-400 hover:bg-red-500/10'
                : 'text-amber-400 hover:bg-amber-500/10'
            }`}
          >
            {u.is_platform_admin ? 'Revoke Admin' : 'Make Admin'}
          </button>
        </div>
      ))}
    </div>
  )
}

function InfraAdminTab() {
  const [servers, setServers] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    adminFetch('/infra/servers').then(d => {
      setServers(d?.data || [])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  const stateColor: Record<string, string> = {
    provisioning: 'bg-amber-400', ready: 'bg-blue-400', active: 'bg-emerald-400',
    idle: 'bg-zinc-400', destroying: 'bg-red-400', destroyed: 'bg-zinc-600',
  }

  const filtered = filter
    ? servers.filter((s: any) => s.workspace_id?.includes(filter) || s.name?.includes(filter))
    : servers

  const totalServers = servers.length
  const totalContainers = servers.reduce((sum: number, s: any) => sum + (s.containers_running || 0), 0)
  const totalCapacity = servers.reduce((sum: number, s: any) => sum + (s.max_containers || 4), 0)
  const totalCost = servers.reduce((sum: number, s: any) => sum + (parseFloat(s.monthly_cost) || 4.5), 0)
  const utilization = totalCapacity > 0 ? Math.round((totalContainers / totalCapacity) * 100) : 0

  if (loading) return <div className="text-sm text-muted-foreground">Loading...</div>

  return (
    <div className="space-y-4">
      {/* Summary cards */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
        <div className="rounded-lg border border-border bg-card p-3 text-center">
          <p className="text-2xl font-semibold">{totalServers}</p>
          <p className="text-xs text-muted-foreground">Servers</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-3 text-center">
          <p className="text-2xl font-semibold">{totalContainers} / {totalCapacity}</p>
          <p className="text-xs text-muted-foreground">Containers</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-3 text-center">
          <p className="text-2xl font-semibold">{utilization}%</p>
          <p className="text-xs text-muted-foreground">Utilization</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-3 text-center">
          <p className="text-2xl font-semibold">{'\u20AC'}{(totalCost * 2).toFixed(0)}</p>
          <p className="text-xs text-muted-foreground">Monthly cost</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-3 text-center">
          <p className="text-2xl font-semibold">{'\u20AC'}{totalCost.toFixed(0)}</p>
          <p className="text-xs text-muted-foreground">Hetzner cost</p>
        </div>
      </div>

      {/* Filter */}
      <input
        value={filter} onChange={(e) => setFilter(e.target.value)}
        placeholder="Filter by workspace or server name..."
        className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm"
      />

      {/* Server list */}
      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {filtered.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No servers{filter ? ' matching filter' : ''}.</div>
        ) : filtered.map((s: any) => (
          <div key={s.id} className="flex items-center gap-4 px-4 py-3">
            <span className={cn('w-2 h-2 rounded-full', stateColor[s.state] || 'bg-zinc-500')} />
            <div className="flex-1">
              <p className="text-sm font-medium">{s.name || s.id}</p>
              <p className="text-xs text-muted-foreground font-mono">
                {s.ip_address || 'pending'} · {s.server_type || 'cx22'} · {s.containers_running || 0}/{s.max_containers || 4} containers
              </p>
            </div>
            <span className="text-xs text-muted-foreground font-mono">{s.workspace_id?.substring(0, 12)}...</span>
            <span className={cn('text-xs px-1.5 py-0.5 rounded font-mono',
              s.state === 'active' ? 'bg-emerald-500/10 text-emerald-400' :
              s.state === 'idle' ? 'bg-zinc-500/10 text-zinc-400' :
              s.state === 'provisioning' ? 'bg-amber-500/10 text-amber-400' :
              'bg-zinc-500/10 text-zinc-400'
            )}>
              {s.state}
            </span>
            <span className="text-xs text-muted-foreground">{s.created_at ? formatTimeAgo(s.created_at) : ''}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
