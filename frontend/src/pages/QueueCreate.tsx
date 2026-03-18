import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { api } from '@/api/client'

const METHODS = ['http', 'ssh', 'ssm', 'k8s'] as const

export function QueueCreate() {
  const [name, setName] = useState('')
  const [method, setMethod] = useState<string>('http')
  const [url, setUrl] = useState('')
  const [concurrency, setConcurrency] = useState(1)
  const [maxAttempts, setMaxAttempts] = useState(3)
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
      if (method === 'http' && url) {
        methodConfig.url = url
        methodConfig.method = 'POST'
      }
      await api.createQueue({
        name,
        execution_method: method,
        method_config: methodConfig,
        concurrency,
        max_attempts: maxAttempts,
      })
      setSuccess(`Queue "${name}" created!`)
      setName('')
    } catch (err: any) {
      setError(err.message || 'Failed to create queue')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl space-y-6">
      <div>
        <button onClick={() => history.back()} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mb-2">
          <ArrowLeft size={12} /> Back
        </button>
        <h1 className="text-xl font-semibold tracking-tight">Create Queue</h1>
        <p className="text-sm text-muted-foreground mt-1">Configure a new job queue</p>
      </div>

      {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-400">{error}</div>}
      {success && <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-sm text-emerald-400">{success}</div>}

      <form onSubmit={handleSubmit} className="space-y-5">
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Queue Name</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="emails" required
            className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
        </div>

        <div className="space-y-1.5">
          <label className="text-sm font-medium">Execution Method</label>
          <div className="flex gap-2">
            {METHODS.map((m) => (
              <button key={m} type="button" onClick={() => setMethod(m)}
                className={`px-3 py-1.5 rounded-md text-sm font-medium uppercase transition-colors ${method === m ? 'bg-indigo-500 text-white' : 'bg-muted text-muted-foreground hover:text-foreground'}`}>
                {m}
              </button>
            ))}
          </div>
        </div>

        {method === 'http' && (
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Default URL</label>
            <input type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://api.example.com/webhook"
              className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Concurrency</label>
            <input type="number" value={concurrency} onChange={(e) => setConcurrency(Number(e.target.value))} min={1} max={100}
              className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
            <p className="text-xs text-muted-foreground">Max parallel jobs</p>
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Max Attempts</label>
            <input type="number" value={maxAttempts} onChange={(e) => setMaxAttempts(Number(e.target.value))} min={1} max={10}
              className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
            <p className="text-xs text-muted-foreground">Including first attempt</p>
          </div>
        </div>

        <button type="submit" disabled={submitting || !name}
          className="px-4 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50">
          {submitting ? 'Creating...' : 'Create Queue'}
        </button>
      </form>
    </div>
  )
}
