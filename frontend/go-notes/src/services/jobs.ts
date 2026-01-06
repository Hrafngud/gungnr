import { api } from '@/services/api'
import type { Job, JobDetail, JobListResponse } from '@/types/jobs'

export const jobsApi = {
  list: (params?: { page?: number; limit?: number }) =>
    api.get<JobListResponse>('/api/v1/jobs', { params }),
  get: (id: number) => api.get<JobDetail>(`/api/v1/jobs/${id}`),
  stop: (id: number, payload?: { error?: string }) =>
    api.post<{ job: Job }>(`/api/v1/jobs/${id}/stop`, payload),
  retry: (id: number) => api.post<{ job: Job }>(`/api/v1/jobs/${id}/retry`),
}
