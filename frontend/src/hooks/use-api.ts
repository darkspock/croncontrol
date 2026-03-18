import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

// Processes
export function useProcesses() {
  return useQuery({
    queryKey: ['processes'],
    queryFn: () => api.listProcesses(),
  })
}

export function useProcess(id: string) {
  return useQuery({
    queryKey: ['process', id],
    queryFn: () => api.getProcess(id),
    enabled: !!id,
  })
}

export function useTriggerProcess() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.triggerProcess(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['runs'] })
      qc.invalidateQueries({ queryKey: ['processes'] })
    },
  })
}

// Runs
export function useRuns(params?: string) {
  return useQuery({
    queryKey: ['runs', params],
    queryFn: () => api.listRuns(params),
  })
}

export function useRun(id: string) {
  return useQuery({
    queryKey: ['run', id],
    queryFn: () => api.getRun(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const state = query.state.data?.data?.state
      return state === 'running' || state === 'pending' ? 5000 : false
    },
  })
}

export function useRunOutput(id: string) {
  return useQuery({
    queryKey: ['run-output', id],
    queryFn: () => api.getRunOutput(id),
    enabled: !!id,
  })
}

// Queues
export function useQueues() {
  return useQuery({
    queryKey: ['queues'],
    queryFn: () => api.listQueues(),
  })
}

// Jobs
export function useJobs(params?: string) {
  return useQuery({
    queryKey: ['jobs', params],
    queryFn: () => api.listJobs(params),
  })
}

// Workspace
export function useWorkspace() {
  return useQuery({
    queryKey: ['workspace'],
    queryFn: () => api.getWorkspace(),
    retry: false,
  })
}
