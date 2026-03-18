import { useState, useEffect } from 'react'
import { Zap, CheckCircle, XCircle } from 'lucide-react'

export function VerifyEmail() {
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [message, setMessage] = useState('')

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')

    if (!token) {
      setStatus('error')
      setMessage('Missing verification token')
      return
    }

    fetch('/api/v1/register/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    })
      .then(async (res) => {
        if (res.ok) {
          setStatus('success')
          setMessage('Email verified! Redirecting...')
          setTimeout(() => { window.location.href = '/' }, 2000)
        } else {
          const data = await res.json()
          setStatus('error')
          setMessage(data.error?.message || 'Verification failed')
        }
      })
      .catch(() => {
        setStatus('error')
        setMessage('Network error')
      })
  }, [])

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-sm text-center space-y-6">
        <div className="flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-indigo-500 flex items-center justify-center">
            <Zap size={24} className="text-white" />
          </div>
          <h1 className="text-xl font-semibold">Email Verification</h1>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          {status === 'loading' && (
            <p className="text-sm text-muted-foreground">Verifying your email...</p>
          )}
          {status === 'success' && (
            <div className="flex flex-col items-center gap-3">
              <CheckCircle size={32} className="text-emerald-400" />
              <p className="text-sm text-emerald-400">{message}</p>
            </div>
          )}
          {status === 'error' && (
            <div className="flex flex-col items-center gap-3">
              <XCircle size={32} className="text-red-400" />
              <p className="text-sm text-red-400">{message}</p>
              <a href="/" className="text-xs text-indigo-400 hover:underline">Go to dashboard</a>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
