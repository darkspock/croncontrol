import { ArrowLeft, RotateCcw, XCircle, ChevronDown, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { StateBadge } from '@/components/domain/state-badge'
import { formatDuration } from '@/lib/utils'
import { api } from '@/api/client'
import { useQueryClient } from '@tanstack/react-query'
import { useJob } from '@/hooks/use-api'

interface JobDetailProps {
  jobId: string
}

export function JobDetail({ jobId }: JobDetailProps) {
  const qc = useQueryClient()
  const { data, isLoading } = useJob(jobId)

  const job = data?.data?.job
  const attempts = data?.data?.attempts || []

  const goBack = () => {
    window.history.pushState(null, '', '/jobs')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  const handleCancel = async () => {
    if (confirm('Cancel this job?')) {
      await api.cancelJob(jobId)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['job', jobId] }),
        qc.invalidateQueries({ queryKey: ['jobs'] }),
      ])
    }
  }

  const handleReplay = async () => {
    await api.replayJob(jobId)
    await Promise.all([
      qc.invalidateQueries({ queryKey: ['job', jobId] }),
      qc.invalidateQueries({ queryKey: ['jobs'] }),
    ])
  }

  if (isLoading) return <div className="p-8 text-center text-sm text-muted-foreground">Loading...</div>
  if (!job) return <div className="p-8 text-center text-sm text-muted-foreground">Job not found</div>

  const isTerminal = ['completed', 'failed', 'killed', 'cancelled'].includes(job.state)
  const isStopping = job.state === 'kill_requested'

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <button onClick={goBack} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
            <ArrowLeft size={12} /> Back to jobs
          </button>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold tracking-tight font-mono">{job.id.slice(0, 20)}...</h1>
            <StateBadge state={job.state} />
          </div>
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>Queue: <strong className="text-foreground">{job.queue_name || job.queue_id?.slice(0, 15)}</strong></span>
            <span>·</span>
            <span className="font-mono">priority: {job.priority}</span>
            <span>·</span>
            <span className="font-mono">attempt: {job.attempt}</span>
            {job.reference && (<><span>·</span><span className="font-mono">ref: {job.reference}</span></>)}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {isTerminal && (
            <button onClick={handleReplay} className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-muted/50 transition-colors">
              <RotateCcw size={13} /> Replay
            </button>
          )}
          {!isTerminal && (
            <button onClick={handleCancel} disabled={isStopping} className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-red-500/10 text-red-400 border border-red-500/20 text-sm hover:bg-red-500/20 transition-colors disabled:opacity-60">
              <XCircle size={13} /> {isStopping ? 'Stopping...' : 'Cancel'}
            </button>
          )}
        </div>
      </div>

      {/* Payload */}
      {job.payload && (
        <div className="rounded-lg border border-border bg-card p-4">
          <span className="text-sm font-medium mb-2 block">Payload</span>
          <pre className="text-xs font-mono text-zinc-300 bg-[#0a0a0c] p-3 rounded overflow-auto max-h-40">
            {typeof job.payload === 'string' ? job.payload : JSON.stringify(job.payload, null, 2)}
          </pre>
        </div>
      )}

      {/* Attempt History */}
      <div className="rounded-lg border border-border bg-card">
        <div className="px-4 py-3 border-b border-border">
          <span className="text-sm font-medium">Attempt History ({attempts.length})</span>
        </div>
        {attempts.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">No attempts yet.</div>
        ) : (
          <div className="divide-y divide-border">
            {attempts.map((att: any) => (
              <AttemptRow key={att.id} attempt={att} />
            ))}
          </div>
        )}
      </div>

      {/* Details */}
      <div className="rounded-lg border border-border bg-card p-4">
        <span className="text-sm font-medium mb-3 block">Details</span>
        <div className="grid grid-cols-2 gap-y-2 gap-x-8 text-xs">
          <div className="flex justify-between"><span className="text-muted-foreground">Job ID</span><span className="font-mono">{job.id}</span></div>
          <div className="flex justify-between"><span className="text-muted-foreground">Queue ID</span><span className="font-mono">{job.queue_id}</span></div>
          <div className="flex justify-between"><span className="text-muted-foreground">State</span><span>{job.state}</span></div>
          {job.state === 'kill_requested' && <div className="flex justify-between"><span className="text-muted-foreground">Stop Status</span><span>Kill requested; waiting for executor confirmation</span></div>}
          <div className="flex justify-between"><span className="text-muted-foreground">Created</span><span>{new Date(job.created_at).toLocaleString()}</span></div>
          {job.idempotency_key && <div className="flex justify-between"><span className="text-muted-foreground">Idempotency Key</span><span className="font-mono">{job.idempotency_key}</span></div>}
          {job.replayed_from_job_id && <div className="flex justify-between"><span className="text-muted-foreground">Replayed From</span><span className="font-mono">{job.replayed_from_job_id}</span></div>}
          {job.cancel_reason && <div className="flex justify-between"><span className="text-muted-foreground">Cancel Reason</span><span>{job.cancel_reason}</span></div>}
        </div>
      </div>
    </div>
  )
}

function AttemptRow({ attempt }: { attempt: any }) {
  const [expanded, setExpanded] = useState(false)
  const success = attempt.response_code >= 200 && attempt.response_code < 300
  const hasResponse = attempt.response_code != null || attempt.response_body || attempt.error_message

  return (
    <div>
      <div
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-3 px-4 py-3 hover:bg-muted/20 transition-colors cursor-pointer"
      >
        {hasResponse ? (
          expanded ? <ChevronDown size={14} className="text-muted-foreground" /> : <ChevronRight size={14} className="text-muted-foreground" />
        ) : <div className="w-3.5" />}
        <span className="text-xs font-mono font-medium w-6">#{attempt.attempt_number}</span>
        <span className={`text-xs font-mono ${success ? 'text-emerald-400' : 'text-red-400'}`}>
          {attempt.response_code || 'ERR'}
        </span>
        <span className="text-xs font-mono text-muted-foreground">{attempt.duration_ms ? formatDuration(attempt.duration_ms) : '—'}</span>
        <span className="flex-1" />
        {attempt.error_message && <span className="text-xs text-red-400 truncate max-w-60">{attempt.error_message}</span>}
        <span className="text-xs text-muted-foreground">{attempt.started_at ? new Date(attempt.started_at).toLocaleTimeString() : ''}</span>
      </div>
      {expanded && hasResponse && (
        <div className="px-4 pb-3 space-y-2">
          {attempt.request && (
            <div>
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Request</span>
              <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-2 rounded mt-1 overflow-auto max-h-32">
                {typeof attempt.request === 'string' ? attempt.request : JSON.stringify(attempt.request, null, 2)}
              </pre>
            </div>
          )}
          {attempt.response_body && (
            <div>
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider">
                Response {attempt.truncated && <span className="text-amber-400">(truncated)</span>}
              </span>
              <pre className="text-xs font-mono text-zinc-400 bg-[#0a0a0c] p-2 rounded mt-1 overflow-auto max-h-32">
                {attempt.response_body}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
