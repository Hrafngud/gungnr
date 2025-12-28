import { api } from '@/services/api'
import type { DockerHealth, TunnelHealth } from '@/types/health'

export const healthApi = {
  docker: () => api.get<DockerHealth>('/health/docker'),
  tunnel: () => api.get<TunnelHealth>('/health/tunnel'),
}
