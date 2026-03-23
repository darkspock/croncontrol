import { useState, useCallback } from 'react'
import { RefreshCw } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { TargetIcon } from '@/components/domain/target-icon'
import { ORIGIN_LABELS } from '@/lib/constants'
import { useRuns } from '@/hooks/use-api'
import { formatDuration } from '@/lib/utils'

const STATE_OPTIONS = ['', 'pending', 'queued', 'waiting_for_worker', 'running', 'kill_requested', 'completed', 'failed', 'hung', 'killed', 'retrying', 'skipped', 'cancelled', 'paused']

export function RunList() {
  const [stateFilter, setStateFilter] = useState('')
  const [originFilter, setOriginFilter] = useState('')

  const params = new URLSearchParams()
  if (stateFilter) params.set('state', stateFilter)
  if (originFilter) params.set('origin', originFilter)
  const queryStr = params.toString()

  const { data, isLoading, refetch } = useRuns(queryStr || undefined)
  const [spinning, setSpinning] = useState(false)
  const runs = data?.data || []

  const handleRefresh = useCallback(() => {
    setSpinning(true)
    refetch().finally(() => setTimeout(() => setSpinning(false), 800))
  }, [refetch])

  const navigateToRun = (runId: string) => {
    window.history.pushState(null, '', `/runs/${runId}`)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Runs</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {isLoading ? 'Loading...' : `${runs.length} runs`}
          </p>
        </div>
        <button
          type="button"
          onClick={handleRefresh}
          disabled={spinning}
          className="relative z-10 flex items-center gap-2 px-4 py-2 rounded-md border border-border text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors cursor-pointer disabled:opacity-50"
        >
          <RefreshCw size={14} className={spinning ? 'animate-spin' : ''} />
          {spinning ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">State:</span>
          <select
            value={stateFilter}
            onChange={(e) => setStateFilter(e.target.value)}
            className="px-2 py-1 rounded-md border border-border bg-background text-sm"
          >
            <option value="">All</option>
            {STATE_OPTIONS.filter(Boolean).map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Origin:</span>
          <select
            value={originFilter}
            onChange={(e) => setOriginFilter(e.target.value)}
            className="px-2 py-1 rounded-md border border-border bg-background text-sm"
          >
            <option value="">All</option>
            {Object.entries(ORIGIN_LABELS).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </select>
        </div>
        {(stateFilter || originFilter) && (
          <button
            onClick={() => { setStateFilter(''); setOriginFilter('') }}
            className="text-xs text-indigo-400 hover:underline"
          >
            Clear filters
          </button>
        )}
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border bg-card overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-sm text-muted-foreground">Loading runs...</div>
        ) : runs.length === 0 ? (
          <div className="p-8 text-center text-sm text-muted-foreground">
            {stateFilter || originFilter
              ? 'No runs match the current filters.'
              : 'No runs yet. Trigger a process to create a run.'}
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Process</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">State</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Origin</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Attempt</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Started</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Duration</th>
                <th className="text-right px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Scheduled</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {runs.map((run: any) => (
                <tr
                  key={run.id}
                  onClick={() => navigateToRun(run.id)}
                  className="hover:bg-muted/20 transition-colors cursor-pointer"
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <TargetIcon method={run.execution_method || 'http'} size={13} />
                      <span className="text-sm font-medium truncate max-w-48">
                        {run.process_name || run.process_id?.slice(0, 20)}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <StateBadge state={run.state} />
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                      {ORIGIN_LABELS[run.origin] || run.origin}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs font-mono text-muted-foreground">
                      {run.attempt}/{run.max_attempts}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs font-mono text-muted-foreground">
                      {run.started_at ? new Date(run.started_at).toLocaleString() : '—'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs font-mono text-muted-foreground">
                      {run.duration_ms ? formatDuration(run.duration_ms) : '—'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <span className="text-xs font-mono text-muted-foreground">
                      {run.scheduled_at ? new Date(run.scheduled_at).toLocaleString() : '—'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
