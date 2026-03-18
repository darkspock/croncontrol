import { useState } from 'react'
import { AlertTriangle, RotateCcw } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { useJobs, useQueues } from '@/hooks/use-api'
import { api } from '@/api/client'
import { formatTimeAgo } from '@/lib/utils'

export function FailedJobs() {
  const { data: jobData, isLoading: jobsLoading, refetch } = useJobs('state=failed')
  const { data: queueData } = useQueues()
  const [replaying, setReplaying] = useState<string | null>(null)

  const jobs = jobData?.data || []
  const queues = queueData?.data || []

  // Group jobs by queue
  const grouped = jobs.reduce((acc: Record<string, any[]>, job: any) => {
    const queueId = job.queue_id || 'unknown'
    if (!acc[queueId]) acc[queueId] = []
    acc[queueId].push(job)
    return acc
  }, {} as Record<string, any[]>)

  const getQueueName = (queueId: string) => {
    const q = queues.find((q: any) => q.id === queueId)
    return q?.name || queueId
  }

  const handleReplay = async (jobId: string) => {
    setReplaying(jobId)
    try {
      await api.replayJob(jobId)
      refetch()
    } finally {
      setReplaying(null)
    }
  }

  const handleBulkReplay = async (queueJobs: any[]) => {
    for (const job of queueJobs) {
      await api.replayJob(job.id)
    }
    refetch()
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Failed Jobs</h1>
        <p className="text-sm text-muted-foreground mt-1">Jobs that exhausted all retry attempts, grouped by queue</p>
      </div>

      {jobsLoading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : jobs.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <AlertTriangle size={32} className="mx-auto text-muted-foreground mb-3" />
          <p className="text-sm text-muted-foreground">No failed jobs</p>
        </div>
      ) : (
        Object.entries(grouped).map(([queueId, queueJobs]) => (
          <div key={queueId} className="rounded-lg border border-border overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 bg-muted/30 border-b border-border">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm">{getQueueName(queueId)}</span>
                <span className="text-xs text-red-400 bg-red-500/10 px-2 py-0.5 rounded-full">{queueJobs.length} failed</span>
              </div>
              <button
                onClick={() => handleBulkReplay(queueJobs)}
                className="inline-flex items-center gap-1 px-2.5 py-1 rounded text-xs text-indigo-400 hover:bg-indigo-500/10 transition-colors"
              >
                <RotateCcw size={12} />
                Replay all
              </button>
            </div>
            <table className="w-full text-sm">
              <tbody>
                {queueJobs.map((job: any) => (
                  <tr key={job.id} className="border-b border-border last:border-0 hover:bg-muted/20">
                    <td className="px-4 py-2.5 font-mono text-xs w-44">{job.id.slice(0, 16)}...</td>
                    <td className="px-4 py-2.5">{job.reference || '-'}</td>
                    <td className="px-4 py-2.5"><StateBadge state={job.state} /></td>
                    <td className="px-4 py-2.5 text-muted-foreground">{job.attempt}/{job.max_attempts || '?'} attempts</td>
                    <td className="px-4 py-2.5 text-muted-foreground">{formatTimeAgo(job.created_at)}</td>
                    <td className="px-4 py-2.5 text-right">
                      <button
                        onClick={() => handleReplay(job.id)}
                        disabled={replaying === job.id}
                        className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-indigo-400 hover:bg-indigo-500/10 transition-colors disabled:opacity-50"
                      >
                        <RotateCcw size={12} />
                        {replaying === job.id ? 'Replaying...' : 'Replay'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))
      )}
    </div>
  )
}
