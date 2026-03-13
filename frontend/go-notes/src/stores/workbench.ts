import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ApiError, parseApiError } from '@/services/api'
import { queryClient } from '@/services/queryClient'
import { workbenchApi } from '@/services/workbench'
import {
  invalidateWorkbenchReadQueries,
  refetchWorkbenchReadQueries,
} from '@/services/workbenchQueries'
import {
  buildWorkbenchModuleSelectorKey,
  buildWorkbenchPortSelectorKey,
  type WorkbenchComposeApplyRequest,
  type WorkbenchComposeApplyResult as WorkbenchComposeApplyApiResult,
  type WorkbenchComposeBackupMetadata,
  type WorkbenchOptionalServiceCatalog,
  type WorkbenchOptionalServiceMutationSummary,
  type WorkbenchComposePreviewRequest,
  type WorkbenchComposePreviewResult as WorkbenchComposePreviewApiResult,
  type WorkbenchComposeRestoreRequest,
  type WorkbenchComposeRestoreResult as WorkbenchComposeRestoreApiResult,
  type WorkbenchImportReason,
  type WorkbenchModuleMutationRequest,
  type WorkbenchModuleMutationSummary,
  type WorkbenchModuleSelector,
  type WorkbenchPortMutationRequest,
  type WorkbenchPortMutationSummary,
  type WorkbenchPortResolveOutcome,
  type WorkbenchPortResolutionSummary,
  type WorkbenchPortSelector,
  type WorkbenchPortSuggestion,
  type WorkbenchPortSuggestionSummary,
  type WorkbenchResourceMutationRequest,
  type WorkbenchResourceMutationSummary,
  type WorkbenchStackResource,
  type WorkbenchStackSnapshot,
} from '@/types/workbench'

export type WorkbenchRequestStatus = 'idle' | 'loading' | 'ready' | 'error'

