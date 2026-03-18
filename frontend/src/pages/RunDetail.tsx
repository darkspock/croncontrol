import { ArrowLeft, RotateCcw, XCircle } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { ProgressBar } from '@/components/domain/progress-bar'
import { OutputViewer } from '@/components/domain/output-viewer'
import { ORIGIN_LABELS } from '@/lib/constants'
import { useRun, useRunOutput } from '@/hooks/use-api'
import { formatDuration } from '@/lib/utils'
import { api } from '@/api/client'

interface RunDetailProps {
  runId: string
}

export function RunDetail({ runId }: RunDetailProps) {
  const { data, isLoading } = useRun(runId)
  const { data: outputData } = useRunOutput(runId)

  const run = data?.data
  const attempts = data?.attempts || []
  const outputs = outputData?.data || []

  // Get error from the latest attempt
  const lastAttempt = attempts.length > 0 ? attempts[attempts.length - 1] : null
  const errorMessage = lastAttempt?.error_message

  const stdout = outputs.filter((o: any) => o.stream === 'stdout').map((o: any) => o.content).join('\n')
  const stderr = outputs.filter((o: any) => o.stream === 'stderr').map((o: any) => o.content).join('\n')

  const handleKill = async () => {
    if (confirm('Are you sure you want to kill this run?')) {
      await api.killRun(runId)
    }
  }

  const goBack = () => {
    window.history.pushState(null, '', '/runs')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  if (isLoading) {
    return <div className="p-8 text-center text-sm text-muted-foreground">Loading run...</div>
  }

  if (!run) {
    return <div className="p-8 text-center text-sm text-muted-foreground">Run not found</div>
  }

  const isRunning = run.state === 'running' || run.state === 'pending'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <button onClick={goBack} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
            <ArrowLeft size={12} /> Back to runs
          </button>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold tracking-tight font-mono">{run.id.slice(0, 20)}...</h1>
            <StateBadge state={run.state} />
          </div>
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            {run.process_name && (
              <>
                <span className="font-medium text-foreground">{run.process_name}</span>
                <span>·</span>
              </>
            )}
            <span className="font-mono px-1.5 py-0.5 rounded bg-muted">
              {ORIGIN_LABELS[run.origin] || run.origin}
            </span>
            <span>·</span>
            <span className="font-mono">attempt {run.attempt}/{run.max_attempts}</span>
            {run.duration_ms && (
              <>
                <span>·</span>
                <span className="font-mono">{formatDuration(run.duration_ms)}</span>
              </>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {!isRunning && (
            <button
              onClick={async () => {
                const res = await api.replayRun(runId)
                if (res?.data?.id) {
                  window.history.pushState(null, '', `/runs/${res.data.id}`)
                  window.dispatchEvent(new PopStateEvent('popstate'))
                }
              }}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-muted/50 transition-colors"
            >
              <RotateCcw size={13} /> Replay
            </button>
          )}
          {isRunning && (
            <button
              onClick={handleKill}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-red-500/10 text-red-400 border border-red-500/20 text-sm hover:bg-red-500/20 transition-colors"
            >
              <XCircle size={13} /> Kill
            </button>
          )}
        </div>
      </div>

      {/* Error message */}
      {errorMessage && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/5 p-4">
          <div className="flex items-start gap-3">
            <XCircle size={16} className="text-red-400 mt-0.5 flex-shrink-0" />
            <div>
              <p className="text-sm font-medium text-red-400">Execution failed</p>
              <p className="text-sm text-muted-foreground mt-1 font-mono whitespace-pre-wrap">{errorMessage}</p>
            </div>
          </div>
        </div>
      )}

      {/* Attempts history */}
      {attempts.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-4">
          <span className="text-sm font-medium mb-3 block">Attempts</span>
          <div className="space-y-2">
            {attempts.map((a: any) => (
              <div key={a.id} className="flex items-center gap-4 text-xs py-2 border-b border-border last:border-0">
                <span className="font-mono text-muted-foreground w-6">#{a.attempt_number}</span>
                <span className={a.error_message ? 'text-red-400' : 'text-emerald-400'}>
                  {a.error_message ? 'failed' : 'ok'}
                </span>
                {a.duration_ms != null && <span className="text-muted-foreground font-mono">{a.duration_ms}ms</span>}
                {a.exit_code != null && <span className="text-muted-foreground font-mono">exit {a.exit_code}</span>}
                {a.error_message && <span className="text-red-400/70 truncate max-w-md">{a.error_message}</span>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Progress (non-HTTP, when available) */}
      {run.progress_total != null && run.progress_total > 0 && (
        <div className="rounded-lg border border-border bg-card p-4">
          <div className="flex items-center justify-between mb-3">
            <span className="text-sm font-medium">Progress</span>
            {isRunning && <span className="text-xs text-muted-foreground font-mono">polling 5s</span>}
          </div>
          <ProgressBar
            total={run.progress_total}
            current={run.progress_current}
            progress={run.progress}
            message={run.progress_message}
          />
        </div>
      )}

      {/* Output */}
      {(stdout || stderr) && (
        <div>
          <span className="text-sm font-medium mb-3 block">Output</span>
          <OutputViewer stdout={stdout} stderr={stderr} autoScroll={isRunning} />
        </div>
      )}

      {/* Details grid */}
      <div className="rounded-lg border border-border bg-card p-4">
        <span className="text-sm font-medium mb-3 block">Details</span>
        <div className="grid grid-cols-2 gap-y-2.5 gap-x-8 text-xs">
          <Detail label="Run ID" value={run.id} mono />
          <Detail label="Process ID" value={run.process_id} mono />
          <Detail label="State" value={run.state} />
          <Detail label="Origin" value={ORIGIN_LABELS[run.origin] || run.origin} />
          <Detail label="Scheduled" value={run.scheduled_at ? new Date(run.scheduled_at).toLocaleString() : '—'} />
          <Detail label="Started" value={run.started_at ? new Date(run.started_at).toLocaleString() : '—'} />
          <Detail label="Finished" value={run.finished_at ? new Date(run.finished_at).toLocaleString() : '—'} />
          <Detail label="Duration" value={run.duration_ms ? formatDuration(run.duration_ms) : '—'} />
          <Detail label="Exit Code" value={run.exit_code != null ? String(run.exit_code) : '—'} mono />
          <Detail label="Attempt" value={`${run.attempt} / ${run.max_attempts}`} />
          {run.actor_type && <Detail label="Actor" value={`${run.actor_type}: ${run.actor_id?.slice(0, 20)}...`} />}
          {run.runtime && <Detail label="Runtime" value={run.runtime} />}
          {run.worker_id && <Detail label="Worker" value={run.worker_id} mono />}
          {run.triggered_by_run_id && <Detail label="Triggered By" value={run.triggered_by_run_id} mono />}
          {run.replayed_from_run_id && <Detail label="Replayed From" value={run.replayed_from_run_id} mono />}
        </div>
      </div>
    </div>
  )
}

function Detail({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-muted-foreground">{label}</span>
      <span className={`truncate max-w-60 ${mono ? 'font-mono' : ''}`}>{value}</span>
    </div>
  )
}
