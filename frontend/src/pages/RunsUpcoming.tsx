import { Clock, XCircle } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { useRuns } from '@/hooks/use-api'
import { api } from '@/api/client'
import { formatTimeAgo } from '@/lib/utils'
import { useState } from 'react'

export function RunsUpcoming() {
  const { data, isLoading, refetch } = useRuns()
  const [cancelling, setCancelling] = useState<string | null>(null)

  const runs = (data?.data || [])
    .filter((run: any) => run.state === 'pending' || run.state === 'queued')
    .sort((a: any, b: any) =>
      new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime()
    )

  const handleCancel = async (runId: string) => {
    setCancelling(runId)
    try {
      await api.cancelRun(runId)
      refetch()
    } finally {
      setCancelling(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Upcoming Runs</h1>
          <p className="text-sm text-muted-foreground mt-1">Pending and queued runs ordered by scheduled time</p>
        </div>
      </div>

      {isLoading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : runs.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <Clock size={32} className="mx-auto text-muted-foreground mb-3" />
          <p className="text-sm text-muted-foreground">No upcoming runs</p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Process</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Scheduled</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">State</th>
                <th className="text-left px-4 py-2 font-medium text-muted-foreground">Origin</th>
                <th className="text-right px-4 py-2 font-medium text-muted-foreground">Actions</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run: any) => (
                <tr key={run.id} className="border-b border-border last:border-0 hover:bg-muted/20">
                  <td className="px-4 py-2.5 font-medium">{run.process_name || run.process_id}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{formatTimeAgo(run.scheduled_at)}</td>
                  <td className="px-4 py-2.5"><StateBadge state={run.state} /></td>
                  <td className="px-4 py-2.5 text-muted-foreground">{run.origin}</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => handleCancel(run.id)}
                      disabled={cancelling === run.id}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                    >
                      <XCircle size={12} />
                      {cancelling === run.id ? 'Cancelling...' : 'Cancel'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
