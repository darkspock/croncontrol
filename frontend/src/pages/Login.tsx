import { useState, useEffect } from 'react'
import { Zap } from 'lucide-react'

type Mode = 'login' | 'register' | 'apikey'

export function Login() {
  const [mode, setMode] = useState<Mode>('login')
  const [googleOAuth, setGoogleOAuth] = useState(false)

  useEffect(() => {
    fetch('/api/v1/config').then(r => r.json()).then(d => {
      setGoogleOAuth(d.data?.google_oauth_enabled === true)
    }).catch(() => {})
  }, [])
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await fetch('/api/v1/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error?.message || 'Login failed')
      localStorage.setItem('cc_api_key', data.data.api_key)
      window.location.href = '/'
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)
    try {
      const res = await fetch('/api/v1/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, name }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error?.message || 'Registration failed')
      localStorage.setItem('cc_api_key', data.data.api_key)
      setSuccess(`Workspace "${data.data.workspace.slug}" created! Redirecting...`)
      setTimeout(() => { window.location.href = '/' }, 1500)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleApiKey = (e: React.FormEvent) => {
    e.preventDefault()
    if (apiKey.trim()) {
      localStorage.setItem('cc_api_key', apiKey.trim())
      window.location.href = '/'
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="absolute inset-0 overflow-hidden">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-indigo-500/5 rounded-full blur-3xl" />
      </div>

      <div className="relative w-full max-w-sm space-y-8">
        {/* Logo */}
        <div className="flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-indigo-500 flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <Zap size={24} className="text-white" />
          </div>
          <div className="text-center">
            <h1 className="text-xl font-semibold tracking-tight">CronControl</h1>
            <p className="text-sm text-muted-foreground mt-1">Control plane for operational workloads</p>
          </div>
        </div>

        {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-400">{error}</div>}
        {success && <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-sm text-emerald-400">{success}</div>}

        <div className="rounded-lg border border-border bg-card p-6 space-y-4">
          {/* Mode tabs */}
          <div className="flex gap-1 bg-muted rounded-md p-0.5">
            {[
              { id: 'login' as Mode, label: 'Sign In' },
              { id: 'register' as Mode, label: 'Register' },
              { id: 'apikey' as Mode, label: 'API Key' },
            ].map((t) => (
              <button
                key={t.id}
                onClick={() => { setMode(t.id); setError('') }}
                className={`flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                  mode === t.id ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          {/* Login form */}
          {mode === 'login' && (
            <form onSubmit={handleLogin} className="space-y-3">
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="Email" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <button type="submit" disabled={loading}
                className="w-full px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50">
                {loading ? 'Signing in...' : 'Sign In'}
              </button>
            </form>
          )}

          {/* Register form */}
          {mode === 'register' && (
            <form onSubmit={handleRegister} className="space-y-3">
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="Workspace name" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="Email" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password (min 12 chars)" minLength={12}
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <button type="submit" disabled={loading}
                className="w-full px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50">
                {loading ? 'Creating workspace...' : 'Create Workspace'}
              </button>
            </form>
          )}

          {/* API Key form */}
          {mode === 'apikey' && (
            <form onSubmit={handleApiKey} className="space-y-3">
              <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="cc_live_..." required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <button type="submit"
                className="w-full px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors">
                Sign In with Key
              </button>
            </form>
          )}

          {/* Google OAuth — only shows if configured on server */}
          {mode === 'login' && googleOAuth && (
            <div className="pt-2 border-t border-border">
              <a href="/api/v1/auth/google/login"
                className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-md border border-border bg-background text-sm font-medium hover:bg-muted/50 transition-colors">
                <svg width="16" height="16" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
                Sign in with Google
              </a>
            </div>
          )}

          {mode === 'login' && (
            <div className="text-center pt-2">
              <a href="#" onClick={(e) => { e.preventDefault(); window.location.href = '/forgot-password' }}
                className="text-xs text-muted-foreground hover:text-indigo-400 transition-colors">Forgot password?</a>
            </div>
          )}
        </div>

        <p className="text-center text-xs text-muted-foreground">
          Open source · MIT License · <a href="https://github.com/darkspock/croncontrol" className="text-indigo-400 hover:underline">GitHub</a>
        </p>
      </div>
    </div>
  )
}
