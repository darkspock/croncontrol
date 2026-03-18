import { useState, useEffect } from 'react'
import { Rocket, X, ArrowRight, Cpu, Globe, Play } from 'lucide-react'
import { api } from '@/api/client'

interface OnboardingStep {
  id: string
  icon: React.ElementType
  title: string
  description: string
  action: string
  href: string
  check: (data: any) => boolean
}

const steps: OnboardingStep[] = [
  {
    id: 'process',
    icon: Cpu,
    title: 'Create your first process',
    description: 'Set up a scheduled job with a cron expression or fixed delay.',
    action: 'Create process',
    href: '/processes/new',
    check: (data) => (data.processes?.data?.length || 0) > 0,
  },
  {
    id: 'run',
    icon: Play,
    title: 'Trigger a test run',
    description: 'Execute your process manually to verify the configuration.',
    action: 'View processes',
    href: '/processes',
    check: (data) => (data.runs?.data?.length || 0) > 0,
  },
  {
    id: 'queue',
    icon: Globe,
    title: 'Set up a queue',
    description: 'Create a durable queue for event-driven background jobs.',
    action: 'Create queue',
    href: '/queues/new',
    check: (data) => (data.queues?.data?.length || 0) > 0,
  },
]

export function OnboardingBanner() {
  const [dismissed, setDismissed] = useState(false)
  const [completedSteps, setCompletedSteps] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (localStorage.getItem('cc_onboarding_dismissed') === 'true') {
      setDismissed(true)
      setLoading(false)
      return
    }

    Promise.all([
      api.listProcesses().catch(() => ({ data: [] })),
      api.listRuns().catch(() => ({ data: [] })),
      api.listQueues().catch(() => ({ data: [] })),
    ]).then(([processes, runs, queues]) => {
      const data = { processes, runs, queues }
      const completed = new Set<string>()
      for (const step of steps) {
        if (step.check(data)) completed.add(step.id)
      }
      setCompletedSteps(completed)
      if (completed.size === steps.length) {
        setDismissed(true)
        localStorage.setItem('cc_onboarding_dismissed', 'true')
      }
      setLoading(false)
    })
  }, [])

  if (dismissed || loading) return null

  const nextStep = steps.find(s => !completedSteps.has(s.id))
  const progress = completedSteps.size / steps.length

  return (
    <div className="relative rounded-lg border border-indigo-500/20 bg-indigo-500/5 p-4 mb-6">
      <button
        onClick={() => {
          setDismissed(true)
          localStorage.setItem('cc_onboarding_dismissed', 'true')
        }}
        className="absolute top-3 right-3 text-muted-foreground hover:text-foreground"
      >
        <X size={14} />
      </button>

      <div className="flex items-center gap-2 mb-3">
        <Rocket size={16} className="text-indigo-400" />
        <span className="text-sm font-medium text-foreground">Getting Started</span>
        <span className="text-xs text-muted-foreground ml-auto">
          {completedSteps.size}/{steps.length} completed
        </span>
      </div>

      {/* Progress bar */}
      <div className="h-1 bg-muted rounded-full mb-4">
        <div
          className="h-1 bg-indigo-500 rounded-full transition-all duration-500"
          style={{ width: `${progress * 100}%` }}
        />
      </div>

      <div className="grid grid-cols-3 gap-3">
        {steps.map((step) => {
          const isComplete = completedSteps.has(step.id)
          const isNext = step === nextStep

          return (
            <div
              key={step.id}
              className={`rounded-md border p-3 transition-colors ${
                isComplete
                  ? 'border-emerald-500/30 bg-emerald-500/5'
                  : isNext
                    ? 'border-indigo-500/30 bg-indigo-500/5'
                    : 'border-border bg-background/50'
              }`}
            >
              <div className="flex items-center gap-2 mb-1">
                <step.icon size={14} className={isComplete ? 'text-emerald-400' : 'text-muted-foreground'} />
                <span className={`text-xs font-medium ${isComplete ? 'text-emerald-400 line-through' : 'text-foreground'}`}>
                  {step.title}
                </span>
              </div>
              <p className="text-xs text-muted-foreground mb-2">{step.description}</p>
              {!isComplete && (
                <a
                  href={step.href}
                  className="inline-flex items-center gap-1 text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
                >
                  {step.action} <ArrowRight size={10} />
                </a>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
