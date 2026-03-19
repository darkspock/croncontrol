import { useState, useEffect } from 'react'
import { Music } from 'lucide-react'
import { StateBadge } from '@/components/domain/state-badge'
import { formatTimeAgo } from '@/lib/utils'

export function OrchestraList() {
  const [orchestras, setOrchestras] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/orchestras', { headers: { 'X-API-Key': localStorage.getItem('cc_api_key') || '' } })
      .then(r => r.json())
      .then(d => { setOrchestras(d.data || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const navigate = (path: string) => {
    window.history.pushState(null, '', path)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Orchestras</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {loading ? 'Loading...' : `${orchestras.length} orchestras`}
          </p>
        </div>
      </div>

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : orchestras.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center">
          <Music size={32} className="mx-auto text-muted-foreground mb-3" />
          <p className="text-sm text-muted-foreground">No orchestras yet. Create one via the SDK or API.</p>
          <code className="block mt-3 text-xs font-mono text-indigo-400">POST /api/v1/orchestras</code>
        </div>
      ) : (
        <div className="space-y-3">
          {orchestras.map((orch: any) => (
            <button
              key={orch.id}
              onClick={() => navigate(`/orchestras/${orch.id}`)}
              className="w-full text-left rounded-lg border border-border bg-card p-4 hover:border-indigo-500/30 transition-colors"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Music size={16} className="text-indigo-400" />
                  <div>
                    <p className="text-sm font-semibold">{orch.name}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {orch.director_type === 'ai' ? 'AI Director' : orch.director_type === 'process' ? 'Code Director' : 'No Director'}
                      {' · '}{orch.movement_count} movements
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <StateBadge state={orch.state} />
                  <span className="text-xs text-muted-foreground">{formatTimeAgo(orch.created_at)}</span>
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
