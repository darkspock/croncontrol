import { useState } from 'react'
import { useRuns } from '@/hooks/use-api'
import { cn } from '@/lib/utils'

const RANGES = [
  { label: '1h', hours: 1 },
  { label: '6h', hours: 6 },
  { label: '24h', hours: 24 },
  { label: '7d', hours: 168 },
]

const STATE_COLORS: Record<string, string> = {
  completed: 'bg-emerald-500/60',
  failed: 'bg-red-500/60',
  running: 'bg-indigo-500/60',
  pending: 'bg-blue-500/40',
  hung: 'bg-amber-500/60',
  killed: 'bg-red-400/50',
  retrying: 'bg-orange-500/50',
  skipped: 'bg-zinc-500/40',
  cancelled: 'bg-zinc-500/30',
  paused: 'bg-yellow-500/40',
}

export function Timeline() {
  const [range, setRange] = useState(24)
  const { data, isLoading } = useRuns()
  const runs = data?.data || []

  const now = new Date()
  const rangeStart = new Date(now.getTime() - range * 60 * 60 * 1000)

  // Group runs by process
  const byProcess: Record<string, { name: string; runs: any[] }> = {}
  for (const run of runs) {
    const key = run.process_id
    if (!byProcess[key]) {
      byProcess[key] = { name: run.process_name || run.process_id?.slice(0, 15), runs: [] }
    }
    byProcess[key].runs.push(run)
  }

  const processes = Object.values(byProcess)

  // Calculate position as percentage of the time range
  const getPosition = (time: string) => {
    const t = new Date(time).getTime()
    const pct = ((t - rangeStart.getTime()) / (now.getTime() - rangeStart.getTime())) * 100
    return Math.max(0, Math.min(100, pct))
  }

  const getWidth = (start: string, end?: string) => {
    const s = new Date(start).getTime()
    const e = end ? new Date(end).getTime() : now.getTime()
    const totalMs = now.getTime() - rangeStart.getTime()
    const widthPct = ((e - s) / totalMs) * 100
    return Math.max(0.5, Math.min(100, widthPct))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Timeline</h1>
          <p className="text-sm text-muted-foreground mt-1">Visual execution timeline</p>
        </div>
        <div className="flex gap-1">
          {RANGES.map((r) => (
            <button
              key={r.hours}
              onClick={() => setRange(r.hours)}
              className={cn(
                'px-3 py-1 rounded-md text-xs font-medium transition-colors',
                range === r.hours ? 'bg-indigo-500 text-white' : 'bg-muted text-muted-foreground hover:text-foreground'
              )}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
      ) : processes.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          No runs in this time range.
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
          {/* Time axis header */}
          <div className="flex items-center h-8 border-b border-border bg-muted/30 px-4">
            <div className="w-36 flex-shrink-0 text-xs text-muted-foreground">Process</div>
            <div className="flex-1 relative">
              {[0, 25, 50, 75, 100].map((pct) => {
                const time = new Date(rangeStart.getTime() + (pct / 100) * (now.getTime() - rangeStart.getTime()))
                return (
                  <span key={pct} className="absolute text-[10px] font-mono text-muted-foreground -translate-x-1/2" style={{ left: `${pct}%` }}>
                    {time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                )
              })}
            </div>
          </div>

          {/* Process rows */}
          {processes.map((proc, i) => (
            <div key={i} className="flex items-center h-10 border-b border-border last:border-0 hover:bg-muted/10 px-4">
              <div className="w-36 flex-shrink-0 text-xs font-medium truncate" title={proc.name}>
                {proc.name}
              </div>
              <div className="flex-1 relative h-6">
                {/* Grid lines */}
                {[25, 50, 75].map((pct) => (
                  <div key={pct} className="absolute top-0 bottom-0 w-px bg-border/30" style={{ left: `${pct}%` }} />
                ))}

                {/* Run bars */}
                {proc.runs.map((run: any) => {
                  const startTime = run.started_at || run.scheduled_at
                  if (!startTime) return null
                  const left = getPosition(startTime)
                  const width = getWidth(startTime, run.finished_at)

                  return (
                    <div
                      key={run.id}
                      title={`${run.state} · ${run.origin}`}
                      className={cn(
                        'absolute top-1 h-4 rounded-sm cursor-pointer hover:brightness-125 transition-all',
                        STATE_COLORS[run.state] || 'bg-zinc-500/30',
                        run.state === 'running' && 'animate-pulse'
                      )}
                      style={{ left: `${left}%`, width: `${width}%`, minWidth: '4px' }}
                      onClick={() => {
                        window.history.pushState(null, '', `/runs/${run.id}`)
                        window.dispatchEvent(new PopStateEvent('popstate'))
                      }}
                    />
                  )
                })}
              </div>
            </div>
          ))}

          {/* Legend */}
          <div className="flex items-center gap-4 px-4 py-2 border-t border-border bg-muted/20">
            {Object.entries(STATE_COLORS).slice(0, 6).map(([state, color]) => (
              <div key={state} className="flex items-center gap-1">
                <div className={cn('w-3 h-2 rounded-sm', color)} />
                <span className="text-[10px] text-muted-foreground">{state}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
