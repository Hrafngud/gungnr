import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ApiError, parseApiError } from '@/services/api'
import { workbenchApi } from '@/services/workbench'
import type { WorkbenchImportReason, WorkbenchStackSnapshot } from '@/types/workbench'

export type WorkbenchRequestStatus = 'idle' | 'loading' | 'ready' | 'error'

export interface WorkbenchImportResult {
  projectName: string
  reason: WorkbenchImportReason
  changed: boolean
  idempotent: boolean
  revision: number
  sourceFingerprint: string
}

function normalizeProjectName(projectName: string): string {
  return projectName.trim()
}

function createProjectNameError(): ApiError {
  return new ApiError('Project name is required.')
}

export const useWorkbenchStore = defineStore('workbench', () => {
  const projectName = ref<string | null>(null)
  const snapshot = ref<WorkbenchStackSnapshot | null>(null)
  const snapshotStatus = ref<WorkbenchRequestStatus>('idle')
  const snapshotError = ref<ApiError | null>(null)
  const importStatus = ref<WorkbenchRequestStatus>('idle')
  const importError = ref<ApiError | null>(null)
  const lastImportResult = ref<WorkbenchImportResult | null>(null)

  const loading = computed(() => snapshotStatus.value === 'loading')
  const submitting = computed(() => importStatus.value === 'loading')
  const ready = computed(() => snapshotStatus.value === 'ready' && snapshot.value !== null)

  let snapshotRequestID = 0
  let importRequestID = 0

  const reset = () => {
    projectName.value = null
    snapshot.value = null
    snapshotStatus.value = 'idle'
    snapshotError.value = null
    importStatus.value = 'idle'
    importError.value = null
    lastImportResult.value = null
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

      snapshot.value = data.stack
      snapshotStatus.value = 'ready'
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

      snapshot.value = data.stack
      snapshotStatus.value = 'ready'
      snapshotError.value = null

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

  return {
    projectName,
    snapshot,
    snapshotStatus,
    snapshotError,
    importStatus,
    importError,
    lastImportResult,
    loading,
    submitting,
    ready,
    reset,
    loadSnapshot,
    runImport,
  }
})
