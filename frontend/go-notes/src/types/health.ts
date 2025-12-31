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
  diagnostics?: TunnelDiagnostics
}

import type { SettingsSources } from '@/types/settings'

export interface TunnelDiagnostics {
  accountId?: string
  zoneId?: string
  tunnel?: string
  domain?: string
  configPath?: string
  tokenSet?: boolean
  tunnelRefType?: string
  sources?: SettingsSources
}
