import { useState, useEffect } from 'react'
import { Plus, Trash2, Copy, Check, Server, Key, Users, Webhook, Shield, UserPlus, Lock, Eye, EyeOff, Pencil, HardDrive, Loader2 } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { api } from '@/api/client'
import { formatTimeAgo } from '@/lib/utils'
import { cn } from '@/lib/utils'

type Tab = 'api-keys' | 'workers' | 'members' | 'webhooks' | 'credentials' | 'secrets' | 'infra'

export function Settings() {
  // Pre-select tab from URL (e.g., /settings/workers)
  const pathTab = window.location.pathname.split('/')[2] as Tab | undefined
  const [tab, setTab] = useState<Tab>(pathTab && ['api-keys', 'workers', 'members', 'webhooks', 'credentials', 'secrets', 'infra'].includes(pathTab) ? pathTab : 'api-keys')

  const tabs: { id: Tab; label: string; icon: React.ElementType }[] = [
    { id: 'api-keys', label: 'API Keys', icon: Key },
    { id: 'workers', label: 'Workers', icon: Server },
    { id: 'secrets', label: 'Secrets', icon: Lock },
    { id: 'members', label: 'Members', icon: Users },
    { id: 'webhooks', label: 'Webhooks', icon: Webhook },
    { id: 'credentials', label: 'Credentials', icon: Shield },
    { id: 'infra', label: 'Infrastructure', icon: HardDrive },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground mt-1">Manage workspace configuration</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              'flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors',
              tab === t.id
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            <t.icon size={14} />
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'api-keys' && <APIKeysTab />}
      {tab === 'workers' && <WorkersTab />}
      {tab === 'secrets' && <SecretsTab />}
      {tab === 'members' && <MembersTab />}
      {tab === 'webhooks' && <WebhooksTab />}
      {tab === 'credentials' && <CredentialsTab />}
      {tab === 'infra' && <InfraTab />}
    </div>
  )
}

// ============================================================
// API Keys Tab
// ============================================================

function APIKeysTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['api-keys'], queryFn: api.listApiKeys })
  const keys = data?.data || []

  const [showCreate, setShowCreate] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [newKeyRole, setNewKeyRole] = useState('operator')
  const [createdKey, setCreatedKey] = useState('')
  const [copied, setCopied] = useState(false)

  const createMutation = useMutation({
    mutationFn: (data: any) => api.createApiKey(data),
    onSuccess: (res) => {
      setCreatedKey(res.data.key)
      qc.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteApiKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })

  const handleCreate = () => {
    createMutation.mutate({ name: newKeyName || 'API Key', role: newKeyRole })
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(createdKey)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{keys.length} API keys</span>
        <button onClick={() => { setShowCreate(true); setCreatedKey('') }} className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
          <Plus size={14} /> New Key
        </button>
      </div>

      {/* Create dialog */}
      {showCreate && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          {createdKey ? (
            <div className="space-y-3">
              <p className="text-sm font-medium text-emerald-400">API key created!</p>
              <div className="flex gap-2">
                <code className="flex-1 px-3 py-2 rounded bg-[#0a0a0c] text-xs font-mono text-zinc-300 overflow-hidden">{createdKey}</code>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button type="button" onClick={handleCopy} className="px-3 py-2 rounded bg-muted hover:bg-muted/80 transition-colors">
                      {copied ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>{copied ? 'Copied!' : 'Copy to clipboard'}</TooltipContent>
                </Tooltip>
              </div>
              <p className="text-xs text-amber-400">Save this key now. It will not be shown again.</p>
              <button onClick={() => { setShowCreate(false); setNewKeyName('') }} className="text-xs text-muted-foreground hover:text-foreground">Close</button>
            </div>
          ) : (
            <div className="space-y-3">
              <input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="Key name" className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
              <select value={newKeyRole} onChange={(e) => setNewKeyRole(e.target.value)} className="px-3 py-1.5 rounded-md border border-border bg-background text-sm">
                <option value="admin">Admin</option>
                <option value="operator">Operator</option>
                <option value="viewer">Viewer</option>
              </select>
              <div className="flex gap-2">
                <button onClick={handleCreate} className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm">Create</button>
                <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Key list */}
      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {isLoading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : keys.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No API keys.</div>
        ) : keys.map((key: any) => (
          <div key={key.id} className="flex items-center gap-4 px-4 py-3">
            <Key size={14} className="text-muted-foreground" />
            <div className="flex-1">
              <p className="text-sm font-medium">{key.name}</p>
              <p className="text-xs text-muted-foreground font-mono">{key.key_prefix}···· · {key.role}</p>
            </div>
            <span className="text-xs text-muted-foreground">{key.last_used_at ? formatTimeAgo(key.last_used_at) : 'never used'}</span>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => { if (confirm('Revoke this key?')) deleteMutation.mutate(key.id) }} className="p-1.5 rounded hover:bg-red-500/10 transition-colors">
                  <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
                </button>
              </TooltipTrigger>
              <TooltipContent>Revoke key</TooltipContent>
            </Tooltip>
          </div>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Workers Tab
// ============================================================

function WorkersTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['workers'], queryFn: api.listWorkers })
  const workers = data?.data || []

  const [showCreate, setShowCreate] = useState(false)
  const [workerName, setWorkerName] = useState('')
  const [enrollToken, setEnrollToken] = useState('')

  const createMutation = useMutation({
    mutationFn: (data: any) => api.createWorker(data),
    onSuccess: (res) => {
      setEnrollToken(res.data.enrollment_token)
      qc.invalidateQueries({ queryKey: ['workers'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteWorker(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workers'] }),
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{workers.length} workers</span>
        <button onClick={() => { setShowCreate(true); setEnrollToken('') }} className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
          <Plus size={14} /> New Worker
        </button>
      </div>

      {showCreate && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          {enrollToken ? (
            <div className="space-y-3">
              <p className="text-sm font-medium text-emerald-400">Worker created!</p>

              <div>
                <p className="text-xs font-medium text-muted-foreground mb-1">1. Download the worker binary</p>
                <a href="https://github.com/darkspock/croncontrol/releases" target="_blank" rel="noopener"
                  className="inline-flex items-center gap-1.5 text-xs text-indigo-400 hover:text-indigo-300 transition-colors">
                  github.com/darkspock/croncontrol/releases →
                </a>
              </div>

              <div>
                <p className="text-xs font-medium text-muted-foreground mb-1">2. Run the worker with this enrollment token</p>
                <code className="block px-3 py-2 rounded bg-[#0a0a0c] text-xs font-mono text-zinc-300 overflow-auto">
                  croncontrol-worker --url {window.location.origin} --credential {enrollToken}
                </code>
              </div>

              <div>
                <p className="text-xs text-muted-foreground">
                  The token expires in 1 hour. See the <a href="https://github.com/darkspock/croncontrol/blob/main/docs/guides/worker-setup.md" target="_blank" rel="noopener" className="text-indigo-400 hover:underline">Worker Setup Guide</a> for full instructions.
                </p>
              </div>

              <button type="button" onClick={() => setShowCreate(false)} className="text-xs text-muted-foreground hover:text-foreground">Close</button>
            </div>
          ) : (
            <div className="space-y-3">
              <input value={workerName} onChange={(e) => setWorkerName(e.target.value)} placeholder="Worker name" className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
              <div className="flex gap-2">
                <button onClick={() => createMutation.mutate({ name: workerName })} className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm">Create</button>
                <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
              </div>
            </div>
          )}
        </div>
      )}

      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {isLoading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : workers.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No workers. Workers execute tasks inside your infrastructure.</div>
        ) : workers.map((w: any) => (
          <div key={w.id} className="flex items-center gap-4 px-4 py-3">
            <span className={cn('w-2 h-2 rounded-full', w.status === 'online' ? 'bg-emerald-400' : w.status === 'unhealthy' ? 'bg-amber-400' : 'bg-zinc-500')} />
            <div className="flex-1">
              <p className="text-sm font-medium">{w.name}</p>
              <p className="text-xs text-muted-foreground font-mono">{w.status} · max {w.max_concurrency} concurrent</p>
            </div>
            {w.last_heartbeat_at && <span className="text-xs text-muted-foreground">heartbeat {formatTimeAgo(w.last_heartbeat_at)}</span>}
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => { if (confirm('Delete this worker?')) deleteMutation.mutate(w.id) }} className="p-1.5 rounded hover:bg-red-500/10 transition-colors">
                  <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
                </button>
              </TooltipTrigger>
              <TooltipContent>Delete worker</TooltipContent>
            </Tooltip>
          </div>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Secrets Tab
// ============================================================

function SecretsTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['secrets'], queryFn: api.listSecrets })
  const secrets = data?.data || []

  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [value, setValue] = useState('')
  const [editingName, setEditingName] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')
  const [revealed, setRevealed] = useState<Set<string>>(new Set())

  const createMutation = useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) => api.createSecret(name, value),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['secrets'] })
      setShowCreate(false)
      setName('')
      setValue('')
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) => api.updateSecret(name, value),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['secrets'] })
      setEditingName(null)
      setEditValue('')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.deleteSecret(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['secrets'] }),
  })

  const toggleReveal = (n: string) => {
    setRevealed(prev => {
      const next = new Set(prev)
      next.has(n) ? next.delete(n) : next.add(n)
      return next
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <span className="text-sm font-medium">{secrets.length} secrets</span>
          <p className="text-xs text-muted-foreground mt-0.5">Encrypted with AES-256-GCM. Injected as env vars into AgentNodes.</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
          <Plus size={14} /> New Secret
        </button>
      </div>

      {showCreate && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          <input value={name} onChange={(e) => setName(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))} placeholder="SECRET_NAME" className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono" />
          <input value={value} onChange={(e) => setValue(e.target.value)} placeholder="Secret value" type="password" className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
          <p className="text-xs text-muted-foreground">Name must be uppercase with underscores only (e.g., API_TOKEN, DB_PASSWORD).</p>
          <div className="flex gap-2">
            <button onClick={() => createMutation.mutate({ name, value })} disabled={!name || !value} className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm disabled:opacity-50">Create</button>
            <button onClick={() => { setShowCreate(false); setName(''); setValue('') }} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
          </div>
        </div>
      )}

      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {isLoading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : secrets.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No secrets. Secrets are encrypted values injected into orchestra AgentNodes as environment variables.</div>
        ) : secrets.map((s: any) => (
          <div key={s.name} className="flex items-center gap-4 px-4 py-3">
            <Lock size={14} className="text-muted-foreground" />
            <div className="flex-1">
              <p className="text-sm font-mono font-medium">{s.name}</p>
              {editingName === s.name ? (
                <div className="flex gap-2 mt-2">
                  <input value={editValue} onChange={(e) => setEditValue(e.target.value)} placeholder="New value" type="password" className="flex-1 px-2 py-1 rounded border border-border bg-background text-xs" />
                  <button onClick={() => updateMutation.mutate({ name: s.name, value: editValue })} className="px-2 py-1 rounded bg-indigo-500 text-white text-xs">Save</button>
                  <button onClick={() => setEditingName(null)} className="px-2 py-1 rounded border border-border text-xs">Cancel</button>
                </div>
              ) : (
                <p className="text-xs text-muted-foreground font-mono">
                  {revealed.has(s.name) ? (s.value || '••••••••') : '••••••••'}
                </p>
              )}
            </div>
            <span className="text-xs text-muted-foreground">{s.updated_at ? formatTimeAgo(s.updated_at) : ''}</span>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => toggleReveal(s.name)} className="p-1.5 rounded hover:bg-muted transition-colors">
                  {revealed.has(s.name) ? <EyeOff size={14} className="text-muted-foreground" /> : <Eye size={14} className="text-muted-foreground" />}
                </button>
              </TooltipTrigger>
              <TooltipContent>{revealed.has(s.name) ? 'Hide' : 'Reveal'}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => { setEditingName(s.name); setEditValue('') }} className="p-1.5 rounded hover:bg-muted transition-colors">
                  <Pencil size={14} className="text-muted-foreground" />
                </button>
              </TooltipTrigger>
              <TooltipContent>Update value</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => { if (confirm(`Delete secret "${s.name}"?`)) deleteMutation.mutate(s.name) }} className="p-1.5 rounded hover:bg-red-500/10 transition-colors">
                  <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
                </button>
              </TooltipTrigger>
              <TooltipContent>Delete secret</TooltipContent>
            </Tooltip>
          </div>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Members Tab
