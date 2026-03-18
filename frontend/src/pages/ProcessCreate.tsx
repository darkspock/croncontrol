import { useState, useMemo } from 'react'
import { ArrowLeft } from 'lucide-react'
import cronstrue from 'cronstrue'
import { api } from '@/api/client'
import { SCHEDULE_LABELS } from '@/lib/constants'

const METHODS = ['http', 'ssh', 'ssm', 'k8s'] as const
const SCHEDULE_TYPES = ['cron', 'fixed_delay', 'on_demand'] as const

export function ProcessCreate() {
  const [name, setName] = useState('')
  const [scheduleType, setScheduleType] = useState<string>('cron')
  const [schedule, setSchedule] = useState('*/5 * * * *')
  const [delayDuration, setDelayDuration] = useState('5m')
  const [method, setMethod] = useState<string>('http')
  const [url, setUrl] = useState('')
  const [httpMethod, setHttpMethod] = useState('POST')
  const [command, setCommand] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setSubmitting(true)

    try {
      const methodConfig: any = {}
      if (method === 'http') {
        methodConfig.url = url
        methodConfig.method = httpMethod
      } else {
        methodConfig.command = command
      }

      await api.createProcess({
        name,
        schedule_type: scheduleType,
        schedule: scheduleType === 'cron' ? schedule : undefined,
        delay_duration: scheduleType === 'fixed_delay' ? delayDuration : undefined,
        execution_method: method,
        method_config: methodConfig,
      })
      setSuccess(`Process "${name}" created successfully!`)
      setName('')
    } catch (err: any) {
      setError(err.message || 'Failed to create process')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl space-y-6">
      <div>
        <button
          onClick={() => history.back()}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mb-2"
        >
          <ArrowLeft size={12} /> Back
        </button>
        <h1 className="text-xl font-semibold tracking-tight">Create Process</h1>
        <p className="text-sm text-muted-foreground mt-1">Configure a new scheduled or on-demand process</p>
      </div>

      {error && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-400">{error}</div>
      )}
      {success && (
        <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-sm text-emerald-400">{success}</div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Name */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Process Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="my-cron-job"
            required
            className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
          />
        </div>

        {/* Schedule Type */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Schedule Type</label>
          <div className="flex gap-2">
            {SCHEDULE_TYPES.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => setScheduleType(t)}
                className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                  scheduleType === t
                    ? 'bg-indigo-500 text-white'
                    : 'bg-muted text-muted-foreground hover:text-foreground'
                }`}
              >
                {SCHEDULE_LABELS[t]}
              </button>
            ))}
          </div>
        </div>

        {/* Schedule config */}
        {scheduleType === 'cron' && (
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Cron Expression</label>
            <input
              type="text"
              value={schedule}
              onChange={(e) => setSchedule(e.target.value)}
              placeholder="*/5 * * * *"
              className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
            />
            <CronPreview expression={schedule} />
          </div>
        )}

        {scheduleType === 'fixed_delay' && (
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Delay Duration</label>
            <input
              type="text"
              value={delayDuration}
              onChange={(e) => setDelayDuration(e.target.value)}
              placeholder="5m"
              className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
            />
            <p className="text-xs text-muted-foreground">Go duration: 30s, 5m, 1h, etc.</p>
          </div>
        )}

        {/* Execution Method */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Execution Method</label>
          <div className="flex gap-2">
            {METHODS.map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setMethod(m)}
                className={`px-3 py-1.5 rounded-md text-sm font-medium uppercase transition-colors ${
                  method === m
                    ? 'bg-indigo-500 text-white'
                    : 'bg-muted text-muted-foreground hover:text-foreground'
                }`}
              >
                {m}
              </button>
            ))}
          </div>
        </div>

        {/* Method config */}
        {method === 'http' && (
          <div className="space-y-3 rounded-lg border border-border p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">HTTP Configuration</p>
            <div className="flex gap-3">
              <div className="w-28">
                <label className="text-xs text-muted-foreground">Method</label>
                <select
                  value={httpMethod}
                  onChange={(e) => setHttpMethod(e.target.value)}
                  className="w-full mt-1 px-2 py-1.5 rounded-md border border-border bg-background text-sm"
                >
                  <option>GET</option>
                  <option>POST</option>
                  <option>PUT</option>
                  <option>PATCH</option>
                  <option>DELETE</option>
                </select>
              </div>
              <div className="flex-1">
                <label className="text-xs text-muted-foreground">URL</label>
                <input
                  type="url"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://api.example.com/webhook"
                  required
                  className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                />
              </div>
            </div>
          </div>
        )}

        {(method === 'ssh' || method === 'ssm' || method === 'k8s') && (
          <div className="space-y-3 rounded-lg border border-border p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{method.toUpperCase()} Configuration</p>
            <div>
              <label className="text-xs text-muted-foreground">Command</label>
              <input
                type="text"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder="php /app/cron/my-task.php"
                className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
              />
            </div>
          </div>
        )}

        {/* Submit */}
        <button
          type="submit"
          disabled={submitting || !name}
          className="px-4 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {submitting ? 'Creating...' : 'Create Process'}
        </button>
      </form>
    </div>
  )
}

function CronPreview({ expression }: { expression: string }) {
  const description = useMemo(() => {
    if (!expression || !expression.trim()) return null
    try {
      return cronstrue.toString(expression, { locale: 'en', use24HourTimeFormat: true })
    } catch {
      return null
    }
  }, [expression])

  return (
    <div className="text-xs text-muted-foreground mt-1">
      {description ? (
        <span className="text-indigo-400">{description}</span>
      ) : (
        <span>Standard 5-field cron syntax (minute hour dom month dow)</span>
      )}
    </div>
  )
}
