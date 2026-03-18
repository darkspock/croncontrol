import { Plus, Play, Pause, Trash2 } from 'lucide-react'
import { TargetIcon } from '@/components/domain/target-icon'
import { SCHEDULE_LABELS } from '@/lib/constants'
import { cn } from '@/lib/utils'
import { useProcesses, useTriggerProcess } from '@/hooks/use-api'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

export function ProcessList() {
  const { data, isLoading } = useProcesses()
  const trigger = useTriggerProcess()
  const qc = useQueryClient()
  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteProcess(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['processes'] }),
  })
  const pauseMutation = useMutation({
    mutationFn: (id: string) => api.pauseProcess(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['processes'] }),
  })
  const resumeMutation = useMutation({
    mutationFn: (id: string) => api.resumeProcess(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['processes'] }),
  })

  const processes = data?.data || []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Processes</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {isLoading ? 'Loading...' : `${processes.length} configured processes`}
          </p>
        </div>
        <button
          onClick={() => { window.history.pushState(null, '', '/processes/new'); window.dispatchEvent(new PopStateEvent('popstate')) }}
          className="flex items-center gap-2 px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors"
        >
          <Plus size={14} />
          New Process
        </button>
      </div>

      <div className="rounded-lg border border-border bg-card overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-sm text-muted-foreground">Loading processes...</div>
        ) : processes.length === 0 ? (
          <div className="p-8 text-center text-sm text-muted-foreground">
            No processes yet. Click "New Process" to create one.
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Name</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Schedule</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Method</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Runtime</th>
                <th className="text-left px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                <th className="text-right px-4 py-2.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {processes.map((proc: any) => (
                <tr key={proc.id}
                  onClick={() => { window.history.pushState(null, '', `/processes/${proc.id}`); window.dispatchEvent(new PopStateEvent('popstate')) }}
                  className="hover:bg-muted/20 transition-colors cursor-pointer">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className={cn('w-1.5 h-1.5 rounded-full', proc.enabled ? 'bg-emerald-400' : 'bg-zinc-500')} />
                      <span className="text-sm font-medium">{proc.name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                        {SCHEDULE_LABELS[proc.schedule_type] || proc.schedule_type}
                      </span>
                      <span className="text-xs font-mono text-muted-foreground">
                        {proc.schedule || proc.delay_duration || '—'}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <TargetIcon method={proc.execution_method} />
                      <span className="text-xs text-muted-foreground">{proc.execution_method}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs font-mono text-muted-foreground">{proc.runtime}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={cn(
                      'text-xs font-mono px-1.5 py-0.5 rounded',
                      proc.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-500/10 text-zinc-400'
                    )}>
                      {proc.enabled ? 'enabled' : 'disabled'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => { if (confirm(`Trigger "${proc.name}" now?`)) trigger.mutate(proc.id) }}
                        className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium text-indigo-400 hover:bg-indigo-500/10 transition-colors"
                      >
                        <Play size={13} />
                        Trigger
                      </button>
                      <button
                        onClick={() => {
                          const action = proc.enabled ? 'Pause' : 'Resume'
                          if (confirm(`${action} "${proc.name}"?`))
                            proc.enabled ? pauseMutation.mutate(proc.id) : resumeMutation.mutate(proc.id)
                        }}
                        className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium text-amber-400 hover:bg-amber-500/10 transition-colors"
                      >
                        <Pause size={13} />
                        {proc.enabled ? 'Pause' : 'Resume'}
                      </button>
                      <button
                        onClick={() => { if (confirm(`Delete "${proc.name}"? This cannot be undone.`)) deleteMutation.mutate(proc.id) }}
                        className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium text-red-400 hover:bg-red-500/10 transition-colors"
                      >
                        <Trash2 size={13} />
                        Delete
                      </button>
                    </div>
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