export interface WorkbenchImportResult {
  projectName: string
  reason: WorkbenchImportReason
  changed: boolean
  idempotent: boolean
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchOptionalServiceMutationResult {
  projectName: string
  changed: boolean
  action: WorkbenchOptionalServiceMutationSummary['action']
  entryKey?: string
  serviceName?: string
  previousCount: number
  currentCount: number
  composeGenerationReady: boolean
  notes: string[]
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchPortResolveResult {
  projectName: string
  changed: boolean
  assigned: number
  conflict: number
  unavailable: number
  outcomes: WorkbenchPortResolveOutcome[]
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchPortMutationResult {
  projectName: string
  changed: boolean
  action: WorkbenchPortMutationSummary['action']
  selector: WorkbenchPortSelector
  status?: string
  message?: string
  source?: string
  previousStrategy?: string
  currentStrategy?: string
  previousHostPort?: number
  requestedHostPort?: number
  preferredHostPort?: number
  assignedHostPort?: number
  attempts?: number
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchPortSuggestionResult {
  projectName: string
  selector: WorkbenchPortSelector
  selectorKey: string
  source?: string
  preferredHostPort?: number
  currentHostPort?: number
  currentStrategy?: string
  currentStatus?: string
  limit: number
  suggestionCount: number
  suggestions: WorkbenchPortSuggestion[]
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchResourceMutationResult {
  projectName: string
  serviceName: string
  changed: boolean
  action: WorkbenchResourceMutationSummary['action']
  updatedFields: WorkbenchResourceMutationSummary['updatedFields']
  clearedFields: WorkbenchResourceMutationSummary['clearedFields']
  previousResource?: WorkbenchStackResource
  currentResource?: WorkbenchStackResource
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchModuleMutationResult {
  projectName: string
  changed: boolean
  action: WorkbenchModuleMutationSummary['action']
  selector: WorkbenchModuleSelector
  previousCount: number
  currentCount: number
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchComposePreviewResultState {
  projectName: string
  compose: string
  revision: number
  sourceFingerprint: string
}

export interface WorkbenchComposeApplyResultState {
  projectName: string
  revision: number
  sourceFingerprint: string
  composePath: string
  composeBytes: number
  backupId: string
  backupSequence: number
  backupRevision: number
  retainedBackups: number
  prunedBackups: number
}

export interface WorkbenchComposeRestoreResultState {
  projectName: string
  revision: number
  sourceFingerprint: string
  restoredFingerprint: string
  composePath: string
  composeBytes: number
  backupId: string
  backupSequence: number
  backupRevision: number
  backupCreatedAt: string
  requiresImport: boolean
}

function normalizeProjectName(projectName: string): string {
  return projectName.trim()
}

function createProjectNameError(): ApiError {
  return new ApiError('Project name is required.')
}

function createServiceNameError(): ApiError {
  return new ApiError('Service name is required.')
}

function createSnapshotRequiredError(message: string): ApiError {
  return new ApiError(message)
}

function createBackupRequiredError(): ApiError {
  return new ApiError('Choose a compose backup before restoring.')
}

async function refreshWorkbenchReadState(
  projectName: string,
  selection: { snapshot?: boolean; graph?: boolean; catalog?: boolean; backups?: boolean } = {},
) {
  await invalidateWorkbenchReadQueries(queryClient, projectName, selection)
  await refetchWorkbenchReadQueries(queryClient, projectName, selection)
}

function snapshotIdentity(value: WorkbenchStackSnapshot | null): string {
  if (!value) return ''
  return `${value.projectName.trim()}::${value.revision}::${value.sourceFingerprint.trim()}`
}

function applySnapshotState(
  nextSnapshot: WorkbenchStackSnapshot,
  snapshotRef: { value: WorkbenchStackSnapshot | null },
  snapshotStatusRef: { value: WorkbenchRequestStatus },
  snapshotErrorRef: { value: ApiError | null },
) {
  snapshotRef.value = nextSnapshot
  snapshotStatusRef.value = 'ready'
  snapshotErrorRef.value = null
}

function createPortResolveResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchPortResolutionSummary,
): WorkbenchPortResolveResult {
  return {
    projectName: snapshotValue.projectName,
    changed: summary.changed,
    assigned: summary.assigned,
    conflict: summary.conflict,
    unavailable: summary.unavailable,
    outcomes: summary.outcomes,
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createOptionalServiceMutationResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchOptionalServiceMutationSummary,
): WorkbenchOptionalServiceMutationResult {
  return {
    projectName: snapshotValue.projectName,
    changed: summary.changed,
    action: summary.action,
    entryKey: summary.entryKey,
    serviceName: summary.serviceName,
    previousCount: summary.previousCount,
    currentCount: summary.currentCount,
    composeGenerationReady: summary.composeGenerationReady,
    notes: Array.isArray(summary.notes) ? summary.notes : [],
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createPortMutationResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchPortMutationSummary,
): WorkbenchPortMutationResult {
  return {
    projectName: snapshotValue.projectName,
    changed: summary.changed,
    action: summary.action,
    selector: summary.selector,
    status: summary.status,
    message: summary.message,
    source: summary.source,
    previousStrategy: summary.previousStrategy,
    currentStrategy: summary.currentStrategy,
    previousHostPort: summary.previousHostPort,
    requestedHostPort: summary.requestedHostPort,
    preferredHostPort: summary.preferredHostPort,
    assignedHostPort: summary.assignedHostPort,
    attempts: summary.attempts,
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createPortSuggestionResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchPortSuggestionSummary,
): WorkbenchPortSuggestionResult {
  return {
    projectName: snapshotValue.projectName,
    selector: summary.selector,
    selectorKey: buildWorkbenchPortSelectorKey(summary.selector),
    source: summary.source,
    preferredHostPort: summary.preferredHostPort,
    currentHostPort: summary.currentHostPort,
    currentStrategy: summary.currentStrategy,
    currentStatus: summary.currentStatus,
    limit: summary.limit,
    suggestionCount: summary.suggestionCount,
    suggestions: summary.suggestions,
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createResourceMutationResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchResourceMutationSummary,
): WorkbenchResourceMutationResult {
  return {
    projectName: snapshotValue.projectName,
    serviceName: summary.selector.serviceName,
    changed: summary.changed,
    action: summary.action,
    updatedFields: summary.updatedFields,
    clearedFields: summary.clearedFields,
    previousResource: summary.previousResource,
    currentResource: summary.currentResource,
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createModuleMutationResult(
  snapshotValue: WorkbenchStackSnapshot,
  summary: WorkbenchModuleMutationSummary,
): WorkbenchModuleMutationResult {
  return {
    projectName: snapshotValue.projectName,
    changed: summary.changed,
    action: summary.action,
    selector: summary.selector,
    previousCount: summary.previousCount,
    currentCount: summary.currentCount,
    revision: snapshotValue.revision,
    sourceFingerprint: snapshotValue.sourceFingerprint,
  }
}

function createComposePreviewResult(
  projectName: string,
  result: WorkbenchComposePreviewApiResult,
): WorkbenchComposePreviewResultState {
  return {
    projectName,
    compose: result.compose,
    revision: result.metadata.revision,
    sourceFingerprint: result.metadata.sourceFingerprint,
  }
}

function createComposeApplyResult(
  projectName: string,
  result: WorkbenchComposeApplyApiResult,
): WorkbenchComposeApplyResultState {
  return {
    projectName,
    revision: result.metadata.revision,
    sourceFingerprint: result.metadata.sourceFingerprint,
    composePath: result.metadata.composePath,
    composeBytes: result.composeBytes,
    backupId: result.backup.backupId,
    backupSequence: result.backup.sequence,
    backupRevision: result.backup.revision,
    retainedBackups: result.retention.retainedCount,
    prunedBackups: result.retention.prunedCount,
  }
}

function createComposeRestoreResult(
  projectName: string,
  result: WorkbenchComposeRestoreApiResult,
): WorkbenchComposeRestoreResultState {
  return {
    projectName,
    revision: result.metadata.revision,
    sourceFingerprint: result.metadata.sourceFingerprint?.trim() || '',
    restoredFingerprint: result.metadata.restoredFingerprint?.trim() || '',
    composePath: result.metadata.composePath,
    composeBytes: result.composeBytes,
    backupId: result.backup.backupId,
    backupSequence: result.backup.sequence,
    backupRevision: result.backup.revision,
    backupCreatedAt: result.backup.createdAt,
    requiresImport: result.metadata.requiresImport,
  }
}

export const useWorkbenchStore = defineStore('workbench', () => {
  const projectName = ref<string | null>(null)
  const snapshot = ref<WorkbenchStackSnapshot | null>(null)
  const snapshotStatus = ref<WorkbenchRequestStatus>('idle')
  const snapshotError = ref<ApiError | null>(null)
  const catalog = ref<WorkbenchOptionalServiceCatalog | null>(null)
  const catalogStatus = ref<WorkbenchRequestStatus>('idle')
  const catalogError = ref<ApiError | null>(null)
  const optionalServiceMutationStatus = ref<WorkbenchRequestStatus>('idle')
  const optionalServiceMutationError = ref<ApiError | null>(null)
  const activeOptionalServiceMutationEntryKey = ref<string | null>(null)
  const lastOptionalServiceMutationResult = ref<WorkbenchOptionalServiceMutationResult | null>(null)
  const importStatus = ref<WorkbenchRequestStatus>('idle')
  const importError = ref<ApiError | null>(null)
  const lastImportResult = ref<WorkbenchImportResult | null>(null)
  const resolveStatus = ref<WorkbenchRequestStatus>('idle')
  const resolveError = ref<ApiError | null>(null)
  const lastResolveResult = ref<WorkbenchPortResolveResult | null>(null)
  const portMutationStatus = ref<WorkbenchRequestStatus>('idle')
  const portMutationError = ref<ApiError | null>(null)
  const activePortMutationSelectorKey = ref<string | null>(null)
  const lastPortMutationResult = ref<WorkbenchPortMutationResult | null>(null)
  const resourceMutationStatus = ref<WorkbenchRequestStatus>('idle')
  const resourceMutationError = ref<ApiError | null>(null)
  const activeResourceMutationServiceName = ref<string | null>(null)
  const lastResourceMutationResult = ref<WorkbenchResourceMutationResult | null>(null)
  const moduleMutationStatus = ref<WorkbenchRequestStatus>('idle')
  const moduleMutationError = ref<ApiError | null>(null)
  const activeModuleMutationSelectorKey = ref<string | null>(null)
  const lastModuleMutationResult = ref<WorkbenchModuleMutationResult | null>(null)
  const previewStatus = ref<WorkbenchRequestStatus>('idle')
  const previewError = ref<ApiError | null>(null)
  const lastPreviewResult = ref<WorkbenchComposePreviewResultState | null>(null)
  const applyStatus = ref<WorkbenchRequestStatus>('idle')
  const applyError = ref<ApiError | null>(null)
  const lastApplyResult = ref<WorkbenchComposeApplyResultState | null>(null)
  const composeBackups = ref<WorkbenchComposeBackupMetadata[]>([])
  const backupInventoryStatus = ref<WorkbenchRequestStatus>('idle')
  const backupInventoryError = ref<ApiError | null>(null)
  const restoreStatus = ref<WorkbenchRequestStatus>('idle')
  const restoreError = ref<ApiError | null>(null)
  const lastRestoreResult = ref<WorkbenchComposeRestoreResultState | null>(null)
  const portSuggestionStatusByKey = ref<Record<string, WorkbenchRequestStatus>>({})
  const portSuggestionErrorByKey = ref<Record<string, ApiError | null>>({})
  const portSuggestionResultByKey = ref<Record<string, WorkbenchPortSuggestionResult | null>>({})

  const loading = computed(() => snapshotStatus.value === 'loading')
  const submitting = computed(() => importStatus.value === 'loading')
  const ready = computed(() => snapshotStatus.value === 'ready' && snapshot.value !== null)

  let snapshotRequestID = 0
  let catalogRequestID = 0
  let optionalServiceMutationRequestID = 0
  let importRequestID = 0
  let resolveRequestID = 0
  let portMutationRequestID = 0
  let resourceMutationRequestID = 0
  let moduleMutationRequestID = 0
  let previewRequestID = 0
  let applyRequestID = 0
  let backupInventoryRequestID = 0
  let restoreRequestID = 0
  const portSuggestionRequestIDs = new Map<string, number>()

  const resetComposePreviewState = () => {
    previewStatus.value = 'idle'
    previewError.value = null
    lastPreviewResult.value = null
  }

  const resetComposeApplyState = () => {
    applyStatus.value = 'idle'
    applyError.value = null
    lastApplyResult.value = null
  }

  const resetComposeRestoreState = () => {
    restoreStatus.value = 'idle'
    restoreError.value = null
    lastRestoreResult.value = null
  }

  const clearRestoreRequirementIfResolved = (nextSnapshot: WorkbenchStackSnapshot) => {
    const restoreResult = lastRestoreResult.value
    if (!restoreResult?.requiresImport) return
    if (nextSnapshot.sourceFingerprint.trim() !== restoreResult.restoredFingerprint.trim()) return

    restoreStatus.value = 'idle'
    restoreError.value = null
    lastRestoreResult.value = null
  }

  const resetComposeExecutionState = () => {
    resetComposePreviewState()
    resetComposeApplyState()
  }

  const resetPortSuggestions = () => {
    portSuggestionStatusByKey.value = {}
    portSuggestionErrorByKey.value = {}
    portSuggestionResultByKey.value = {}
    portSuggestionRequestIDs.clear()
  }

  const reset = () => {
    projectName.value = null
    snapshot.value = null
    snapshotStatus.value = 'idle'
    snapshotError.value = null
    catalog.value = null
    catalogStatus.value = 'idle'
    catalogError.value = null
    optionalServiceMutationStatus.value = 'idle'
    optionalServiceMutationError.value = null
    activeOptionalServiceMutationEntryKey.value = null
    lastOptionalServiceMutationResult.value = null
    importStatus.value = 'idle'
    importError.value = null
    lastImportResult.value = null
    resolveStatus.value = 'idle'
    resolveError.value = null
    lastResolveResult.value = null
    portMutationStatus.value = 'idle'
    portMutationError.value = null
    activePortMutationSelectorKey.value = null
    lastPortMutationResult.value = null
    resourceMutationStatus.value = 'idle'
    resourceMutationError.value = null
    activeResourceMutationServiceName.value = null
    lastResourceMutationResult.value = null
    moduleMutationStatus.value = 'idle'
    moduleMutationError.value = null
    activeModuleMutationSelectorKey.value = null
    lastModuleMutationResult.value = null
    composeBackups.value = []
    backupInventoryStatus.value = 'idle'
    backupInventoryError.value = null
    resetComposeRestoreState()
    resetComposeExecutionState()
    resetPortSuggestions()
  }

  const syncProjectContext = (nextProjectName: string) => {
    if (projectName.value === nextProjectName) return

    projectName.value = nextProjectName
    snapshot.value = null
    snapshotStatus.value = 'idle'
    snapshotError.value = null
    catalog.value = null
    catalogStatus.value = 'idle'
    catalogError.value = null
    optionalServiceMutationStatus.value = 'idle'
    optionalServiceMutationError.value = null
    activeOptionalServiceMutationEntryKey.value = null
    lastOptionalServiceMutationResult.value = null
    importStatus.value = 'idle'
    importError.value = null
    lastImportResult.value = null
    resolveStatus.value = 'idle'
    resolveError.value = null
    lastResolveResult.value = null
    portMutationStatus.value = 'idle'
    portMutationError.value = null
    activePortMutationSelectorKey.value = null
    lastPortMutationResult.value = null
    resourceMutationStatus.value = 'idle'
    resourceMutationError.value = null
    activeResourceMutationServiceName.value = null
    lastResourceMutationResult.value = null
    moduleMutationStatus.value = 'idle'
    moduleMutationError.value = null
    activeModuleMutationSelectorKey.value = null
    lastModuleMutationResult.value = null
    composeBackups.value = []
    backupInventoryStatus.value = 'idle'
    backupInventoryError.value = null
    resetComposeRestoreState()
    resetComposeExecutionState()
    resetPortSuggestions()
  }

  async function loadSnapshot(targetProjectName: string): Promise<WorkbenchStackSnapshot | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      snapshotStatus.value = 'error'
      snapshotError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++snapshotRequestID
    projectName.value = normalizedProjectName
    snapshotStatus.value = 'loading'
    snapshotError.value = null

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.getSnapshot(normalizedProjectName)
      if (requestID !== snapshotRequestID || projectName.value !== normalizedProjectName) {
        return snapshot.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      return data.stack
    } catch (error: unknown) {
      if (requestID !== snapshotRequestID || projectName.value !== normalizedProjectName) {
        return snapshot.value
      }

      snapshot.value = null
      snapshotStatus.value = 'error'
      snapshotError.value = parseApiError(error)
      return null
    }
  }

  async function loadCatalog(
    targetProjectName: string,
  ): Promise<WorkbenchOptionalServiceCatalog | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      catalogStatus.value = 'error'
      catalogError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++catalogRequestID
    projectName.value = normalizedProjectName
    catalogStatus.value = 'loading'
    catalogError.value = null

    try {
      const { data } = await workbenchApi.getCatalog(normalizedProjectName)
      if (requestID !== catalogRequestID || projectName.value !== normalizedProjectName) {
        return catalog.value
      }

      catalog.value = data.catalog
      catalogStatus.value = 'ready'
      catalogError.value = null
      return data.catalog
    } catch (error: unknown) {
      if (requestID !== catalogRequestID || projectName.value !== normalizedProjectName) {
        return catalog.value
      }

      catalog.value = null
      catalogStatus.value = 'error'
      catalogError.value = parseApiError(error)
      return null
    }
  }

  async function runImport(
    targetProjectName: string,
    reason: WorkbenchImportReason = 'manual',
  ): Promise<WorkbenchImportResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      importStatus.value = 'error'
      importError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++importRequestID
    const snapshotUpdateRequestID = ++snapshotRequestID
    const shouldDriveSnapshotState = snapshot.value === null || snapshotStatus.value !== 'ready'
    projectName.value = normalizedProjectName
    importStatus.value = 'loading'
    importError.value = null
    if (shouldDriveSnapshotState) {
      snapshotStatus.value = 'loading'
      snapshotError.value = null
    }

    try {
      const { data } = await workbenchApi.importSnapshot(normalizedProjectName, reason)
      if (requestID !== importRequestID) {
        return lastImportResult.value
      }
      if (
        snapshotUpdateRequestID !== snapshotRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        importStatus.value = 'idle'
        importError.value = null
        return lastImportResult.value
      }

      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()

      const result: WorkbenchImportResult = {
        projectName: data.stack.projectName,
        reason,
        changed: data.changed,
        idempotent: data.idempotent,
        revision: data.stack.revision,
        sourceFingerprint: data.stack.sourceFingerprint,
      }

      lastImportResult.value = result
      importStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
        catalog: true,
        backups: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== importRequestID) {
        return lastImportResult.value
      }
      if (
        snapshotUpdateRequestID !== snapshotRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        importStatus.value = 'idle'
        importError.value = null
        return lastImportResult.value
      }

      const parsedError = parseApiError(error)
      importStatus.value = 'error'
      importError.value = parsedError
      if (shouldDriveSnapshotState) {
        snapshotStatus.value = 'error'
        snapshotError.value = parsedError
      }
      return null
    }
  }

  async function addOptionalService(
    targetProjectName: string,
    entryKey: string,
  ): Promise<WorkbenchOptionalServiceMutationResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = createProjectNameError()
      return null
    }

    const normalizedEntryKey = entryKey.trim().toLowerCase()
    if (!normalizedEntryKey) {
      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = new ApiError('Optional-service entry key is required.')
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++optionalServiceMutationRequestID
    projectName.value = normalizedProjectName
    optionalServiceMutationStatus.value = 'loading'
    optionalServiceMutationError.value = null
    activeOptionalServiceMutationEntryKey.value = normalizedEntryKey

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.addOptionalService(normalizedProjectName, {
        entryKey: normalizedEntryKey,
      })
      if (
        requestID !== optionalServiceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastOptionalServiceMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createOptionalServiceMutationResult(data.stack, data.mutation)
      lastOptionalServiceMutationResult.value = result
      optionalServiceMutationStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
        catalog: true,
      })
      return result
    } catch (error: unknown) {
      if (
        requestID !== optionalServiceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastOptionalServiceMutationResult.value
      }

      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = parseApiError(error)
      return null
    } finally {
      if (requestID === optionalServiceMutationRequestID) {
        activeOptionalServiceMutationEntryKey.value = null
      }
    }
  }

  async function removeOptionalService(
    targetProjectName: string,
    entryKey: string,
    serviceName: string,
  ): Promise<WorkbenchOptionalServiceMutationResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = createProjectNameError()
      return null
    }

    const normalizedEntryKey = entryKey.trim().toLowerCase()
    const normalizedServiceName = serviceName.trim()
    if (!normalizedServiceName) {
      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = createServiceNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++optionalServiceMutationRequestID
    projectName.value = normalizedProjectName
    optionalServiceMutationStatus.value = 'loading'
    optionalServiceMutationError.value = null
    activeOptionalServiceMutationEntryKey.value = normalizedEntryKey || normalizedServiceName

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.removeOptionalService(
        normalizedProjectName,
        normalizedServiceName,
      )
      if (
        requestID !== optionalServiceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastOptionalServiceMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createOptionalServiceMutationResult(data.stack, data.mutation)
      lastOptionalServiceMutationResult.value = result
      optionalServiceMutationStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
        catalog: true,
      })
      return result
    } catch (error: unknown) {
      if (
        requestID !== optionalServiceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastOptionalServiceMutationResult.value
      }

      optionalServiceMutationStatus.value = 'error'
      optionalServiceMutationError.value = parseApiError(error)
      return null
    } finally {
      if (requestID === optionalServiceMutationRequestID) {
        activeOptionalServiceMutationEntryKey.value = null
      }
    }
  }

  async function fetchComposeBackups(
    normalizedProjectName: string,
    options: { setLoading: boolean } = { setLoading: true },
  ): Promise<WorkbenchComposeBackupMetadata[]> {
    const requestID = ++backupInventoryRequestID
    projectName.value = normalizedProjectName
    if (options.setLoading) {
      backupInventoryStatus.value = 'loading'
      backupInventoryError.value = null
    }

    try {
      const { data } = await workbenchApi.getComposeBackups(normalizedProjectName)
      if (
        requestID !== backupInventoryRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return composeBackups.value
      }

      composeBackups.value = data.backups
      backupInventoryStatus.value = 'ready'
      backupInventoryError.value = null
      return data.backups
    } catch (error: unknown) {
      if (
        requestID !== backupInventoryRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return composeBackups.value
      }

      composeBackups.value = []
      backupInventoryStatus.value = 'error'
      backupInventoryError.value = parseApiError(error)
      return []
    }
  }

  async function resolvePorts(targetProjectName: string): Promise<WorkbenchPortResolveResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      resolveStatus.value = 'error'
      resolveError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++resolveRequestID
    projectName.value = normalizedProjectName
    resolveStatus.value = 'loading'
    resolveError.value = null

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.resolvePorts(normalizedProjectName)
      if (requestID !== resolveRequestID || projectName.value !== normalizedProjectName) {
        return lastResolveResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createPortResolveResult(data.stack, data.resolve)
      lastResolveResult.value = result
      resolveStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== resolveRequestID || projectName.value !== normalizedProjectName) {
        return lastResolveResult.value
      }

      resolveStatus.value = 'error'
      resolveError.value = parseApiError(error)
      return null
    }
  }

  async function mutatePort(
    targetProjectName: string,
    payload: WorkbenchPortMutationRequest,
  ): Promise<WorkbenchPortMutationResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      portMutationStatus.value = 'error'
      portMutationError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const selectorKey = buildWorkbenchPortSelectorKey(payload.selector)
    const requestID = ++portMutationRequestID
    projectName.value = normalizedProjectName
    portMutationStatus.value = 'loading'
    portMutationError.value = null
    activePortMutationSelectorKey.value = selectorKey

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.mutatePort(normalizedProjectName, payload)
      if (requestID !== portMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastPortMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createPortMutationResult(data.stack, data.mutation)
      lastPortMutationResult.value = result
      portMutationStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== portMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastPortMutationResult.value
      }

      portMutationStatus.value = 'error'
      portMutationError.value = parseApiError(error)
      return null
    } finally {
      if (requestID === portMutationRequestID) {
        activePortMutationSelectorKey.value = null
      }
    }
  }

  async function mutateResource(
    targetProjectName: string,
    serviceName: string,
    payload: WorkbenchResourceMutationRequest,
  ): Promise<WorkbenchResourceMutationResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      resourceMutationStatus.value = 'error'
      resourceMutationError.value = createProjectNameError()
      return null
    }

    const normalizedServiceName = serviceName.trim()
    if (!normalizedServiceName) {
      resourceMutationStatus.value = 'error'
      resourceMutationError.value = createServiceNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const requestID = ++resourceMutationRequestID
    projectName.value = normalizedProjectName
    resourceMutationStatus.value = 'loading'
    resourceMutationError.value = null
    activeResourceMutationServiceName.value = normalizedServiceName

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.mutateResource(
        normalizedProjectName,
        normalizedServiceName,
        payload,
      )
      if (
        requestID !== resourceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastResourceMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createResourceMutationResult(data.stack, data.mutation)
      lastResourceMutationResult.value = result
      resourceMutationStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
      })
      return result
    } catch (error: unknown) {
      if (
        requestID !== resourceMutationRequestID ||
        projectName.value !== normalizedProjectName
      ) {
        return lastResourceMutationResult.value
      }

      resourceMutationStatus.value = 'error'
      resourceMutationError.value = parseApiError(error)
      return null
    } finally {
      if (requestID === resourceMutationRequestID) {
        activeResourceMutationServiceName.value = null
      }
    }
  }

  async function loadPortSuggestions(
    targetProjectName: string,
    selector: WorkbenchPortSelector,
    limit = 5,
  ): Promise<WorkbenchPortSuggestionResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const selectorKey = buildWorkbenchPortSelectorKey(selector)
    const requestID = (portSuggestionRequestIDs.get(selectorKey) ?? 0) + 1
    portSuggestionRequestIDs.set(selectorKey, requestID)
    projectName.value = normalizedProjectName
    portSuggestionStatusByKey.value = {
      ...portSuggestionStatusByKey.value,
      [selectorKey]: 'loading',
    }
    portSuggestionErrorByKey.value = {
      ...portSuggestionErrorByKey.value,
      [selectorKey]: null,
    }

    try {
      const { data } = await workbenchApi.suggestPorts(normalizedProjectName, { selector, limit })
      if (
        portSuggestionRequestIDs.get(selectorKey) !== requestID ||
        projectName.value !== normalizedProjectName
      ) {
        return portSuggestionResultByKey.value[selectorKey] ?? null
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      const result = createPortSuggestionResult(data.stack, data.suggestions)
      portSuggestionResultByKey.value = {
        ...portSuggestionResultByKey.value,
        [selectorKey]: result,
      }
      portSuggestionStatusByKey.value = {
        ...portSuggestionStatusByKey.value,
        [selectorKey]: 'ready',
      }
      return result
    } catch (error: unknown) {
      if (
        portSuggestionRequestIDs.get(selectorKey) !== requestID ||
        projectName.value !== normalizedProjectName
      ) {
        return portSuggestionResultByKey.value[selectorKey] ?? null
      }

      portSuggestionStatusByKey.value = {
        ...portSuggestionStatusByKey.value,
        [selectorKey]: 'error',
      }
      portSuggestionErrorByKey.value = {
        ...portSuggestionErrorByKey.value,
        [selectorKey]: parseApiError(error),
      }
      return null
    }
  }

  async function mutateModule(
    targetProjectName: string,
    payload: WorkbenchModuleMutationRequest,
  ): Promise<WorkbenchModuleMutationResult | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      moduleMutationStatus.value = 'error'
      moduleMutationError.value = createProjectNameError()
      return null
    }

    const normalizedServiceName = payload.selector.serviceName.trim()
    if (!normalizedServiceName) {
      moduleMutationStatus.value = 'error'
      moduleMutationError.value = createServiceNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const selector = {
      serviceName: normalizedServiceName,
      moduleType: payload.selector.moduleType.trim().toLowerCase(),
    }
    const selectorKey = buildWorkbenchModuleSelectorKey(selector)
    const requestID = ++moduleMutationRequestID
    projectName.value = normalizedProjectName
    moduleMutationStatus.value = 'loading'
    moduleMutationError.value = null
    activeModuleMutationSelectorKey.value = selectorKey

    try {
      const previousSnapshotIdentity = snapshotIdentity(snapshot.value)
      const { data } = await workbenchApi.mutateModule(normalizedProjectName, {
        ...payload,
        selector,
      })
      if (requestID !== moduleMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastModuleMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      clearRestoreRequirementIfResolved(data.stack)
      if (previousSnapshotIdentity !== snapshotIdentity(data.stack)) {
        resetComposeExecutionState()
      }
      resetPortSuggestions()
      const result = createModuleMutationResult(data.stack, data.mutation)
      lastModuleMutationResult.value = result
      moduleMutationStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== moduleMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastModuleMutationResult.value
      }

      moduleMutationStatus.value = 'error'
      moduleMutationError.value = parseApiError(error)
      return null
    } finally {
      if (requestID === moduleMutationRequestID) {
        activeModuleMutationSelectorKey.value = null
      }
    }
  }

  async function previewCompose(
    targetProjectName: string,
    payload: WorkbenchComposePreviewRequest = {},
  ): Promise<WorkbenchComposePreviewResultState | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      previewStatus.value = 'error'
      previewError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const currentSnapshot = snapshot.value
    const expectedRevision = payload.expectedRevision ?? currentSnapshot?.revision
    if (typeof expectedRevision !== 'number') {
      previewStatus.value = 'error'
      previewError.value = createSnapshotRequiredError(
        'Import a Workbench snapshot before generating a compose preview.',
      )
      lastPreviewResult.value = null
      return null
    }

    const requestID = ++previewRequestID
    projectName.value = normalizedProjectName
    previewStatus.value = 'loading'
    previewError.value = null

    try {
      const { data } = await workbenchApi.previewCompose(normalizedProjectName, {
        expectedRevision,
      })
      if (requestID !== previewRequestID || projectName.value !== normalizedProjectName) {
        return lastPreviewResult.value
      }

      const result = createComposePreviewResult(normalizedProjectName, data.preview)
      lastPreviewResult.value = result
      previewStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== previewRequestID || projectName.value !== normalizedProjectName) {
        return lastPreviewResult.value
      }

      previewStatus.value = 'error'
      previewError.value = parseApiError(error)
      lastPreviewResult.value = null
      return null
    }
  }

  async function applyCompose(
    targetProjectName: string,
    payload: WorkbenchComposeApplyRequest = {},
  ): Promise<WorkbenchComposeApplyResultState | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      applyStatus.value = 'error'
      applyError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const currentSnapshot = snapshot.value
    const expectedRevision = payload.expectedRevision ?? currentSnapshot?.revision
    const expectedSourceFingerprint =
      payload.expectedSourceFingerprint?.trim() || currentSnapshot?.sourceFingerprint?.trim() || ''
    if (typeof expectedRevision !== 'number' || !expectedSourceFingerprint) {
      applyStatus.value = 'error'
      applyError.value = createSnapshotRequiredError(
        'Import a Workbench snapshot before applying compose changes.',
      )
      return null
    }

    const requestID = ++applyRequestID
    projectName.value = normalizedProjectName
    applyStatus.value = 'loading'
    applyError.value = null

    try {
      const { data } = await workbenchApi.applyCompose(normalizedProjectName, {
        expectedRevision,
        expectedSourceFingerprint,
      })
      if (requestID !== applyRequestID || projectName.value !== normalizedProjectName) {
        return lastApplyResult.value
      }

      if (snapshot.value) {
        applySnapshotState(
          {
            ...snapshot.value,
            revision: data.apply.metadata.revision,
            sourceFingerprint: data.apply.metadata.sourceFingerprint,
            composePath: data.apply.metadata.composePath,
          },
          snapshot,
          snapshotStatus,
          snapshotError,
        )
        clearRestoreRequirementIfResolved(snapshot.value)
      }

      resetComposePreviewState()
      const result = createComposeApplyResult(normalizedProjectName, data.apply)
      lastApplyResult.value = result
      applyStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
        backups: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== applyRequestID || projectName.value !== normalizedProjectName) {
        return lastApplyResult.value
      }

      const parsedError = parseApiError(error)
      applyStatus.value = 'error'
      applyError.value = parsedError
      if (
        parsedError.code === 'WB-409-STALE-REVISION' ||
        parsedError.code === 'WB-409-DRIFT-DETECTED' ||
        parsedError.code === 'WB-422-VALIDATION'
      ) {
        resetComposePreviewState()
      }
      return null
    }
  }

  async function loadComposeBackups(
    targetProjectName: string,
  ): Promise<WorkbenchComposeBackupMetadata[]> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      backupInventoryStatus.value = 'error'
      backupInventoryError.value = createProjectNameError()
      return []
    }

    syncProjectContext(normalizedProjectName)
    return fetchComposeBackups(normalizedProjectName)
  }

  async function restoreCompose(
    targetProjectName: string,
    payload: WorkbenchComposeRestoreRequest,
  ): Promise<WorkbenchComposeRestoreResultState | null> {
    const normalizedProjectName = normalizeProjectName(targetProjectName)
    if (!normalizedProjectName) {
      reset()
      restoreStatus.value = 'error'
      restoreError.value = createProjectNameError()
      return null
    }

    syncProjectContext(normalizedProjectName)

    const backupId = payload.backupId.trim()
    if (!backupId) {
      restoreStatus.value = 'error'
      restoreError.value = createBackupRequiredError()
      return null
    }

    const requestID = ++restoreRequestID
    projectName.value = normalizedProjectName
    restoreStatus.value = 'loading'
    restoreError.value = null

    try {
      const { data } = await workbenchApi.restoreCompose(normalizedProjectName, { backupId })
      if (requestID !== restoreRequestID || projectName.value !== normalizedProjectName) {
        return lastRestoreResult.value
      }

      if (snapshot.value) {
        applySnapshotState(
          {
            ...snapshot.value,
            composePath: data.restore.metadata.composePath,
          },
          snapshot,
          snapshotStatus,
          snapshotError,
        )
      }

      resetComposeExecutionState()
      const result = createComposeRestoreResult(normalizedProjectName, data.restore)
      lastRestoreResult.value = result
      restoreStatus.value = 'ready'
      await refreshWorkbenchReadState(normalizedProjectName, {
        snapshot: true,
        graph: true,
        backups: true,
      })
      return result
    } catch (error: unknown) {
      if (requestID !== restoreRequestID || projectName.value !== normalizedProjectName) {
        return lastRestoreResult.value
      }

      restoreStatus.value = 'error'
      restoreError.value = parseApiError(error)
      return null
    }
  }

  function clearPortSuggestion(selector: WorkbenchPortSelector | string) {
    const selectorKey =
      typeof selector === 'string' ? selector : buildWorkbenchPortSelectorKey(selector)

    portSuggestionStatusByKey.value = {
      ...portSuggestionStatusByKey.value,
      [selectorKey]: 'idle',
    }
    portSuggestionErrorByKey.value = {
      ...portSuggestionErrorByKey.value,
      [selectorKey]: null,
    }
    portSuggestionResultByKey.value = {
      ...portSuggestionResultByKey.value,
      [selectorKey]: null,
    }
    portSuggestionRequestIDs.delete(selectorKey)
  }

  return {
    projectName,
    snapshot,
    snapshotStatus,
    snapshotError,
    catalog,
    catalogStatus,
    catalogError,
    optionalServiceMutationStatus,
    optionalServiceMutationError,
    activeOptionalServiceMutationEntryKey,
    lastOptionalServiceMutationResult,
    importStatus,
    importError,
    lastImportResult,
    resolveStatus,
    resolveError,
    lastResolveResult,
    portMutationStatus,
    portMutationError,
    activePortMutationSelectorKey,
    lastPortMutationResult,
    resourceMutationStatus,
    resourceMutationError,
    activeResourceMutationServiceName,
    lastResourceMutationResult,
    moduleMutationStatus,
    moduleMutationError,
    activeModuleMutationSelectorKey,
    lastModuleMutationResult,
    previewStatus,
    previewError,
    lastPreviewResult,
    applyStatus,
    applyError,
    lastApplyResult,
    composeBackups,
    backupInventoryStatus,
    backupInventoryError,
    restoreStatus,
    restoreError,
    lastRestoreResult,
    portSuggestionStatusByKey,
    portSuggestionErrorByKey,
    portSuggestionResultByKey,
    loading,
    submitting,
    ready,
    reset,
    loadSnapshot,
    loadCatalog,
    addOptionalService,
    removeOptionalService,
    runImport,
    resolvePorts,
    mutatePort,
    mutateResource,
    mutateModule,
    previewCompose,
    applyCompose,
    loadComposeBackups,
    restoreCompose,
    loadPortSuggestions,
    clearPortSuggestion,
  }
})
