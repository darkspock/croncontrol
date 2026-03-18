import { Cpu, Play, AlertTriangle, Layers, Activity } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { TargetIcon } from '@/components/domain/target-icon'
import { OnboardingBanner } from '@/components/domain/onboarding-banner'
import { useProcesses, useRuns } from '@/hooks/use-api'
import { formatTimeAgo } from '@/lib/utils'

export function Dashboard() {
  const { data: procData, isLoading: procLoading } = useProcesses()
  const { data: runData, isLoading: runLoading } = useRuns()

  const processes = procData?.data || []
  const runs = runData?.data || []

  const runningCount = runs.filter((r: any) => r.state === 'running').length
  const failedCount = runs.filter((r: any) => r.state === 'failed').length

  const summaryCards = [
    { icon: Cpu, label: 'Processes', value: processes.length.toString(), color: 'text-indigo-400' },
    { icon: Play, label: 'Running', value: runningCount.toString(), color: 'text-emerald-400' },
    { icon: AlertTriangle, label: 'Failed', value: failedCount.toString(), color: 'text-red-400' },
    { icon: Layers, label: 'Total Runs', value: runs.length.toString(), color: 'text-blue-400' },
  ]

  return (
    <div className="space-y-6">
      <OnboardingBanner />
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">Overview of scheduled processes and execution activity</p>
      </div>

      <div className="grid grid-cols-4 gap-4">
        {summaryCards.map((card) => (
          <div key={card.label} className="rounded-lg border border-border bg-card p-4 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{card.label}</span>
              <card.icon size={16} className={card.color} />
            </div>
            <p className={`text-2xl font-semibold font-mono tracking-tight ${card.color}`}>
              {procLoading || runLoading ? '—' : card.value}
            </p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-6">
        <div className="col-span-2 rounded-lg border border-border bg-card">
          <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
            <Activity size={14} className="text-muted-foreground" />
            <span className="text-sm font-medium">Recent Runs</span>
          </div>
          {runLoading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
          ) : runs.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">
              No runs yet. Create a process and trigger it.
            </div>
          ) : (
            <div className="divide-y divide-border">
              {runs.slice(0, 10).map((run: any) => (
                <div key={run.id} className="flex items-center gap-4 px-4 py-2.5 hover:bg-muted/30 transition-colors cursor-pointer">
                  <TargetIcon method={run.execution_method || 'http'} />
                  <span className="text-sm font-medium flex-1 truncate">{run.process_name || run.process_id?.slice(0, 20)}</span>
                  <StateBadge state={run.state} />
                  <span className="text-xs text-muted-foreground w-20 text-right">
                    {run.created_at ? formatTimeAgo(run.created_at) : '—'}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="rounded-lg border border-border bg-card">
          <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
            <Cpu size={14} className="text-muted-foreground" />
            <span className="text-sm font-medium">Processes</span>
          </div>
          {procLoading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
          ) : processes.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No processes yet.</div>
          ) : (
            <div className="divide-y divide-border">
              {processes.map((proc: any) => (
                <div key={proc.id} className="flex items-center gap-3 px-4 py-2.5 hover:bg-muted/30 transition-colors cursor-pointer">
                  <span className={`w-2 h-2 rounded-full ${proc.enabled ? 'bg-emerald-400' : 'bg-zinc-500'}`} />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{proc.name}</p>
                    <p className="text-[10px] font-mono text-muted-foreground">{proc.schedule_type} · {proc.execution_method}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
