const BASE_URL = '/api/v1'

class HttpError extends Error {
  code: string
  hint?: string
  status: number
  constructor(status: number, error: { code: string; message: string; hint?: string }) {
    super(error.message)
    this.code = error.code
    this.hint = error.hint
    this.status = status
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const apiKey = localStorage.getItem('cc_api_key')
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { 'X-API-Key': apiKey } : {}),
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: { code: 'UNKNOWN', message: res.statusText } }))
    throw new HttpError(res.status, body.error)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Workspace
  getWorkspace: () => request<{ data: any }>('/workspace'),

  // Processes
  listProcesses: (params?: string) => request<{ data: any[]; meta: any }>(`/processes${params ? `?${params}` : ''}`),
  getProcess: (id: string) => request<{ data: any }>(`/processes/${id}`),
  createProcess: (data: any) => request<{ data: any }>('/processes', { method: 'POST', body: JSON.stringify(data) }),
  updateProcess: (id: string, data: any) => request<{ data: any }>(`/processes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProcess: (id: string) => request(`/processes/${id}`, { method: 'DELETE' }),
  triggerProcess: (id: string) => request<{ data: any }>(`/processes/${id}/trigger`, { method: 'POST' }),
  pauseProcess: (id: string, cancelPending = false) =>
    request(`/processes/${id}/pause?cancel_pending=${cancelPending}`, { method: 'POST' }),
  resumeProcess: (id: string) => request(`/processes/${id}/resume`, { method: 'POST' }),

  // Runs
  listRuns: (params?: string) => request<{ data: any[]; meta: any }>(`/runs${params ? `?${params}` : ''}`),
  getRun: (id: string) => request<{ data: any }>(`/runs/${id}`),
  cancelRun: (id: string) => request(`/runs/${id}/cancel`, { method: 'POST' }),
  killRun: (id: string) => request(`/runs/${id}/kill`, { method: 'POST' }),
  replayRun: (id: string) => request<{ data: any }>(`/runs/${id}/replay`, { method: 'POST' }),
  getRunOutput: (id: string, stream?: string) =>
    request<{ data: any[] }>(`/runs/${id}/output${stream ? `?stream=${stream}` : ''}`),

  // Queues
  listQueues: () => request<{ data: any[]; meta: any }>('/queues'),
  getQueue: (id: string) => request<{ data: any }>(`/queues/${id}`),
  createQueue: (data: any) => request<{ data: any }>('/queues', { method: 'POST', body: JSON.stringify(data) }),

  // Jobs
  listJobs: (params?: string) => request<{ data: any[]; meta: any }>(`/jobs${params ? `?${params}` : ''}`),
  getJob: (id: string) => request<{ data: any }>(`/jobs/${id}`),
  enqueueJob: (data: any) => request<{ data: any }>('/jobs', { method: 'POST', body: JSON.stringify(data) }),
  cancelJob: (id: string) => request(`/jobs/${id}/cancel`, { method: 'POST' }),
  replayJob: (id: string, overrides?: any) =>
    request<{ data: any }>(`/jobs/${id}/replay`, { method: 'POST', body: overrides ? JSON.stringify(overrides) : undefined }),

  // Workers
  listWorkers: () => request<{ data: any[]; meta: any }>('/workers'),
  createWorker: (data: any) => request<{ data: any }>('/workers', { method: 'POST', body: JSON.stringify(data) }),
  deleteWorker: (id: string) => request(`/workers/${id}`, { method: 'DELETE' }),

  // API Keys
  listApiKeys: () => request<{ data: any[]; meta: any }>('/api-keys'),
  createApiKey: (data: any) => request<{ data: any }>('/api-keys', { method: 'POST', body: JSON.stringify(data) }),
  deleteApiKey: (id: string) => request(`/api-keys/${id}`, { method: 'DELETE' }),

  // Members
  listMembers: () => request<{ data: any[]; meta: any }>('/users'),

  // Workspaces (multi-workspace)
  listWorkspaces: () => request<{ data: any[]; meta: any }>('/workspaces'),
  switchWorkspace: (id: string) => request<{ data: any }>(`/workspaces/${id}/switch`, { method: 'POST' }),

  // Secrets
  listSecrets: () => request<{ data: any[] }>('/secrets'),
  createSecret: (name: string, value: string) => request<{ data: any }>('/secrets', { method: 'POST', body: JSON.stringify({ name, value }) }),
  updateSecret: (name: string, value: string) => request<{ data: any }>(`/secrets/${name}`, { method: 'PUT', body: JSON.stringify({ value }) }),
  deleteSecret: (name: string) => request(`/secrets/${name}`, { method: 'DELETE' }),

  // Heartbeat
  sendHeartbeat: (data: any) => request('/heartbeat', { method: 'POST', body: JSON.stringify(data) }),
}

export { HttpError }