// ============================================================

function MembersTab() {
  const { data, isLoading } = useQuery({ queryKey: ['members'], queryFn: api.listMembers })
  const members = data?.data || []
  const [showInvite, setShowInvite] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('viewer')
  const [inviteStatus, setInviteStatus] = useState('')
  const qc = useQueryClient()

  const handleInvite = async () => {
    try {
      const res = await fetch('/api/v1/users/invite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
        body: JSON.stringify({ email: inviteEmail, role: inviteRole }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error?.message || 'Failed')
      setInviteStatus(data.data.status === 'member_added' ? 'Member added!' : 'Invitation sent!')
      qc.invalidateQueries({ queryKey: ['members'] })
      setTimeout(() => { setShowInvite(false); setInviteStatus('') }, 2000)
    } catch (err: any) {
      setInviteStatus(err.message)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{members.length} members</span>
        <button onClick={() => setShowInvite(true)} className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
          <UserPlus size={14} /> Invite
        </button>
      </div>

      {showInvite && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          {inviteStatus ? (
            <div><p className="text-sm text-emerald-400">{inviteStatus}</p></div>
          ) : (
            <>
              <input value={inviteEmail} onChange={(e) => setInviteEmail(e.target.value)} placeholder="Email address" type="email"
                className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
              <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value)}
                className="px-3 py-1.5 rounded-md border border-border bg-background text-sm">
                <option value="admin">Admin</option>
                <option value="operator">Operator</option>
                <option value="viewer">Viewer</option>
              </select>
              <div className="flex gap-2">
                <button onClick={handleInvite} className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm">Invite</button>
                <button onClick={() => setShowInvite(false)} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
              </div>
            </>
          )}
        </div>
      )}
      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {isLoading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : members.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No members.</div>
        ) : members.map((m: any) => (
          <div key={m.user_id || m.id} className="flex items-center gap-4 px-4 py-3">
            <div className="w-7 h-7 rounded-full bg-indigo-500/20 flex items-center justify-center">
              <span className="text-xs font-medium text-indigo-400">{(m.user_name || m.email || '?')[0].toUpperCase()}</span>
            </div>
            <div className="flex-1">
              <p className="text-sm font-medium">{m.user_name || m.email}</p>
              <p className="text-xs text-muted-foreground">{m.email}</p>
            </div>
            <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">{m.role}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Webhooks Tab
// ============================================================

function WebhooksTab() {
  const [subs, setSubs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [events, setEvents] = useState('run.*,job.*')
  const [testResult, setTestResult] = useState<Record<string, string>>({})

  useEffect(() => {
    fetch('/api/v1/webhook-subscriptions', {
      headers: { 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
    }).then(r => r.ok ? r.json() : { data: [] }).then(d => {
      setSubs(d.data || [])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  const handleCreate = async () => {
    const res = await fetch('/api/v1/webhook-subscriptions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
      body: JSON.stringify({ url, secret, event_types: events.split(',').map(s => s.trim()) }),
    })
    if (res.ok) {
      const data = await res.json()
      setSubs([...subs, data.data])
      setShowCreate(false)
      setUrl('')
      setSecret('')
    }
  }

  const handleTest = async (subId: string) => {
    setTestResult(prev => ({ ...prev, [subId]: 'testing...' }))
    try {
      const res = await fetch(`/api/v1/webhook-subscriptions/${subId}/test`, {
        method: 'POST',
        headers: { 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
      })
      const data = await res.json()
      setTestResult(prev => ({ ...prev, [subId]: data.data?.delivered ? 'delivered' : `failed (${data.data?.status_code || data.data?.error})` }))
    } catch {
      setTestResult(prev => ({ ...prev, [subId]: 'error' }))
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{subs.length} webhook subscriptions</span>
        <button onClick={() => setShowCreate(true)} className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
          <Plus size={14} /> New Webhook
        </button>
      </div>

      {showCreate && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="Webhook URL (https://...)" type="url"
            className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
          <input value={secret} onChange={(e) => setSecret(e.target.value)} placeholder="HMAC secret"
            className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono" />
          <input value={events} onChange={(e) => setEvents(e.target.value)} placeholder="Event types (run.*, job.*)"
            className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm" />
          <p className="text-xs text-muted-foreground">Events: run.completed, run.failed, run.hung, run.killed, job.completed, job.failed, worker.offline, etc.</p>
          <div className="flex gap-2">
            <button onClick={handleCreate} className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm">Create</button>
            <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
          </div>
        </div>
      )}

      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {loading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : subs.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No webhook subscriptions. Webhooks deliver event notifications to your endpoints.</div>
        ) : subs.map((sub: any) => (
          <div key={sub.id} className="flex items-center gap-4 px-4 py-3">
            <Webhook size={14} className="text-muted-foreground" />
            <div className="flex-1">
              <p className="text-sm font-mono">{sub.url}</p>
              <p className="text-xs text-muted-foreground">{(sub.event_types || []).join(', ') || 'all events'}</p>
            </div>
            <span className={cn('text-xs px-1.5 py-0.5 rounded', sub.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400')}>
              {sub.enabled ? 'active' : 'disabled'}
            </span>
            <button onClick={() => handleTest(sub.id)} className="px-2 py-1 rounded text-xs text-indigo-400 hover:bg-indigo-500/10 transition-colors">
              {testResult[sub.id] || 'Test'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Credentials Tab (SSH, SSM, K8s)
// ============================================================

function CredentialsTab() {
  return (
    <div className="space-y-6">
      <CredentialSection
        title="SSH Credentials"
        description="Key-based authentication for SSH execution targets"
        type="ssh"
        endpoint="/api/v1/ssh-credentials"
      />
      <CredentialSection
        title="SSM Profiles"
        description="AWS Systems Manager connection profiles"
        type="ssm"
        endpoint="/api/v1/ssm-profiles"
      />
      <CredentialSection
        title="K8s Clusters"
        description="Kubernetes cluster connection configurations"
        type="k8s"
        endpoint="/api/v1/k8s-clusters"
      />
    </div>
  )
}

// ============================================================
// Infrastructure Tab
// ============================================================

function InfraTab() {
  const qc = useQueryClient()
  const { data, isLoading, error } = useQuery({
    queryKey: ['infra-servers'],
    queryFn: api.listInfraServers,
    retry: false,
  })
  const servers = data?.data || []
  const notConfigured = (error as any)?.status === 501

  const [provisioning, setProvisioning] = useState(false)

  const provisionMutation = useMutation({
    mutationFn: () => api.provisionServer(),
    onMutate: () => setProvisioning(true),
    onSettled: () => setProvisioning(false),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['infra-servers'] }),
  })

  const destroyMutation = useMutation({
    mutationFn: (id: string) => api.destroyServer(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['infra-servers'] }),
  })

  const stateColor: Record<string, string> = {
    provisioning: 'bg-amber-400',
    ready: 'bg-blue-400',
    active: 'bg-emerald-400',
    idle: 'bg-zinc-400',
    destroying: 'bg-red-400',
    destroyed: 'bg-zinc-600',
  }

  const activeServers = servers.filter((s: any) => !['destroyed'].includes(s.state))
  const totalCost = activeServers.reduce((sum: number, s: any) => sum + (parseFloat(s.monthly_cost) || 4.5), 0)

  if (notConfigured) {
    return (
      <div className="space-y-4">
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <HardDrive size={32} className="mx-auto text-muted-foreground mb-3" />
          <p className="text-sm font-medium">Infrastructure not configured</p>
          <p className="text-xs text-muted-foreground mt-1">Set <code className="font-mono text-xs">infra.enabled: true</code> and configure Hetzner API credentials in config.yaml to enable auto-provisioned servers.</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <span className="text-sm font-medium">{activeServers.length} servers</span>
          <p className="text-xs text-muted-foreground mt-0.5">Auto-provisioned Hetzner CX22 nodes for container execution.</p>
        </div>
        <button
          onClick={() => provisionMutation.mutate()}
          disabled={provisioning}
          className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors disabled:opacity-50"
        >
          {provisioning ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
          Provision Server
        </button>
      </div>

      {/* Cost summary */}
      {activeServers.length > 0 && (
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-lg border border-border bg-card p-3 text-center">
            <p className="text-2xl font-semibold">{activeServers.length}</p>
            <p className="text-xs text-muted-foreground">Active servers</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-3 text-center">
            <p className="text-2xl font-semibold">{activeServers.reduce((sum: number, s: any) => sum + (s.containers_running || 0), 0)} / {activeServers.reduce((sum: number, s: any) => sum + (s.max_containers || 4), 0)}</p>
            <p className="text-xs text-muted-foreground">Containers</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-3 text-center">
            <p className="text-2xl font-semibold">{'\u20AC'}{(totalCost * 2).toFixed(0)}/mo</p>
            <p className="text-xs text-muted-foreground">Estimated cost</p>
          </div>
        </div>
      )}

      {/* Server list */}
      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {isLoading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : servers.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No servers provisioned. Servers are created automatically when an orchestra needs container execution.</div>
        ) : servers.map((s: any) => (
          <div key={s.id} className="flex items-center gap-4 px-4 py-3">
            <span className={cn('w-2 h-2 rounded-full', stateColor[s.state] || 'bg-zinc-500')} />
            <div className="flex-1">
              <p className="text-sm font-medium">{s.name || s.id}</p>
              <p className="text-xs text-muted-foreground font-mono">
                {s.ip_address || 'pending'} · {s.server_type || 'cx22'} · {s.containers_running || 0}/{s.max_containers || 4} containers
              </p>
            </div>
            <span className={cn('text-xs px-1.5 py-0.5 rounded font-mono',
              s.state === 'active' ? 'bg-emerald-500/10 text-emerald-400' :
              s.state === 'idle' ? 'bg-zinc-500/10 text-zinc-400' :
              s.state === 'provisioning' ? 'bg-amber-500/10 text-amber-400' :
              s.state === 'destroying' ? 'bg-red-500/10 text-red-400' :
              'bg-zinc-500/10 text-zinc-400'
            )}>
              {s.state}
            </span>
            <span className="text-xs text-muted-foreground">{s.created_at ? formatTimeAgo(s.created_at) : ''}</span>
            {s.state !== 'destroyed' && s.state !== 'destroying' && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    onClick={() => { if (confirm(`Destroy server "${s.name || s.id}"? This will terminate all running containers.`)) destroyMutation.mutate(s.id) }}
                    className="p-1.5 rounded hover:bg-red-500/10 transition-colors"
                  >
                    <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
                  </button>
                </TooltipTrigger>
                <TooltipContent>Destroy server</TooltipContent>
              </Tooltip>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function CredentialSection({ title, description, type, endpoint }: { title: string; description: string; type: string; endpoint: string }) {
  const [items, setItems] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(endpoint, {
      headers: { 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
    }).then(r => r.ok ? r.json() : { data: [] }).then(d => {
      setItems(d.data || [])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  const handleDelete = async (id: string) => {
    if (!confirm(`Delete this ${type} credential?`)) return
    await fetch(`${endpoint}/${id}`, {
      method: 'DELETE',
      headers: { 'X-API-Key': localStorage.getItem('cc_api_key') || '' },
    })
    setItems(items.filter(i => i.id !== id))
  }

  return (
    <div className="space-y-3">
      <div>
        <h3 className="text-sm font-medium">{title}</h3>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <div className="rounded-lg border border-border bg-card divide-y divide-border">
        {loading ? (
          <div className="p-4 text-sm text-muted-foreground">Loading...</div>
        ) : items.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No {type} credentials configured.</div>
        ) : items.map((item: any) => (
          <div key={item.id} className="flex items-center gap-4 px-4 py-3">
            <Shield size={14} className="text-muted-foreground" />
            <div className="flex-1">
              <p className="text-sm font-medium">{item.name}</p>
              <p className="text-xs text-muted-foreground font-mono">
                {type === 'ssh' && `${item.fingerprint || ''}${item.username ? ` · ${item.username}` : ''}`}
                {type === 'ssm' && `${item.region || ''}${item.role_arn ? ` · ${item.role_arn}` : ''}`}
                {type === 'k8s' && `${item.default_namespace || 'default'}`}
              </p>
            </div>
            <span className="text-xs text-muted-foreground">{item.created_at ? formatTimeAgo(item.created_at) : ''}</span>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" onClick={() => handleDelete(item.id)} className="p-1.5 rounded hover:bg-red-500/10 transition-colors">
                  <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
                </button>
              </TooltipTrigger>
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          </div>
        ))}
      </div>
    </div>
  )
}
