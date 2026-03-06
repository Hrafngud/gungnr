import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ApiError, parseApiError } from '@/services/api'
import { workbenchApi } from '@/services/workbench'
import {
  buildWorkbenchModuleSelectorKey,
  buildWorkbenchPortSelectorKey,
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

function normalizeProjectName(projectName: string): string {
  return projectName.trim()
}

function createProjectNameError(): ApiError {
  return new ApiError('Project name is required.')
}

function createServiceNameError(): ApiError {
  return new ApiError('Service name is required.')
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

export const useWorkbenchStore = defineStore('workbench', () => {
  const projectName = ref<string | null>(null)
  const snapshot = ref<WorkbenchStackSnapshot | null>(null)
  const snapshotStatus = ref<WorkbenchRequestStatus>('idle')
  const snapshotError = ref<ApiError | null>(null)
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
  const portSuggestionStatusByKey = ref<Record<string, WorkbenchRequestStatus>>({})
  const portSuggestionErrorByKey = ref<Record<string, ApiError | null>>({})
  const portSuggestionResultByKey = ref<Record<string, WorkbenchPortSuggestionResult | null>>({})

  const loading = computed(() => snapshotStatus.value === 'loading')
  const submitting = computed(() => importStatus.value === 'loading')
  const ready = computed(() => snapshotStatus.value === 'ready' && snapshot.value !== null)

  let snapshotRequestID = 0
  let importRequestID = 0
  let resolveRequestID = 0
  let portMutationRequestID = 0
  let resourceMutationRequestID = 0
  let moduleMutationRequestID = 0
  const portSuggestionRequestIDs = new Map<string, number>()

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
    resetPortSuggestions()
  }

  const syncProjectContext = (nextProjectName: string) => {
    if (projectName.value === nextProjectName) return

    projectName.value = nextProjectName
    snapshot.value = null
    snapshotStatus.value = 'idle'
    snapshotError.value = null
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
      const { data } = await workbenchApi.getSnapshot(normalizedProjectName)
      if (requestID !== snapshotRequestID || projectName.value !== normalizedProjectName) {
        return snapshot.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
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

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
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
      const { data } = await workbenchApi.resolvePorts(normalizedProjectName)
      if (requestID !== resolveRequestID || projectName.value !== normalizedProjectName) {
        return lastResolveResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      resetPortSuggestions()
      const result = createPortResolveResult(data.stack, data.resolve)
      lastResolveResult.value = result
      resolveStatus.value = 'ready'
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
      const { data } = await workbenchApi.mutatePort(normalizedProjectName, payload)
      if (requestID !== portMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastPortMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      resetPortSuggestions()
      const result = createPortMutationResult(data.stack, data.mutation)
      lastPortMutationResult.value = result
      portMutationStatus.value = 'ready'
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
      resetPortSuggestions()
      const result = createResourceMutationResult(data.stack, data.mutation)
      lastResourceMutationResult.value = result
      resourceMutationStatus.value = 'ready'
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
      const { data } = await workbenchApi.mutateModule(normalizedProjectName, {
        ...payload,
        selector,
      })
      if (requestID !== moduleMutationRequestID || projectName.value !== normalizedProjectName) {
        return lastModuleMutationResult.value
      }

      applySnapshotState(data.stack, snapshot, snapshotStatus, snapshotError)
      resetPortSuggestions()
      const result = createModuleMutationResult(data.stack, data.mutation)
      lastModuleMutationResult.value = result
      moduleMutationStatus.value = 'ready'
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
    portSuggestionStatusByKey,
    portSuggestionErrorByKey,
    portSuggestionResultByKey,
    loading,
    submitting,
    ready,
    reset,
    loadSnapshot,
    runImport,
    resolvePorts,
    mutatePort,
    mutateResource,
    mutateModule,
    loadPortSuggestions,
    clearPortSuggestion,
  }
})
