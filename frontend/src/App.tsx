import { useState, useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { AppLayout } from '@/components/layout/app-layout'
import { Login } from '@/pages/Login'
import { VerifyEmail } from '@/pages/VerifyEmail'
import { ForgotPassword } from '@/pages/ForgotPassword'
import { Dashboard } from '@/pages/Dashboard'
import { ProcessList } from '@/pages/ProcessList'
import { ProcessCreate } from '@/pages/ProcessCreate'
import { RunList } from '@/pages/RunList'
import { RunDetail } from '@/pages/RunDetail'
import { RunsUpcoming } from '@/pages/RunsUpcoming'
import { QueueOverview } from '@/pages/QueueOverview'
import { QueueDetail } from '@/pages/QueueDetail'
import { QueueCreate } from '@/pages/QueueCreate'
import { JobList } from '@/pages/JobList'
import { JobDetail } from '@/pages/JobDetail'
import { FailedJobs } from '@/pages/FailedJobs'
import { Settings } from '@/pages/Settings'
import { Timeline } from '@/pages/Timeline'
import { ProcessDetail } from '@/pages/ProcessDetail'
import { OrchestraList } from '@/pages/OrchestraList'
import { OrchestraDetail } from '@/pages/OrchestraDetail'
import { Admin } from '@/pages/Admin'
import { NotFound } from '@/pages/NotFound'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      retry: 1,
    },
  },
})

function Router() {
  const [path, setPath] = useState(window.location.pathname)

  useEffect(() => {
    const handler = () => setPath(window.location.pathname)
    window.addEventListener('popstate', handler)
    const originalPushState = history.pushState
    history.pushState = function (...args) {
      originalPushState.apply(this, args)
      handler()
    }
    return () => {
      window.removeEventListener('popstate', handler)
      history.pushState = originalPushState
    }
  }, [])

  const runIdMatch = path.match(/^\/runs\/([^/]+)$/)
  const jobIdMatch = path.match(/^\/jobs\/([^/]+)$/)
  const procIdMatch = path.match(/^\/processes\/([^/]+)$/)
  const queueIdMatch = path.match(/^\/queues\/([^/]+)$/)
  const orchIdMatch = path.match(/^\/orchestras\/([^/]+)$/)

  switch (true) {
    case path === '/':
      return <Dashboard />
    case path === '/processes/new':
      return <ProcessCreate />
    case path === '/processes' && procIdMatch === null:
      return <ProcessList />
    case procIdMatch !== null && procIdMatch[1] !== 'new':
      return <ProcessDetail processId={procIdMatch![1]} />
    case path === '/runs/timeline':
      return <Timeline />
    case path === '/runs/upcoming':
      return <RunsUpcoming />
    case path === '/runs':
      return <RunList />
    case runIdMatch !== null && runIdMatch[1] !== 'timeline' && runIdMatch[1] !== 'upcoming':
      return <RunDetail runId={runIdMatch![1]} />
    case path === '/queues/new':
      return <QueueCreate />
    case path === '/queues':
      return <QueueOverview />
    case queueIdMatch !== null && queueIdMatch[1] !== 'new':
      return <QueueDetail />
    case path === '/jobs/failed':
      return <FailedJobs />
    case path === '/jobs':
      return <JobList />
    case jobIdMatch !== null && jobIdMatch[1] !== 'failed':
      return <JobDetail jobId={jobIdMatch![1]} />
    case path === '/orchestras':
      return <OrchestraList />
    case orchIdMatch !== null:
      return <OrchestraDetail orchestraId={orchIdMatch![1]} />
    case path.startsWith('/settings'):
      return <Settings />
    case path.startsWith('/admin'):
      return <Admin />
    default:
      return <NotFound />
  }
}

function App() {
  const apiKey = localStorage.getItem('cc_api_key')
  const currentPath = window.location.pathname

  // Check for API key in cookie (from Google OAuth callback)
  const oauthCookie = document.cookie.split('; ').find(c => c.startsWith('cc_oauth_key='))
  if (oauthCookie) {
    const key = oauthCookie.split('=')[1]
    localStorage.setItem('cc_api_key', key)
    // Clear the cookie
    document.cookie = 'cc_oauth_key=; path=/; max-age=0'
    window.history.replaceState(null, '', '/')
    window.location.reload()
    return null
  }

  // Public pages (no auth required)
  if (currentPath === '/verify-email') {
    return <QueryClientProvider client={queryClient}><VerifyEmail /></QueryClientProvider>
  }
  if (currentPath === '/forgot-password' || currentPath === '/reset-password') {
    return <QueryClientProvider client={queryClient}><ForgotPassword /></QueryClientProvider>
  }

  if (!apiKey) {
    return (
      <QueryClientProvider client={queryClient}>
        <Login />
      </QueryClientProvider>
    )
  }

  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={300}>
        <Toaster theme="dark" position="bottom-right" richColors />
        <AppLayout>
          <Router />
        </AppLayout>
      </TooltipProvider>
    </QueryClientProvider>
  )
}

export default App
