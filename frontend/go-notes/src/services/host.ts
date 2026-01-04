import { api } from '@/services/api'
import type { DockerContainer, DockerUsageSummary } from '@/types/host'

export const hostApi = {
  listDocker: () => api.get<{ containers: DockerContainer[] }>('/api/v1/host/docker'),
  dockerUsage: (project?: string) =>
    api.get<{ summary: DockerUsageSummary }>('/api/v1/host/docker/usage', {
      params: project ? { project } : undefined,
    }),
  stopContainer: (container: string) =>
    api.post('/api/v1/host/docker/stop', { container }),
  restartContainer: (container: string) =>
    api.post('/api/v1/host/docker/restart', { container }),
  removeContainer: (container: string, removeVolumes = false) =>
    api.post('/api/v1/host/docker/remove', { container, removeVolumes }),
}
