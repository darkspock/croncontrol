import { useEffect, useState } from 'react'
import { ArrowLeft, Play, Pause, Trash2 } from 'lucide-react'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
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

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const
const HTTP_DISPATCH_MODES = ['sync', 'async_blind', 'async_tracked'] as const

function decodeJSONField(value: any) {
  if (!value) return null
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(atob(value))
  } catch {
    try {
      return JSON.parse(value)
    } catch {
      return value
    }
  }
}

function joinNumberList(value: any): string {
  return Array.isArray(value) ? value.join(', ') : ''
}

function joinStringList(value: any): string {
  return Array.isArray(value) ? value.join(', ') : ''
}

function parseNumberList(value: string): number[] | undefined {
  const items = value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  if (!items.length) return undefined
  return items.map((item) => {
    const parsed = Number(item)
    if (!Number.isInteger(parsed)) {
      throw new Error(`Invalid numeric value "${item}"`)
    }
    return parsed
  })
}

function parseStringList(value: string): string[] | undefined {
  const items = value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  return items.length ? items : undefined
}

export function ProcessDetail({ processId }: ProcessDetailProps) {
  const [tab, setTab] = useState<Tab>('overview')
  const [runtimeDraft, setRuntimeDraft] = useState<string>('direct')
  const [workerIdDraft, setWorkerIdDraft] = useState('')
  const [workerLabelsDraft, setWorkerLabelsDraft] = useState('')
  const [routingError, setRoutingError] = useState('')
  const [httpMethodDraft, setHttpMethodDraft] = useState('POST')
  const [dispatchModeDraft, setDispatchModeDraft] = useState('sync')
  const [urlDraft, setURLDraft] = useState('')
  const [acceptedStatusCodesDraft, setAcceptedStatusCodesDraft] = useState('')
  const [jobIDFieldDraft, setJobIDFieldDraft] = useState('')
  const [statusURLFieldDraft, setStatusURLFieldDraft] = useState('')
  const [statusURLTemplateDraft, setStatusURLTemplateDraft] = useState('')
  const [cancelURLFieldDraft, setCancelURLFieldDraft] = useState('')
  const [cancelURLTemplateDraft, setCancelURLTemplateDraft] = useState('')
  const [statusFieldDraft, setStatusFieldDraft] = useState('')
  const [pollMethodDraft, setPollMethodDraft] = useState('GET')
  const [cancelMethodDraft, setCancelMethodDraft] = useState('POST')
  const [runningValuesDraft, setRunningValuesDraft] = useState('')
  const [successValuesDraft, setSuccessValuesDraft] = useState('')
  const [failedValuesDraft, setFailedValuesDraft] = useState('')
  const [httpConfigError, setHTTPConfigError] = useState('')
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
  const methodConfig = decodeJSONField(proc?.method_config)

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
  const updateRouting = useMutation({
    mutationFn: (payload: any) => api.updateProcess(processId, payload),
    onSuccess: () => {
      setRoutingError('')
      qc.invalidateQueries({ queryKey: ['process', processId] })
    },
    onError: (err: any) => setRoutingError(err.message || 'Failed to update process routing'),
  })

  const goBack = () => {
    window.history.pushState(null, '', '/processes')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  useEffect(() => {
    if (!proc) return
    setRuntimeDraft(proc.runtime || 'direct')
    setWorkerIdDraft(proc.worker_id || '')
    setWorkerLabelsDraft(proc.worker_labels ? (typeof proc.worker_labels === 'string' ? proc.worker_labels : JSON.stringify(proc.worker_labels, null, 2)) : '')
    const cfg = decodeJSONField(proc.method_config)
    if (proc.execution_method === 'http' && cfg && typeof cfg === 'object') {
      setHttpMethodDraft(cfg.method || 'POST')
      setDispatchModeDraft(cfg.dispatch_mode || 'sync')
      setURLDraft(cfg.url || '')
      setAcceptedStatusCodesDraft(joinNumberList(cfg.accepted_status_codes))
      setJobIDFieldDraft(cfg.job_id_field || '')
      setStatusURLFieldDraft(cfg.status_url_field || '')
      setStatusURLTemplateDraft(cfg.status_url_template || '')
      setCancelURLFieldDraft(cfg.cancel_url_field || '')
      setCancelURLTemplateDraft(cfg.cancel_url_template || '')
      setStatusFieldDraft(cfg.status_field || '')
      setPollMethodDraft(cfg.poll_method || 'GET')
      setCancelMethodDraft(cfg.cancel_method || 'POST')
      setRunningValuesDraft(joinStringList(cfg.running_values))
      setSuccessValuesDraft(joinStringList(cfg.success_values))
      setFailedValuesDraft(joinStringList(cfg.failed_values))
    } else {
      setHttpMethodDraft('POST')
      setDispatchModeDraft('sync')
      setURLDraft('')
      setAcceptedStatusCodesDraft('')
      setJobIDFieldDraft('')
      setStatusURLFieldDraft('')
      setStatusURLTemplateDraft('')
      setCancelURLFieldDraft('')
      setCancelURLTemplateDraft('')
      setStatusFieldDraft('')
      setPollMethodDraft('GET')
      setCancelMethodDraft('POST')
      setRunningValuesDraft('')
      setSuccessValuesDraft('')
      setFailedValuesDraft('')
    }
    setHTTPConfigError('')
  }, [proc?.id, proc?.runtime, proc?.worker_id, proc?.worker_labels])

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
          <Tooltip>
            <TooltipTrigger asChild>
              <button type="button" onClick={() => { if (confirm(`Delete "${proc.name}"?`)) deleteMut.mutate() }}
                className="p-1.5 rounded-md border border-border hover:bg-red-500/10 transition-colors">
                <Trash2 size={14} className="text-muted-foreground hover:text-red-400" />
              </button>
            </TooltipTrigger>
            <TooltipContent>Delete process</TooltipContent>
          </Tooltip>
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
            <Detail label="Worker ID" value={proc.worker_id || '—'} mono />
            <Detail label="Max Attempts" value={String(proc.max_attempts)} />
            <Detail label="Allow Parallel" value={proc.allow_parallel ? 'Yes' : 'No'} />
            <Detail label="Max Parallel" value={String(proc.max_parallel)} />
            <Detail label="On Overlap" value={proc.on_overlap} />
            <Detail label="Timeout Action" value={proc.timeout_action} />
            <Detail label="Created" value={new Date(proc.created_at).toLocaleString()} />
          </div>
          <div className="mt-4 rounded-lg border border-border p-4 space-y-3">
            <div>
              <span className="text-sm font-medium">Routing</span>
              <p className="text-xs text-muted-foreground mt-1">Update runtime and worker selection without changing the rest of the process config.</p>
            </div>
            {routingError && <div className="rounded border border-red-500/20 bg-red-500/10 p-2 text-xs text-red-400">{routingError}</div>}
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <label className="text-xs text-muted-foreground">Runtime</label>
                <select value={runtimeDraft} onChange={(e) => setRuntimeDraft(e.target.value)} className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm">
                  <option value="direct">direct</option>
                  <option value="worker">worker</option>
                </select>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-muted-foreground">Worker ID</label>
                <input
                  value={workerIdDraft}
                  onChange={(e) => setWorkerIdDraft(e.target.value)}
                  placeholder="wrk_..."
                  className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono"
                />
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">Worker Labels JSON</label>
              <textarea
                value={workerLabelsDraft}
                onChange={(e) => setWorkerLabelsDraft(e.target.value)}
                rows={4}
                placeholder='["linux","prod"] or {"region":"eu-west-1"}'
                className="w-full px-2 py-2 rounded border border-border bg-background text-sm font-mono"
              />
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => {
                  try {
                    const payload: any = { runtime: runtimeDraft }
                    payload.worker_id = workerIdDraft.trim() || null
                    payload.worker_labels = workerLabelsDraft.trim() ? JSON.parse(workerLabelsDraft) : null
                    updateRouting.mutate(payload)
                  } catch {
                    setRoutingError('Worker labels must be valid JSON')
                  }
                }}
                className="px-3 py-1.5 rounded bg-indigo-500 text-white text-xs hover:bg-indigo-400 transition-colors"
              >
                Save Routing
              </button>
              <button
                type="button"
                onClick={() => {
                  setRoutingError('')
                  setRuntimeDraft(proc.runtime || 'direct')
                  setWorkerIdDraft(proc.worker_id || '')
                  setWorkerLabelsDraft(proc.worker_labels ? (typeof proc.worker_labels === 'string' ? proc.worker_labels : JSON.stringify(proc.worker_labels, null, 2)) : '')
                }}
                className="px-3 py-1.5 rounded border border-border text-xs"
              >
                Reset
              </button>
            </div>
          </div>
          {proc.execution_method === 'http' && (
            <div className="mt-4 rounded-lg border border-border p-4 space-y-3">
              <div>
                <span className="text-sm font-medium">HTTP Config</span>
                <p className="text-xs text-muted-foreground mt-1">Update dispatch mode and tracked polling settings for this process.</p>
              </div>
              {httpConfigError && <div className="rounded border border-red-500/20 bg-red-500/10 p-2 text-xs text-red-400">{httpConfigError}</div>}
              <div className="grid grid-cols-3 gap-4">
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">HTTP Method</label>
                  <select value={httpMethodDraft} onChange={(e) => setHttpMethodDraft(e.target.value)} className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm">
                    {HTTP_METHODS.map((value) => <option key={value}>{value}</option>)}
                  </select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Dispatch Mode</label>
                  <select value={dispatchModeDraft} onChange={(e) => setDispatchModeDraft(e.target.value)} className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm">
                    {HTTP_DISPATCH_MODES.map((value) => <option key={value}>{value}</option>)}
                  </select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">URL</label>
                  <input value={urlDraft} onChange={(e) => setURLDraft(e.target.value)} placeholder="https://api.example.com/jobs" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                </div>
              </div>
              {dispatchModeDraft !== 'sync' && (
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Accepted Status Codes</label>
                  <input value={acceptedStatusCodesDraft} onChange={(e) => setAcceptedStatusCodesDraft(e.target.value)} placeholder="200,201,202" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                </div>
              )}
              {dispatchModeDraft === 'async_tracked' && (
                <>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Job ID Field</label>
                      <input value={jobIDFieldDraft} onChange={(e) => setJobIDFieldDraft(e.target.value)} placeholder="data.id" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Status Field</label>
                      <input value={statusFieldDraft} onChange={(e) => setStatusFieldDraft(e.target.value)} placeholder="data.status" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Status URL Field</label>
                      <input value={statusURLFieldDraft} onChange={(e) => setStatusURLFieldDraft(e.target.value)} placeholder="links.status" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Status URL Template</label>
                      <input value={statusURLTemplateDraft} onChange={(e) => setStatusURLTemplateDraft(e.target.value)} placeholder="https://api.example.com/jobs/{{job.id}}" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Cancel URL Field</label>
                      <input value={cancelURLFieldDraft} onChange={(e) => setCancelURLFieldDraft(e.target.value)} placeholder="links.cancel" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Cancel URL Template</label>
                      <input value={cancelURLTemplateDraft} onChange={(e) => setCancelURLTemplateDraft(e.target.value)} placeholder="https://api.example.com/jobs/{{job.id}}/cancel" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Poll Method</label>
                      <select value={pollMethodDraft} onChange={(e) => setPollMethodDraft(e.target.value)} className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm">
                        {HTTP_METHODS.map((value) => <option key={value}>{value}</option>)}
                      </select>
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Cancel Method</label>
                      <select value={cancelMethodDraft} onChange={(e) => setCancelMethodDraft(e.target.value)} className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm">
                        {HTTP_METHODS.map((value) => <option key={value}>{value}</option>)}
                      </select>
                    </div>
                  </div>
                  <div className="grid grid-cols-3 gap-4">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Running Values</label>
                      <input value={runningValuesDraft} onChange={(e) => setRunningValuesDraft(e.target.value)} placeholder="queued,running" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Success Values</label>
                      <input value={successValuesDraft} onChange={(e) => setSuccessValuesDraft(e.target.value)} placeholder="completed,succeeded" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Failed Values</label>
                      <input value={failedValuesDraft} onChange={(e) => setFailedValuesDraft(e.target.value)} placeholder="failed,error" className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
                    </div>
                  </div>
                </>
              )}
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => {
                    try {
                      const methodConfigDraft: Record<string, any> = {
                        url: urlDraft.trim(),
                        method: httpMethodDraft,
                        dispatch_mode: dispatchModeDraft,
                      }
                      const accepted = parseNumberList(acceptedStatusCodesDraft)
                      if (accepted?.length) methodConfigDraft.accepted_status_codes = accepted
                      if (dispatchModeDraft === 'async_tracked') {
                        if (jobIDFieldDraft.trim()) methodConfigDraft.job_id_field = jobIDFieldDraft.trim()
                        if (statusURLFieldDraft.trim()) methodConfigDraft.status_url_field = statusURLFieldDraft.trim()
                        if (statusURLTemplateDraft.trim()) methodConfigDraft.status_url_template = statusURLTemplateDraft.trim()
                        if (cancelURLFieldDraft.trim()) methodConfigDraft.cancel_url_field = cancelURLFieldDraft.trim()
                        if (cancelURLTemplateDraft.trim()) methodConfigDraft.cancel_url_template = cancelURLTemplateDraft.trim()
                        if (statusFieldDraft.trim()) methodConfigDraft.status_field = statusFieldDraft.trim()
                        const running = parseStringList(runningValuesDraft)
                        if (running?.length) methodConfigDraft.running_values = running
                        const success = parseStringList(successValuesDraft)
                        if (success?.length) methodConfigDraft.success_values = success
                        const failed = parseStringList(failedValuesDraft)
                        if (failed?.length) methodConfigDraft.failed_values = failed
                        if (pollMethodDraft.trim()) methodConfigDraft.poll_method = pollMethodDraft.trim().toUpperCase()
                        if (cancelMethodDraft.trim()) methodConfigDraft.cancel_method = cancelMethodDraft.trim().toUpperCase()
                      }
                      setHTTPConfigError('')
                      updateRouting.mutate({ method_config: methodConfigDraft })
                    } catch (err: any) {
                      setHTTPConfigError(err.message || 'Invalid HTTP configuration')
                    }
                  }}
                  className="px-3 py-1.5 rounded bg-indigo-500 text-white text-xs hover:bg-indigo-400 transition-colors"
                >
                  Save HTTP Config
                </button>
                <button
                  type="button"
                  onClick={() => {
                    const cfg = decodeJSONField(proc.method_config)
                    setHTTPConfigError('')
                    setHttpMethodDraft(cfg?.method || 'POST')
                    setDispatchModeDraft(cfg?.dispatch_mode || 'sync')
                    setURLDraft(cfg?.url || '')
                    setAcceptedStatusCodesDraft(joinNumberList(cfg?.accepted_status_codes))
                    setJobIDFieldDraft(cfg?.job_id_field || '')
                    setStatusURLFieldDraft(cfg?.status_url_field || '')
                    setStatusURLTemplateDraft(cfg?.status_url_template || '')
                    setCancelURLFieldDraft(cfg?.cancel_url_field || '')
                    setCancelURLTemplateDraft(cfg?.cancel_url_template || '')
                    setStatusFieldDraft(cfg?.status_field || '')
                    setPollMethodDraft(cfg?.poll_method || 'GET')
                    setCancelMethodDraft(cfg?.cancel_method || 'POST')
                    setRunningValuesDraft(joinStringList(cfg?.running_values))
                    setSuccessValuesDraft(joinStringList(cfg?.success_values))
                    setFailedValuesDraft(joinStringList(cfg?.failed_values))
                  }}
                  className="px-3 py-1.5 rounded border border-border text-xs"
                >
                  Reset HTTP Config
                </button>
              </div>
            </div>
          )}
          {proc.worker_labels && (
            <div className="mt-4">
              <span className="text-xs text-muted-foreground block mb-1">Worker Labels</span>
              <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-32">
                {typeof proc.worker_labels === 'string'
                  ? (() => { try { return JSON.stringify(JSON.parse(atob(proc.worker_labels)), null, 2) } catch { return proc.worker_labels } })()
                  : JSON.stringify(proc.worker_labels, null, 2)}
              </pre>
            </div>
          )}
          {proc.method_config && (
            <div className="mt-4">
              <span className="text-xs text-muted-foreground block mb-1">Method Config</span>
              {proc.execution_method === 'http' && methodConfig && typeof methodConfig === 'object' && (
                <div className="mb-3 grid grid-cols-2 gap-x-8 gap-y-2 text-xs rounded border border-border p-3">
                  <Detail label="Dispatch Mode" value={methodConfig.dispatch_mode || 'sync'} />
                  <Detail label="HTTP Method" value={methodConfig.method || 'POST'} />
                  <Detail label="URL" value={methodConfig.url || '—'} mono />
                  <Detail label="Accepted Codes" value={Array.isArray(methodConfig.accepted_status_codes) ? methodConfig.accepted_status_codes.join(', ') : 'default'} mono />
                  <Detail label="Status URL Field" value={methodConfig.status_url_field || '—'} mono />
                  <Detail label="Status URL Template" value={methodConfig.status_url_template || '—'} mono />
                  <Detail label="Cancel URL Field" value={methodConfig.cancel_url_field || '—'} mono />
                  <Detail label="Cancel URL Template" value={methodConfig.cancel_url_template || '—'} mono />
                  <Detail label="Job ID Field" value={methodConfig.job_id_field || '—'} mono />
                  <Detail label="Status Field" value={methodConfig.status_field || '—'} mono />
                </div>
              )}
              <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-40">
                {typeof methodConfig === 'string' ? methodConfig : JSON.stringify(methodConfig, null, 2)}
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
