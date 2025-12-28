import { api } from '@/services/api'
import type { Job } from '@/types/jobs'

export const jobsApi = {
  list: () => api.get<{ jobs: Job[] }>('/api/v1/jobs'),
}
