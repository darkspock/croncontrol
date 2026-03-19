import { Music, ExternalLink } from 'lucide-react'

export function OrchestraList() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Orchestras</h1>
        <p className="text-sm text-muted-foreground mt-1">Multi-step workflows with AI Director</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-10 text-center space-y-4">
        <div className="w-14 h-14 rounded-2xl bg-indigo-500/10 flex items-center justify-center mx-auto">
          <Music size={28} className="text-indigo-400" />
        </div>
        <div>
          <span className="inline-block px-3 py-1 text-xs font-medium rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20">
            Coming Soon
          </span>
        </div>
        <h2 className="text-lg font-semibold">Orchestras are in development</h2>
        <p className="text-sm text-muted-foreground max-w-md mx-auto leading-relaxed">
          A Director (code or AI) coordinates AgentNodes through dynamic, multi-step workflows.
          Human-in-the-loop decisions, real-time chat, container execution, and budget controls — all built in.
        </p>
        <div className="pt-2">
          <a
            href="https://croncontrol.dev/orchestras/"
            target="_blank"
            rel="noopener"
            className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-indigo-400 hover:text-indigo-300 border border-indigo-500/20 hover:border-indigo-500/40 rounded-lg transition-colors"
          >
            Learn more <ExternalLink size={14} />
          </a>
        </div>
      </div>
    </div>
  )
}
