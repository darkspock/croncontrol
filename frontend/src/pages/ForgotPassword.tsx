import { useState } from 'react'
import { Zap, ArrowLeft } from 'lucide-react'

export function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Check if this is a reset page (has token)
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')

  if (token) {
    return <ResetPassword token={token} />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await fetch('/api/v1/auth/forgot-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error?.message || 'Request failed')
      }
      setSent(true)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-indigo-500 flex items-center justify-center">
            <Zap size={24} className="text-white" />
          </div>
          <h1 className="text-xl font-semibold">Reset Password</h1>
          <p className="text-sm text-muted-foreground">Enter your email to receive a reset link</p>
        </div>

        {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-400">{error}</div>}

        <div className="rounded-lg border border-border bg-card p-6">
          {sent ? (
            <div className="text-center space-y-3">
              <p className="text-sm text-emerald-400">If an account exists with that email, a reset link has been sent.</p>
              <p className="text-xs text-muted-foreground">Check your inbox and spam folder.</p>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-3">
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="Email" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <button type="submit" disabled={loading}
                className="w-full px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50">
                {loading ? 'Sending...' : 'Send Reset Link'}
              </button>
            </form>
          )}
        </div>

        <div className="text-center">
          <a href="/" className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-indigo-400 transition-colors">
            <ArrowLeft size={12} /> Back to sign in
          </a>
        </div>
      </div>
    </div>
  )
}

function ResetPassword({ token }: { token: string }) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 12) {
      setError('Password must be at least 12 characters')
      return
    }
    setError('')
    setLoading(true)
    try {
      const res = await fetch('/api/v1/auth/reset-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: password }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error?.message || 'Reset failed')
      }
      setSuccess(true)
      setTimeout(() => { window.location.href = '/' }, 2000)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-indigo-500 flex items-center justify-center">
            <Zap size={24} className="text-white" />
          </div>
          <h1 className="text-xl font-semibold">Set New Password</h1>
        </div>

        {error && <div className="rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-sm text-red-400">{error}</div>}

        <div className="rounded-lg border border-border bg-card p-6">
          {success ? (
            <p className="text-center text-sm text-emerald-400">Password reset! Redirecting to sign in...</p>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-3">
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
                placeholder="New password (min 12 chars)" minLength={12} required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)}
                placeholder="Confirm password" required
                className="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/40" />
              <button type="submit" disabled={loading}
                className="w-full px-3 py-2 rounded-md bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-400 transition-colors disabled:opacity-50">
                {loading ? 'Resetting...' : 'Reset Password'}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
