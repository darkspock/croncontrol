import { useState, useMemo } from 'react'
import { ArrowLeft } from 'lucide-react'
import cronstrue from 'cronstrue'
import { api } from '@/api/client'
import { SCHEDULE_LABELS } from '@/lib/constants'

const METHODS = ['http', 'ssh', 'ssm', 'k8s'] as const
const SCHEDULE_TYPES = ['cron', 'fixed_delay', 'on_demand'] as const
const HTTP_DISPATCH_MODES = ['sync', 'async_blind', 'async_tracked'] as const
const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const

function parseNumberList(value: string): number[] | undefined {
  const items = value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  if (!items.length) return undefined
  return items.map((item) => {
    const parsed = Number(item)
    if (!Number.isInteger(parsed)) {
      throw new Error(`Invalid numeric value "${item}"`)
    }
    return parsed
  })
}

function parseStringList(value: string): string[] | undefined {
  const items = value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  return items.length ? items : undefined
}

function buildHttpMethodConfig(input: {
  url: string
  method: string
  dispatchMode: string
  acceptedStatusCodes: string
  statusURLField: string
  statusURLTemplate: string
  cancelURLField: string
  cancelURLTemplate: string
  jobIDField: string
  pollMethod: string
  cancelMethod: string
  statusField: string
  runningValues: string
  successValues: string
  failedValues: string
}) {
  const methodConfig: Record<string, any> = {
    url: input.url,
    method: input.method,
    dispatch_mode: input.dispatchMode,
  }

  const acceptedStatusCodes = parseNumberList(input.acceptedStatusCodes)
  if (acceptedStatusCodes?.length) {
    methodConfig.accepted_status_codes = acceptedStatusCodes
  }

  if (input.dispatchMode === 'async_tracked') {
    if (input.statusURLField.trim()) methodConfig.status_url_field = input.statusURLField.trim()
    if (input.statusURLTemplate.trim()) methodConfig.status_url_template = input.statusURLTemplate.trim()
    if (input.cancelURLField.trim()) methodConfig.cancel_url_field = input.cancelURLField.trim()
    if (input.cancelURLTemplate.trim()) methodConfig.cancel_url_template = input.cancelURLTemplate.trim()
    if (input.jobIDField.trim()) methodConfig.job_id_field = input.jobIDField.trim()
    if (input.statusField.trim()) methodConfig.status_field = input.statusField.trim()

    const runningValues = parseStringList(input.runningValues)
    if (runningValues?.length) methodConfig.running_values = runningValues

    const successValues = parseStringList(input.successValues)
    if (successValues?.length) methodConfig.success_values = successValues

    const failedValues = parseStringList(input.failedValues)
    if (failedValues?.length) methodConfig.failed_values = failedValues

    if (input.pollMethod.trim()) methodConfig.poll_method = input.pollMethod.trim().toUpperCase()
    if (input.cancelMethod.trim()) methodConfig.cancel_method = input.cancelMethod.trim().toUpperCase()
  }

  return methodConfig
}

