import { useState } from 'react'
import { StateBadge } from '@/components/domain/state-badge'
import { useJobs } from '@/hooks/use-api'
import { formatTimeAgo } from '@/lib/utils'

export function JobList() {
  const [stateFilter, setStateFilter] = useState('')
  const params = stateFilter ? `state=${stateFilter}` : undefined
  const { data, isLoading } = useJobs(params)
  const jobs = data?.data || []

  const navigateToJob = (jobId: string) => {
    window.history.pushState(null, '', `/jobs/${jobId}`)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Jobs</h1>
        <p className="text-sm text-muted-foreground mt-1">
          {isLoading ? 'Loading...' : `${jobs.length} jobs`}
        </p>
      </div>

      <div className="flex items-center gap-3">
        <span className="text-xs text-muted-foreground">State:</span>
        <select
          value={stateFilter}
          onChange={(e) => setStateFilter(e.target.value)}
          className="px-2 py-1 rounded-md border border-border bg-background text-sm"
        >
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="waiting_for_worker">Waiting for worker</option>
          <option value="running">Running</option>
          <option value="kill_requested">Kill requested</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="retrying">Retrying</option>
          <option value="killed">Killed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      <div className="rounded-lg border border-border bg-card overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
        ) : jobs.length === 0 ? (
          <div className="p-8 text-center text-sm text-muted-foreground">No jobs yet.</div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Queue</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">State</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Priority</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Attempt</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Reference</th>
                <th className="text-right px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {jobs.map((job: any) => (
                <tr key={job.id} onClick={() => navigateToJob(job.id)} className="hover:bg-muted/20 transition-colors cursor-pointer">
                  <td className="px-4 py-3 text-sm font-medium">{job.queue_name || job.queue_id?.slice(0, 15)}</td>
                  <td className="px-4 py-3"><StateBadge state={job.state} /></td>
                  <td className="px-4 py-3 text-xs font-mono text-muted-foreground">{job.priority}</td>
                  <td className="px-4 py-3 text-xs font-mono text-muted-foreground">{job.attempt}</td>
                  <td className="px-4 py-3 text-xs font-mono text-muted-foreground truncate max-w-40">{job.reference || '—'}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground text-right">{job.created_at ? formatTimeAgo(job.created_at) : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
