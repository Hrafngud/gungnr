export type HealthStatus = 'ok' | 'warning' | 'error' | 'missing'

export interface DockerHealth {
  status: HealthStatus
  detail?: string
  containers?: number
}

export interface TunnelHealth {
  status: HealthStatus
  detail?: string
  tunnel?: string
  connections?: number
  configPath?: string
}
