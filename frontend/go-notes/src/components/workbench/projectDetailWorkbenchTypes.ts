import type {
  WorkbenchManagedService,
  WorkbenchMutationIssue,
  WorkbenchOptionalServiceComposeMatch,
  WorkbenchOptionalServiceMutationAction,
  WorkbenchPortSelector,
  WorkbenchResourceField,
  WorkbenchStackModule,
} from '@/types/workbench'

export type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

export interface WorkbenchServiceInventoryRow {
  serviceName: string
  image: string | null
  buildSource: string | null
  restartPolicy: string | null
  dependencies: string[]
  portCount: number
  networkCount: number
  managedEntryKeys: string[]
  legacyModuleTypes: string[]
  originLabel: string
  originTone: BadgeTone
}

export interface WorkbenchPortInventoryRow {
  key: string
  selector: WorkbenchPortSelector
  serviceName: string
  containerPort: number
  protocol: string
  hostIp: string
  assignmentStrategy: string
  assignmentStrategyLabel: string
  assignmentStrategyTone: BadgeTone
  allocationStatus: string
  allocationStatusLabel: string
  allocationStatusTone: BadgeTone
  requestedHostPort: string | null
  effectiveHostPort: string | null
  effectiveHostPortLabel: string
  mappingLabel: string
  guidance: string
}

export interface WorkbenchResourceInventoryRow {
  key: string
  serviceName: string
  tracked: boolean
  limitCpus: string | null
  limitMemory: string | null
  reservationCpus: string | null
  reservationMemory: string | null
  hasLimits: boolean
  hasReservations: boolean
}

export interface WorkbenchResourceInputState {
  limitCpus: string
  limitMemory: string
  reservationCpus: string
  reservationMemory: string
}

export interface WorkbenchResourceEditorField {
  key: WorkbenchResourceField
  label: string
  placeholder: string
  section: 'limits' | 'reservations'
}

export interface WorkbenchOptionalServiceCatalogRow {
  key: string
  displayName: string
  description: string
  category: string
  defaultServiceName: string
  suggestedImage: string | null
  defaultContainerPortLabel: string
  availabilityLabel: string
  availabilityTone: BadgeTone
  composeServices: WorkbenchOptionalServiceComposeMatch[]
  managedServices: WorkbenchManagedService[]
  legacyModules: WorkbenchStackModule[]
  currentStateLabel: string
  currentStateTone: BadgeTone
  targetStateLabel: string
  mutationReady: boolean
  composeGenerationReady: boolean
  legacyModuleType: string | null
  legacyMutationPath: string | null
  notes: string[]
}

export interface WorkbenchComposeContextSummary {
  importedServices: number
  catalogManagedServices: number
}

export interface WorkbenchTopologyInventoryRow {
  key: string
  serviceName: string
  dependsOn: string[]
  dependedBy: string[]
  networkNames: string[]
  moduleTypes: string[]
}

export interface WorkbenchInlineFeedbackState {
  tone: BadgeTone
  message: string
}

export interface WorkbenchPendingOptionalServiceMutation {
  entryKey: string
  action: WorkbenchOptionalServiceMutationAction
  serviceName: string
  displayName: string
}

export interface WorkbenchComposeIssueInventoryRow {
  key: string
  source: 'preview' | 'apply'
  sourceLabel: string
  issue: WorkbenchMutationIssue
}
