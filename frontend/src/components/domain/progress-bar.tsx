import { cn } from '@/lib/utils'

interface ProgressBarProps {
  total?: number | null
  current?: number | null
  progress?: number | null
  message?: string | null
  className?: string
}

export function ProgressBar({ total, current, progress, message, className }: ProgressBarProps) {
  const pct = progress ?? (total && current ? Math.round((current / total) * 100) : 0)

  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex items-center justify-between text-xs">
        <span className="font-mono text-muted-foreground">
          {current != null && total != null ? `${current.toLocaleString()} / ${total.toLocaleString()}` : '—'}
        </span>
        <span className="font-mono font-medium text-indigo-400">{pct}%</span>
      </div>
      <div className="h-2 bg-muted rounded-full overflow-hidden">
        <div
          className="h-full bg-indigo-500 rounded-full transition-all duration-500 ease-out"
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
      {message && (
        <p className="text-xs text-muted-foreground font-mono truncate">{message}</p>
      )}
    </div>
  )
}
