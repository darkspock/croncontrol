import { ArrowLeft } from 'lucide-react'

export function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] space-y-4">
      <p className="text-6xl font-mono font-bold text-muted-foreground/30">404</p>
      <p className="text-sm text-muted-foreground">Page not found</p>
      <button
        onClick={() => { window.history.pushState(null, '', '/'); window.dispatchEvent(new PopStateEvent('popstate')) }}
        className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border text-sm hover:bg-muted/50 transition-colors"
      >
        <ArrowLeft size={13} /> Back to Dashboard
      </button>
    </div>
  )
}
