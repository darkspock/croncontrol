import { useState } from 'react'
import { ArrowLeft, Play, Pause, Trash2 } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { TargetIcon } from '@/components/domain/target-icon'
import { SCHEDULE_LABELS, ORIGIN_LABELS } from '@/lib/constants'
import { formatTimeAgo, formatDuration, cn } from '@/lib/utils'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

type Tab = 'overview' | 'runs' | 'config'

interface ProcessDetailProps {
  processId: string
}

export function ProcessDetail({ processId }: ProcessDetailProps) {
  const [tab, setTab] = useState<Tab>('overview')
  const qc = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['process', processId],
    queryFn: () => api.getProcess(processId),
  })
  const { data: runsData } = useQuery({
    queryKey: ['runs', `process_id=${processId}`],
    queryFn: () => api.listRuns(`process_id=${processId}`),
  })

  const proc = data?.data
  const runs = runsData?.data || []

  const trigger = useMutation({
    mutationFn: () => api.triggerProcess(processId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['runs'] }),
  })
  const pause = useMutation({
    mutationFn: () => api.pauseProcess(processId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['process', processId] }),
  })
  const resume = useMutation({
    mutationFn: () => api.resumeProcess(processId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['process', processId] }),
  })
  const deleteMut = useMutation({
    mutationFn: () => api.deleteProcess(processId),
    onSuccess: () => {
      window.history.pushState(null, '', '/processes')
      window.dispatchEvent(new PopStateEvent('popstate'))
    },
  })

  const goBack = () => {
    window.history.pushState(null, '', '/processes')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  if (isLoading) return <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
  if (!proc) return <div className="p-8 text-center text-sm text-muted-foreground">Process not found</div>

  const completedRuns = runs.filter((r: any) => r.state === 'completed').length
  const failedRuns = runs.filter((r: any) => r.state === 'failed').length
  const successRate = runs.length > 0 ? Math.round((completedRuns / runs.length) * 100) : 0

  const tabs: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'runs', label: `Runs (${runs.length})` },
    { id: 'config', label: 'Configuration' },
  ]

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <button onClick={goBack} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
            <ArrowLeft size={12} /> Processes
          </button>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold tracking-tight">{proc.name}</h1>
            <span className={cn('text-xs font-mono px-1.5 py-0.5 rounded', proc.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-500/10 text-zinc-400')}>
              {proc.enabled ? 'enabled' : 'disabled'}
            </span>
          </div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <TargetIcon method={proc.execution_method} size={12} />
            <span>{proc.execution_method}</span>
            <span>·</span>
            <span className="font-mono">{SCHEDULE_LABELS[proc.schedule_type]}: {proc.schedule || proc.delay_duration || 'on demand'}</span>
            <span>·</span>
            <span>runtime: {proc.runtime}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => trigger.mutate()} className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors">
            <Play size={13} /> Trigger
          </button>
          <button onClick={() => proc.enabled ? pause.mutate() : resume.mutate()}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-muted/50 transition-colors">
            <Pause size={13} /> {proc.enabled ? 'Pause' : 'Resume'}
          </button>
          <button onClick={() => { if (confirm(`Delete "${proc.name}"?`)) deleteMut.mutate() }}
            className="p-1.5 rounded-md border border-border hover:bg-red-500/10 transition-colors">
            <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {tabs.map((t) => (
          <button key={t.id} onClick={() => setTab(t.id)}
            className={cn('px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors',
              tab === t.id ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-muted-foreground hover:text-foreground')}>
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {tab === 'overview' && (
        <div className="space-y-4">
          {/* Stats */}
          <div className="grid grid-cols-4 gap-4">
            <StatCard label="Total Runs" value={runs.length.toString()} />
            <StatCard label="Completed" value={completedRuns.toString()} color="text-emerald-400" />
            <StatCard label="Failed" value={failedRuns.toString()} color="text-red-400" />
            <StatCard label="Success Rate" value={`${successRate}%`} color="text-indigo-400" />
          </div>

          {/* Dependency */}
          {proc.depends_on_process_id && (
            <div className="rounded-lg border border-border bg-card p-4">
              <span className="text-sm font-medium mb-2 block">Dependency</span>
              <div className="flex items-center gap-2 text-xs">
                <span className="text-muted-foreground">Depends on:</span>
                <span className="font-mono text-foreground">{proc.depends_on_process_id.slice(0, 20)}...</span>
                <span className="font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">{proc.dependency_type}</span>
              </div>
            </div>
          )}

          {/* Tags */}
          {proc.tags && proc.tags.length > 0 && (
            <div className="flex gap-1.5">
              {proc.tags.map((tag: string) => (
                <span key={tag} className="text-xs font-mono px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-400">{tag}</span>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'runs' && (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
          {runs.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No runs yet. Click "Trigger" to create one.</div>
          ) : (
            <div className="divide-y divide-border">
              {runs.map((run: any) => (
                <div key={run.id}
                  onClick={() => { window.history.pushState(null, '', `/runs/${run.id}`); window.dispatchEvent(new PopStateEvent('popstate')) }}
                  className="flex items-center gap-4 px-4 py-2.5 hover:bg-muted/20 cursor-pointer transition-colors">
                  <StateBadge state={run.state} />
                  <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                    {ORIGIN_LABELS[run.origin] || run.origin}
                  </span>
                  <span className="text-xs font-mono text-muted-foreground">{run.attempt}/{run.max_attempts}</span>
                  <span className="flex-1" />
                  <span className="text-xs font-mono text-muted-foreground">{run.duration_ms ? formatDuration(run.duration_ms) : '—'}</span>
                  <span className="text-xs text-muted-foreground">{run.created_at ? formatTimeAgo(run.created_at) : '—'}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'config' && (
        <div className="rounded-lg border border-border bg-card p-4">
          <div className="grid grid-cols-2 gap-y-3 gap-x-8 text-xs">
            <Detail label="ID" value={proc.id} mono />
            <Detail label="Schedule Type" value={SCHEDULE_LABELS[proc.schedule_type]} />
            <Detail label="Schedule" value={proc.schedule || '—'} mono />
            <Detail label="Delay Duration" value={proc.delay_duration || '—'} mono />
            <Detail label="Timezone" value={proc.timezone || 'UTC'} />
            <Detail label="Miss Policy" value={proc.miss_policy || 'skip'} />
            <Detail label="Execution Method" value={proc.execution_method} />
            <Detail label="Runtime" value={proc.runtime} />
            <Detail label="Max Attempts" value={String(proc.max_attempts)} />
            <Detail label="Allow Parallel" value={proc.allow_parallel ? 'Yes' : 'No'} />
            <Detail label="Max Parallel" value={String(proc.max_parallel)} />
            <Detail label="On Overlap" value={proc.on_overlap} />
            <Detail label="Timeout Action" value={proc.timeout_action} />
            <Detail label="Created" value={new Date(proc.created_at).toLocaleString()} />
          </div>
          {proc.method_config && (
            <div className="mt-4">
              <span className="text-xs text-muted-foreground block mb-1">Method Config</span>
              <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-40">
                {typeof proc.method_config === 'string'
                  ? (() => { try { return JSON.stringify(JSON.parse(atob(proc.method_config)), null, 2) } catch { return proc.method_config } })()
                  : JSON.stringify(proc.method_config, null, 2)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <p className="text-xs text-muted-foreground uppercase tracking-wider">{label}</p>
      <p className={cn('text-xl font-mono font-semibold mt-1', color || 'text-foreground')}>{value}</p>
    </div>
  )
}

function Detail({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn('truncate max-w-60', mono && 'font-mono')}>{value}</span>
    </div>
  )
}
