import { useState, useEffect } from 'react'
import { Layers, Pause, Play } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { TargetIcon } from '@/components/domain/target-icon'
import { api } from '@/api/client'
import { formatTimeAgo } from '@/lib/utils'
import { useMutation } from '@tanstack/react-query'

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

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const
const HTTP_DISPATCH_MODES = ['sync', 'async_blind', 'async_tracked'] as const

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

export function QueueDetail() {
  const queueId = window.location.pathname.split('/').pop() || ''
  const [queue, setQueue] = useState<any>(null)
  const [jobs, setJobs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [stateFilter, setStateFilter] = useState('')
  const [runtimeDraft, setRuntimeDraft] = useState('direct')
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

  useEffect(() => {
    Promise.all([
      api.getQueue(queueId),
      api.listJobs(`queue_id=${queueId}${stateFilter ? `&state=${stateFilter}` : ''}`),
    ]).then(([q, j]) => {
      setQueue(q.data)
      setJobs(j.data || [])
      setRuntimeDraft(q.data?.runtime || 'direct')
      setWorkerIdDraft(q.data?.worker_id || '')
      setWorkerLabelsDraft(q.data?.worker_labels ? (typeof q.data.worker_labels === 'string' ? q.data.worker_labels : JSON.stringify(q.data.worker_labels, null, 2)) : '')
      const cfg = decodeJSONField(q.data?.method_config)
      if (q.data?.execution_method === 'http' && cfg && typeof cfg === 'object') {
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
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [queueId, stateFilter])

  const updateRouting = useMutation({
    mutationFn: (payload: any) => api.updateQueue(queueId, payload),
    onSuccess: async () => {
      setRoutingError('')
      const q = await api.getQueue(queueId)
      setQueue(q.data)
      setRuntimeDraft(q.data?.runtime || 'direct')
      setWorkerIdDraft(q.data?.worker_id || '')
      setWorkerLabelsDraft(q.data?.worker_labels ? (typeof q.data.worker_labels === 'string' ? q.data.worker_labels : JSON.stringify(q.data.worker_labels, null, 2)) : '')
    },
    onError: (err: any) => setRoutingError(err.message || 'Failed to update queue routing'),
  })

  if (loading) return <div className="text-sm text-muted-foreground p-6">Loading...</div>
  if (!queue) return <div className="text-sm text-red-400 p-6">Queue not found</div>

  const states = ['', 'pending', 'waiting_for_worker', 'running', 'kill_requested', 'retrying', 'completed', 'failed', 'killed', 'cancelled']
  const methodConfig = decodeJSONField(queue.method_config)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center">
            <Layers size={20} className="text-blue-400" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{queue.name}</h1>
            <div className="flex items-center gap-2 mt-0.5">
              <TargetIcon method={queue.execution_method} />
              <span className="text-xs text-muted-foreground">{queue.execution_method}</span>
              <span className="text-xs text-muted-foreground">·</span>
              <span className="text-xs text-muted-foreground">Runtime: {queue.runtime}</span>
              <span className="text-xs text-muted-foreground">·</span>
              <span className="text-xs text-muted-foreground">Concurrency: {queue.concurrency}</span>
              <span className="text-xs text-muted-foreground">·</span>
              <span className="text-xs text-muted-foreground">Max attempts: {queue.max_attempts}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {queue.enabled ? (
            <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs bg-emerald-500/10 text-emerald-400">
              <Play size={10} /> Active
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs bg-amber-500/10 text-amber-400">
              <Pause size={10} /> Paused
            </span>
          )}
        </div>
      </div>

      {/* State filter */}
      <div className="grid grid-cols-2 gap-4 text-xs">
        <div className="rounded-lg border border-border bg-card p-3">
          <div className="flex justify-between gap-3">
            <span className="text-muted-foreground">Worker ID</span>
            <span className="font-mono text-right">{queue.worker_id || '—'}</span>
          </div>
        </div>
        <div className="rounded-lg border border-border bg-card p-3">
          <div className="flex justify-between gap-3">
            <span className="text-muted-foreground">Retry Backoff</span>
            <span className="font-mono text-right">{queue.retry_backoff}</span>
          </div>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 space-y-3">
        <div>
          <span className="text-sm font-medium">Routing</span>
          <p className="text-xs text-muted-foreground mt-1">Update runtime and worker selection for future jobs in this queue.</p>
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
            <input value={workerIdDraft} onChange={(e) => setWorkerIdDraft(e.target.value)} placeholder="wrk_..."
              className="w-full px-2 py-1.5 rounded border border-border bg-background text-sm font-mono" />
          </div>
        </div>
        <div className="space-y-1">
          <label className="text-xs text-muted-foreground">Worker Labels JSON</label>
          <textarea value={workerLabelsDraft} onChange={(e) => setWorkerLabelsDraft(e.target.value)} rows={4}
            placeholder='["linux","prod"] or {"region":"eu-west-1"}'
            className="w-full px-2 py-2 rounded border border-border bg-background text-sm font-mono" />
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
              setRuntimeDraft(queue.runtime || 'direct')
              setWorkerIdDraft(queue.worker_id || '')
              setWorkerLabelsDraft(queue.worker_labels ? (typeof queue.worker_labels === 'string' ? queue.worker_labels : JSON.stringify(queue.worker_labels, null, 2)) : '')
            }}
            className="px-3 py-1.5 rounded border border-border text-xs"
          >
            Reset
          </button>
        </div>
      </div>
      {queue.execution_method === 'http' && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-3">
          <div>
            <span className="text-sm font-medium">HTTP Config</span>
            <p className="text-xs text-muted-foreground mt-1">Update dispatch mode and tracked polling settings for future jobs in this queue.</p>
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
                const cfg = decodeJSONField(queue.method_config)
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

      {queue.worker_labels && (
        <div className="rounded-lg border border-border bg-card p-4">
          <span className="text-xs text-muted-foreground block mb-2">Worker Labels</span>
          <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-32">
            {typeof queue.worker_labels === 'string'
              ? (() => { try { return JSON.stringify(JSON.parse(atob(queue.worker_labels)), null, 2) } catch { return queue.worker_labels } })()
              : JSON.stringify(queue.worker_labels, null, 2)}
          </pre>
        </div>
      )}

      {queue.method_config && (
        <div className="rounded-lg border border-border bg-card p-4">
          <span className="text-xs text-muted-foreground block mb-2">Method Config</span>
          {queue.execution_method === 'http' && methodConfig && typeof methodConfig === 'object' && (
            <div className="mb-3 grid grid-cols-2 gap-x-8 gap-y-2 text-xs rounded border border-border p-3">
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Dispatch Mode</span><span className="font-mono text-right">{methodConfig.dispatch_mode || 'sync'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">HTTP Method</span><span className="font-mono text-right">{methodConfig.method || 'POST'}</span></div>
              <div className="flex justify-between gap-3 col-span-2"><span className="text-muted-foreground">URL</span><span className="font-mono text-right break-all">{methodConfig.url || '—'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Accepted Codes</span><span className="font-mono text-right">{Array.isArray(methodConfig.accepted_status_codes) ? methodConfig.accepted_status_codes.join(', ') : 'default'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Status Field</span><span className="font-mono text-right">{methodConfig.status_field || '—'}</span></div>
              <div className="flex justify-between gap-3 col-span-2"><span className="text-muted-foreground">Status URL</span><span className="font-mono text-right break-all">{methodConfig.status_url_field || methodConfig.status_url_template || '—'}</span></div>
              <div className="flex justify-between gap-3 col-span-2"><span className="text-muted-foreground">Cancel URL</span><span className="font-mono text-right break-all">{methodConfig.cancel_url_field || methodConfig.cancel_url_template || '—'}</span></div>
            </div>
          )}
          <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-40">
            {typeof methodConfig === 'string' ? methodConfig : JSON.stringify(methodConfig, null, 2)}
          </pre>
        </div>
      )}

      <div className="flex gap-1">
        {states.map((s) => (
          <button
            key={s}
            onClick={() => setStateFilter(s)}
            className={`px-2.5 py-1 rounded text-xs transition-colors ${
              stateFilter === s ? 'bg-indigo-500/10 text-indigo-400' : 'text-muted-foreground hover:text-foreground hover:bg-muted/50'
            }`}
          >
            {s || 'All'}
          </button>
        ))}
      </div>

      {/* Jobs table */}
      {jobs.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">No jobs {stateFilter ? `with state "${stateFilter}"` : 'in this queue'}</p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">ID</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Reference</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">State</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Priority</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Attempts</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Created</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job: any) => (
                <tr
                  key={job.id}
                  onClick={() => { window.history.pushState(null, '', `/jobs/${job.id}`); window.dispatchEvent(new PopStateEvent('popstate')) }}
                  className="border-b border-border last:border-0 hover:bg-muted/20 cursor-pointer"
                >
                  <td className="px-4 py-2.5 font-mono text-xs">{job.id.slice(0, 16)}...</td>
                  <td className="px-4 py-2.5">{job.reference || '-'}</td>
                  <td className="px-4 py-2.5"><StateBadge state={job.state} /></td>
                  <td className="px-4 py-2.5 text-muted-foreground">{job.priority}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{job.attempt}/{job.max_attempts || '?'}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{formatTimeAgo(job.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
