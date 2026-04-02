import { api, getApiBaseUrl } from '@/services/api'
import type { DockerContainer, DockerReadDiagnostic, DockerUsageSummary, HostRuntimeSnapshot } from '@/types/host'
import type { Job } from '@/types/jobs'

const restartProjectTimeoutMs = 10 * 60 * 1000

export const hostApi = {
  listDocker: () =>
    api.get<{ containers: DockerContainer[]; diagnostics?: DockerReadDiagnostic[] }>('/api/v1/host/docker'),
  dockerUsage: (project?: string) =>
    api.get<{ summary: DockerUsageSummary; diagnostics?: DockerReadDiagnostic[] }>('/api/v1/host/docker/usage', {
      params: project ? { project } : undefined,
    }),
  runtimeSnapshot: () =>
    api.get<{ snapshot: HostRuntimeSnapshot }>('/api/v1/host/stats'),
  runtimeStatsStreamUrl: () => `${getApiBaseUrl().replace(/\/$/, '')}/api/v1/host/stats/stream`,
  stopContainer: (container: string) =>
    api.post('/api/v1/host/docker/stop', { container }),
  restartContainer: (container: string) =>
    api.post('/api/v1/host/docker/restart', { container }),
  restartProject: (project: string) =>
    api.post<{ job: Job }>(
      '/api/v1/host/docker/project/restart',
      { project },
      { timeout: restartProjectTimeoutMs },
    ),
  removeContainer: (container: string, removeVolumes = false) =>
    api.post('/api/v1/host/docker/remove', { container, removeVolumes }),
}
