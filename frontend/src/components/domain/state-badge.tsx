import { cn } from '@/lib/utils'
import { STATE_COLORS } from '@/lib/constants'

interface StateBadgeProps {
  state: string
  className?: string
}

export function StateBadge({ state, className }: StateBadgeProps) {
  const colors = STATE_COLORS[state] || STATE_COLORS.cancelled

  return (
    <span className={cn(
      'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-medium font-mono uppercase tracking-wider',
      colors.bg,
      colors.text,
      className
    )}>
      <span className={cn(
        'w-1.5 h-1.5 rounded-full',
        colors.dot,
        state === 'running' && 'animate-pulse-dot'
      )} />
      {state.replace(/_/g, ' ')}
    </span>
  )
}
