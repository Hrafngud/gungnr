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

export interface WorkbenchPortSelector {
  serviceName: string
  containerPort: number
  protocol?: string
  hostIp?: string
}

export interface WorkbenchMutationIssue {
  class: string
  code: string
  path: string
  message: string
  service?: string
  field?: string
  moduleType?: string
  action?: string
  protocol?: string
  hostIp?: string
  hostPort?: string
  strategy?: string
  source?: string
}

export type WorkbenchPortResolutionIssue = WorkbenchMutationIssue

export interface WorkbenchPortResolveOutcome {
  serviceName: string
  containerPort: number
  protocol: string
  hostIp?: string
  requestedHostPort?: number
  requestedHostPortRaw?: string
  preferredHostPort?: number
  assignedHostPort?: number
  status: string
  strategy: string
  source: string
  attempts?: number
  message?: string
}

export interface WorkbenchPortResolutionSummary {
  changed: boolean
  assigned: number
  conflict: number
  unavailable: number
  outcomes: WorkbenchPortResolveOutcome[]
}

export interface WorkbenchPortResolveResponse {
  stack: WorkbenchStackSnapshot
  resolve: WorkbenchPortResolutionSummary
}

export type WorkbenchPortMutationAction = 'set_manual' | 'clear_manual'

export interface WorkbenchPortMutationRequest {
  selector: WorkbenchPortSelector
  action: WorkbenchPortMutationAction
  manualHostPort?: number
}

export interface WorkbenchPortMutationSummary {
  changed: boolean
  action: WorkbenchPortMutationAction
  selector: WorkbenchPortSelector
  source?: string
  status?: string
  message?: string
  previousStrategy?: string
  currentStrategy?: string
  previousHostPort?: number
  requestedHostPort?: number
  preferredHostPort?: number
  assignedHostPort?: number
  attempts?: number
}

export interface WorkbenchPortMutationResponse {
  stack: WorkbenchStackSnapshot
  mutation: WorkbenchPortMutationSummary
}

export interface WorkbenchPortSuggestionRequest {
  selector: WorkbenchPortSelector
  limit?: number
}

export interface WorkbenchPortSuggestion {
  hostPort: number
  rank: number
}

export interface WorkbenchPortSuggestionSummary {
  selector: WorkbenchPortSelector
  source?: string
  preferredHostPort?: number
  currentHostPort?: number
  currentStrategy?: string
  currentStatus?: string
  limit: number
  suggestionCount: number
  suggestions: WorkbenchPortSuggestion[]
}

export interface WorkbenchPortSuggestionResponse {
  stack: WorkbenchStackSnapshot
  suggestions: WorkbenchPortSuggestionSummary
}

export type WorkbenchResourceMutationAction = 'set' | 'clear'

export type WorkbenchResourceField =
  | 'limitCpus'
  | 'limitMemory'
  | 'reservationCpus'
  | 'reservationMemory'

export interface WorkbenchResourceMutationRequest {
  action: WorkbenchResourceMutationAction
  limitCpus?: string
  limitMemory?: string
  reservationCpus?: string
  reservationMemory?: string
  clearFields?: WorkbenchResourceField[]
}

export interface WorkbenchResourceMutationSummary {
  changed: boolean
  action: WorkbenchResourceMutationAction
  selector: {
    serviceName: string
  }
  updatedFields: WorkbenchResourceField[]
  clearedFields: WorkbenchResourceField[]
  previousResource?: WorkbenchStackResource
  currentResource?: WorkbenchStackResource
}

export interface WorkbenchResourceMutationResponse {
  stack: WorkbenchStackSnapshot
  mutation: WorkbenchResourceMutationSummary
}

export type WorkbenchModuleMutationAction = 'add' | 'remove'

export interface WorkbenchModuleSelector {
  serviceName: string
  moduleType: string
}

export interface WorkbenchModuleMutationRequest {
  selector: WorkbenchModuleSelector
  action: WorkbenchModuleMutationAction
}

export interface WorkbenchModuleMutationSummary {
  changed: boolean
  action: WorkbenchModuleMutationAction
  selector: WorkbenchModuleSelector
  previousCount: number
  currentCount: number
}

export interface WorkbenchModuleMutationResponse {
  stack: WorkbenchStackSnapshot
  mutation: WorkbenchModuleMutationSummary
}

export function buildWorkbenchPortSelectorKey(selector: WorkbenchPortSelector): string {
  const protocol = selector.protocol?.trim().toLowerCase() || 'tcp'
  const hostIp = selector.hostIp?.trim() || '0.0.0.0'
  return `${selector.serviceName.trim()}::${selector.containerPort}::${protocol}::${hostIp}`
}

export function buildWorkbenchModuleSelectorKey(selector: WorkbenchModuleSelector): string {
  return `${selector.serviceName.trim()}::${selector.moduleType.trim().toLowerCase()}`
}
