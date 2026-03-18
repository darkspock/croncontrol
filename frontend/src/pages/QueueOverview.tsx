import { useState } from 'react'
import { Plus, Send } from 'lucide-react'
import { TargetIcon } from '@/components/domain/target-icon'
import { useQueues } from '@/hooks/use-api'
import { api } from '@/api/client'

export function QueueOverview() {
  const { data, isLoading } = useQueues()
  const queues = data?.data || []
  const [enqueueQueue, setEnqueueQueue] = useState<string | null>(null)
  const [payload, setPayload] = useState('{}')
  const [reference, setReference] = useState('')
  const [enqueueSuccess, setEnqueueSuccess] = useState('')

  const navigate = (path: string) => {
    window.history.pushState(null, '', path)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Queues</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {isLoading ? 'Loading...' : `${queues.length} configured queues`}
          </p>
        </div>
        <button
          onClick={() => navigate('/queues/new')}
          className="flex items-center gap-2 px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors"
        >
          <Plus size={14} /> New Queue
        </button>
      </div>

      {isLoading ? (
        <div className="p-8 text-center text-sm text-muted-foreground">Loading queues...</div>
      ) : queues.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">No queues yet. Create one to start processing background jobs.</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          {queues.map((q: any) => (
            <div key={q.id} className="rounded-lg border border-border bg-card p-4 hover:border-indigo-500/30 transition-colors cursor-pointer">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2.5">
                  <span className={`w-2 h-2 rounded-full ${q.enabled ? 'bg-emerald-400' : 'bg-zinc-500'}`} />
                  <span className="text-sm font-semibold">{q.name}</span>
                  {!q.enabled && (
                    <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-zinc-500/10 text-zinc-400">paused</span>
                  )}
                </div>
                <div className="flex items-center gap-1.5">
                  <TargetIcon method={q.execution_method} size={12} />
                  <span className="text-xs text-muted-foreground">{q.execution_method}</span>
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3 text-center mb-3">
                <div>
                  <p className="text-lg font-mono font-semibold text-indigo-400">{q.concurrency}</p>
                  <p className="text-[10px] text-muted-foreground uppercase tracking-wider">Concurrency</p>
                </div>
                <div>
                  <p className="text-lg font-mono font-semibold text-muted-foreground">{q.max_attempts}</p>
                  <p className="text-[10px] text-muted-foreground uppercase tracking-wider">Max Attempts</p>
                </div>
                <div>
                  <p className="text-lg font-mono font-semibold text-muted-foreground">{q.runtime}</p>
                  <p className="text-[10px] text-muted-foreground uppercase tracking-wider">Runtime</p>
                </div>
              </div>
              <button
                onClick={(e) => { e.stopPropagation(); setEnqueueQueue(q.id); setEnqueueSuccess('') }}
                className="w-full flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-xs text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              >
                <Send size={12} /> Enqueue Job
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Enqueue dialog */}
      {enqueueQueue && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setEnqueueQueue(null)}>
          <div className="bg-card border border-border rounded-lg p-6 w-full max-w-md space-y-4" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-sm font-semibold">Enqueue Job</h3>
            {enqueueSuccess && <div className="text-xs text-emerald-400 bg-emerald-500/10 rounded p-2">{enqueueSuccess}</div>}
            <div className="space-y-1.5">
              <label className="text-xs text-muted-foreground">Payload (JSON)</label>
              <textarea value={payload} onChange={(e) => setPayload(e.target.value)} rows={4}
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-xs font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs text-muted-foreground">Reference (optional)</label>
              <input value={reference} onChange={(e) => setReference(e.target.value)} placeholder="order-123"
                className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
            </div>
            <div className="flex gap-2">
              <button
                onClick={async () => {
                  try {
                    const parsed = JSON.parse(payload)
                    await api.enqueueJob({ queue_id: enqueueQueue, payload: parsed, reference: reference || undefined })
                    setEnqueueSuccess('Job enqueued!')
                    setPayload('{}')
                    setReference('')
                  } catch (err: any) { setEnqueueSuccess('Error: ' + err.message) }
                }}
                className="px-3 py-1.5 rounded-md bg-indigo-500 text-white text-sm hover:bg-indigo-400 transition-colors"
              >Enqueue</button>
              <button onClick={() => setEnqueueQueue(null)} className="px-3 py-1.5 rounded-md border border-border text-sm">Cancel</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
