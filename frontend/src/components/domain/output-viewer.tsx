import { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'

interface OutputViewerProps {
  stdout?: string
  stderr?: string
  autoScroll?: boolean
  className?: string
}

export function OutputViewer({ stdout, stderr, autoScroll = true, className }: OutputViewerProps) {
  const [tab, setTab] = useState<'stdout' | 'stderr'>('stdout')
  const scrollRef = useRef<HTMLPreElement>(null)

  const content = tab === 'stdout' ? stdout : stderr
  const hasStderr = stderr && stderr.length > 0

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [content, autoScroll])

  return (
    <div className={cn('rounded-lg border border-border bg-[#0a0a0c] overflow-hidden', className)}>
      <div className="flex items-center gap-1 px-3 py-2 border-b border-border bg-muted/30">
        <button
          onClick={() => setTab('stdout')}
          className={cn(
            'px-2.5 py-1 rounded text-xs font-mono transition-colors',
            tab === 'stdout' ? 'bg-indigo-500/20 text-indigo-400' : 'text-muted-foreground hover:text-foreground'
          )}
        >
          stdout
        </button>
        {hasStderr && (
          <button
            onClick={() => setTab('stderr')}
            className={cn(
              'px-2.5 py-1 rounded text-xs font-mono transition-colors',
              tab === 'stderr' ? 'bg-red-500/20 text-red-400' : 'text-muted-foreground hover:text-foreground'
            )}
          >
            stderr
          </button>
        )}
      </div>
      <pre
        ref={scrollRef}
        className="p-4 text-xs font-mono text-zinc-300 leading-relaxed overflow-auto max-h-96 output-scroll whitespace-pre-wrap"
      >
        {content || <span className="text-muted-foreground italic">No output</span>}
      </pre>
    </div>
  )
}
