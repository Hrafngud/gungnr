import { api } from '@/services/api'
import type { HostDeployRequest, HostDeployResponse } from '@/types/jobs'

export const hostJobsApi = {
  createHostDeploy: (payload: HostDeployRequest) =>
    api.post<HostDeployResponse>('/api/v1/jobs/host-deploy', payload),
}
