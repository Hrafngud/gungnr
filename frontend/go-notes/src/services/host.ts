import { api } from '@/services/api'
import type { DockerContainer } from '@/types/host'

export const hostApi = {
  listDocker: () => api.get<{ containers: DockerContainer[] }>('/api/v1/host/docker'),
}
