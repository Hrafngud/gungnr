export type WorkbenchImportReason = 'manual' | 'auto_deploy' | 'auto_redeploy'

export interface WorkbenchStackService {
  serviceName: string
  image?: string
  buildSource?: string
  restartPolicy?: string
}

export interface WorkbenchStackDependency {
  serviceName: string
  dependsOn: string
}

export interface WorkbenchStackPort {
  serviceName: string
  containerPort: number
  hostPort?: number
  hostPortRaw?: string
  protocol: string
  hostIp?: string
  assignmentStrategy?: string
  allocationStatus?: string
}

export interface WorkbenchStackResource {
  serviceName: string
  limitCpus?: string
  limitMemory?: string
  reservationCpus?: string
  reservationMemory?: string
}

export interface WorkbenchStackNetworkRef {
  serviceName: string
  networkName: string
}

export interface WorkbenchStackVolumeRef {
  serviceName: string
  volumeName: string
}

export interface WorkbenchStackEnvRef {
  serviceName?: string
  path: string
  expression: string
  variable: string
}

export interface WorkbenchStackModule {
  moduleType: string
  serviceName?: string
}

export interface WorkbenchStackWarning {
  code: string
  path: string
  message: string
}

export interface WorkbenchStackSnapshot {
  projectName: string
  projectDir: string
  composePath: string
  modelVersion: number
  revision: number
  sourceFingerprint: string
  services: WorkbenchStackService[]
  dependencies: WorkbenchStackDependency[]
  ports: WorkbenchStackPort[]
  resources: WorkbenchStackResource[]
  networkRefs: WorkbenchStackNetworkRef[]
  volumeRefs: WorkbenchStackVolumeRef[]
  envRefs: WorkbenchStackEnvRef[]
  modules: WorkbenchStackModule[]
  warnings: WorkbenchStackWarning[]
}

export interface WorkbenchSnapshotResponse {
  stack: WorkbenchStackSnapshot
}

export interface WorkbenchImportRequest {
  reason?: WorkbenchImportReason
}

export interface WorkbenchImportResponse {
  stack: WorkbenchStackSnapshot
  changed: boolean
  idempotent: boolean
}
