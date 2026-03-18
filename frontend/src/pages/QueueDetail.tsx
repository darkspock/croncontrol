import { useState, useEffect } from 'react'
import { Layers, Pause, Play } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { TargetIcon } from '@/components/domain/target-icon'
import { api } from '@/api/client'
import { formatTimeAgo } from '@/lib/utils'

export function QueueDetail() {
  const queueId = window.location.pathname.split('/').pop() || ''
  const [queue, setQueue] = useState<any>(null)
  const [jobs, setJobs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [stateFilter, setStateFilter] = useState('')

  useEffect(() => {
    Promise.all([
      api.getQueue(queueId),
      api.listJobs(`queue_id=${queueId}${stateFilter ? `&state=${stateFilter}` : ''}`),
    ]).then(([q, j]) => {
      setQueue(q.data)
      setJobs(j.data || [])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [queueId, stateFilter])

  if (loading) return <div className="text-sm text-muted-foreground p-6">Loading...</div>
  if (!queue) return <div className="text-sm text-red-400 p-6">Queue not found</div>

  const states = ['', 'pending', 'running', 'retrying', 'completed', 'failed', 'killed', 'cancelled']

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