export function ProcessCreate() {
  const [name, setName] = useState('')
  const [scheduleType, setScheduleType] = useState<string>('cron')
  const [schedule, setSchedule] = useState('*/5 * * * *')
  const [delayDuration, setDelayDuration] = useState('5m')
  const [runtime, setRuntime] = useState<string>('direct')
  const [method, setMethod] = useState<string>('http')
  const [url, setUrl] = useState('')
  const [httpMethod, setHttpMethod] = useState('POST')
  const [dispatchMode, setDispatchMode] = useState<string>('sync')
  const [acceptedStatusCodes, setAcceptedStatusCodes] = useState('200,201,202')
  const [statusURLField, setStatusURLField] = useState('')
  const [statusURLTemplate, setStatusURLTemplate] = useState('')
  const [cancelURLField, setCancelURLField] = useState('')
  const [cancelURLTemplate, setCancelURLTemplate] = useState('')
  const [jobIDField, setJobIDField] = useState('')
  const [pollMethod, setPollMethod] = useState('GET')
  const [cancelMethod, setCancelMethod] = useState('POST')
  const [statusField, setStatusField] = useState('')
  const [runningValues, setRunningValues] = useState('')
  const [successValues, setSuccessValues] = useState('')
  const [failedValues, setFailedValues] = useState('')
  const [command, setCommand] = useState('')
  const [workerId, setWorkerId] = useState('')
  const [workerLabels, setWorkerLabels] = useState('')
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
        Object.assign(methodConfig, buildHttpMethodConfig({
          url,
          method: httpMethod,
          dispatchMode,
          acceptedStatusCodes,
          statusURLField,
          statusURLTemplate,
          cancelURLField,
          cancelURLTemplate,
          jobIDField,
          pollMethod,
          cancelMethod,
          statusField,
          runningValues,
          successValues,
          failedValues,
        }))
      } else {
        methodConfig.command = command
      }

      let parsedWorkerLabels: any = undefined
      if (workerLabels.trim()) {
        parsedWorkerLabels = JSON.parse(workerLabels)
      }

      await api.createProcess({
        name,
        schedule_type: scheduleType,
        schedule: scheduleType === 'cron' ? schedule : undefined,
        delay_duration: scheduleType === 'fixed_delay' ? delayDuration : undefined,
        execution_method: method,
        runtime,
        method_config: methodConfig,
        worker_id: runtime === 'worker' && workerId.trim() ? workerId.trim() : undefined,
        worker_labels: runtime === 'worker' ? parsedWorkerLabels : undefined,
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
        <div className="grid grid-cols-2 gap-4">
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
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Runtime</label>
            <div className="flex gap-2">
              {['direct', 'worker'].map((value) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setRuntime(value)}
                  className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    runtime === value
                      ? 'bg-indigo-500 text-white'
                      : 'bg-muted text-muted-foreground hover:text-foreground'
                  }`}
                >
                  {value}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Method config */}
        {method === 'http' && (
          <div className="space-y-3 rounded-lg border border-border p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">HTTP Configuration</p>
            <div className="grid grid-cols-3 gap-3">
              <div className="w-28">
                <label className="text-xs text-muted-foreground">Method</label>
                <select
                  value={httpMethod}
                  onChange={(e) => setHttpMethod(e.target.value)}
                  className="w-full mt-1 px-2 py-1.5 rounded-md border border-border bg-background text-sm"
                >
                  {HTTP_METHODS.map((value) => (
                    <option key={value}>{value}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Dispatch Mode</label>
                <select
                  value={dispatchMode}
                  onChange={(e) => setDispatchMode(e.target.value)}
                  className="w-full mt-1 px-2 py-1.5 rounded-md border border-border bg-background text-sm"
                >
                  {HTTP_DISPATCH_MODES.map((value) => (
                    <option key={value} value={value}>{value}</option>
                  ))}
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
            {dispatchMode !== 'sync' && (
              <div className="space-y-1">
                <label className="text-xs text-muted-foreground">Accepted Status Codes</label>
                <input
                  type="text"
                  value={acceptedStatusCodes}
                  onChange={(e) => setAcceptedStatusCodes(e.target.value)}
                  placeholder="200,201,202"
                  className="w-full px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                />
                <p className="text-xs text-muted-foreground">Dispatch response codes treated as accepted by CronControl.</p>
              </div>
            )}
            {dispatchMode === 'async_tracked' && (
              <>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Job ID Field</label>
                    <input
                      type="text"
                      value={jobIDField}
                      onChange={(e) => setJobIDField(e.target.value)}
                      placeholder="data.id"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Status Field</label>
                    <input
                      type="text"
                      value={statusField}
                      onChange={(e) => setStatusField(e.target.value)}
                      placeholder="data.status"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Status URL Field</label>
                    <input
                      type="text"
                      value={statusURLField}
                      onChange={(e) => setStatusURLField(e.target.value)}
                      placeholder="links.status"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Status URL Template</label>
                    <input
                      type="text"
                      value={statusURLTemplate}
                      onChange={(e) => setStatusURLTemplate(e.target.value)}
                      placeholder="https://api.example.com/jobs/{{job.id}}"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Cancel URL Field</label>
                    <input
                      type="text"
                      value={cancelURLField}
                      onChange={(e) => setCancelURLField(e.target.value)}
                      placeholder="links.cancel"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Cancel URL Template</label>
                    <input
                      type="text"
                      value={cancelURLTemplate}
                      onChange={(e) => setCancelURLTemplate(e.target.value)}
                      placeholder="https://api.example.com/jobs/{{job.id}}/cancel"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Poll Method</label>
                    <select
                      value={pollMethod}
                      onChange={(e) => setPollMethod(e.target.value)}
                      className="w-full mt-1 px-2 py-1.5 rounded-md border border-border bg-background text-sm"
                    >
                      {HTTP_METHODS.map((value) => (
                        <option key={value}>{value}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Cancel Method</label>
                    <select
                      value={cancelMethod}
                      onChange={(e) => setCancelMethod(e.target.value)}
                      className="w-full mt-1 px-2 py-1.5 rounded-md border border-border bg-background text-sm"
                    >
                      {HTTP_METHODS.map((value) => (
                        <option key={value}>{value}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Running Values</label>
                    <input
                      type="text"
                      value={runningValues}
                      onChange={(e) => setRunningValues(e.target.value)}
                      placeholder="queued,running"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Success Values</label>
                    <input
                      type="text"
                      value={successValues}
                      onChange={(e) => setSuccessValues(e.target.value)}
                      placeholder="completed,succeeded"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Failed Values</label>
                    <input
                      type="text"
                      value={failedValues}
                      onChange={(e) => setFailedValues(e.target.value)}
                      placeholder="failed,error"
                      className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
                    />
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">
                  Tracked mode requires a status URL, either extracted from the dispatch response or built with a template.
                </p>
              </>
            )}
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

        {runtime === 'worker' && (
          <div className="space-y-3 rounded-lg border border-border p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Worker Routing</p>
            <div>
              <label className="text-xs text-muted-foreground">Worker ID (optional)</label>
              <input
                type="text"
                value={workerId}
                onChange={(e) => setWorkerId(e.target.value)}
                placeholder="wrk_..."
                className="w-full mt-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
              />
              <p className="text-xs text-muted-foreground mt-1">If empty, matching uses worker labels and capacity.</p>
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Worker Labels JSON (optional)</label>
              <textarea
                value={workerLabels}
                onChange={(e) => setWorkerLabels(e.target.value)}
                placeholder='["linux","prod"] or {"region":"eu-west-1"}'
                rows={3}
                className="w-full mt-1 px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40"
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
