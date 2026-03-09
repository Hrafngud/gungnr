<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import ProjectActivityTimelineSection from '@/components/projectDetail/ProjectActivityTimelineSection.vue'
import ProjectArchiveExecutionSection from '@/components/projectDetail/ProjectArchiveExecutionSection.vue'
import ProjectRuntimeUnitsSection from '@/components/projectDetail/ProjectRuntimeUnitsSection.vue'
import ProjectSectionTabs from '@/components/projectDetail/ProjectSectionTabs.vue'
import WorkbenchServiceInspectorPanel from '@/components/workbench/WorkbenchServiceInspectorPanel.vue'
import WorkbenchCatalogControlsPanel from '@/components/workbench/WorkbenchCatalogControlsPanel.vue'
import type { ProjectDetailSectionTab } from '@/composables/projectDetail/useProjectDetailTabs'
import {
  useWorkbenchCatalogQuery,
  useWorkbenchComposeBackupsQuery,
  useWorkbenchSnapshotQuery,
} from '@/composables/workbench/useWorkbenchQueries'
import type {
  BadgeTone,
  WorkbenchComposeContextSummary,
  WorkbenchComposeIssueInventoryRow,
  WorkbenchInlineFeedbackState,
  WorkbenchOptionalServiceCatalogRow,
  WorkbenchPendingOptionalServiceMutation,
  WorkbenchPortInventoryRow,
  WorkbenchResourceEditorField,
  WorkbenchResourceInputState,
  WorkbenchResourceInventoryRow,
  WorkbenchServiceInventoryRow,
  WorkbenchTopologyInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import { projectsApi } from '@/services/projects'
import { ApiError, apiErrorMessage, parseApiError } from '@/services/api'
import { queryClient } from '@/services/queryClient'
import { refetchWorkbenchReadQueries } from '@/services/workbenchQueries'
import { useAuthStore } from '@/stores/auth'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import { useWorkbenchStore, type WorkbenchRequestStatus } from '@/stores/workbench'
import type {
  ProjectDetail,
} from '@/types/projects'
import {
  buildWorkbenchPortSelectorKey,
  type WorkbenchComposeBackupMetadata,
  type WorkbenchMutationIssue,
  type WorkbenchOptionalServiceCatalogEntry,
  type WorkbenchOptionalServiceMutationAction,
  type WorkbenchOptionalServiceMutationSummary,
  type WorkbenchPortMutationSummary,
  type WorkbenchPortSelector,
  type WorkbenchResourceField,
  type WorkbenchResourceMutationRequest,
  type WorkbenchResourceMutationSummary,
} from '@/types/workbench'

type WorkbenchRemediationAction = 'refresh' | 'import' | 'refresh_backups'

interface WorkbenchStructuredErrorDetails {
  revision?: number
  expectedRevision?: number
  sourceFingerprint?: string
  expectedSourceFingerprint?: string
  currentSourceFingerprint?: string
  composePath?: string
}

interface WorkbenchRemediationState {
  tone: BadgeTone
  sourceLabel: string
  title: string
  message: string
  primaryAction?: WorkbenchRemediationAction
  secondaryAction?: WorkbenchRemediationAction
  details?: WorkbenchStructuredErrorDetails
}

const route = useRoute()
const authStore = useAuthStore()
const toastStore = useToastStore()
const pageLoading = usePageLoadingStore()
const workbenchStore = useWorkbenchStore()
const loading = ref(false)
const error = ref<string | null>(null)
const detail = ref<ProjectDetail | null>(null)
const stackRestarting = ref(false)
const stackRestartError = ref<string | null>(null)
const workbenchRestoreSelectedBackupId = ref('')
const workbenchRestoreConfirmInput = ref('')
const workbenchPendingOptionalServiceMutation = ref<WorkbenchPendingOptionalServiceMutation | null>(null)
const workbenchPortManualInputs = ref<Record<string, string>>({})
const workbenchResourceInputs = ref<Record<string, WorkbenchResourceInputState>>({})
const workbenchSelectedServiceName = ref('')
const activeSectionTab = ref<ProjectDetailSectionTab>('workbench')
const isAdmin = computed(() => authStore.isAdmin)

const projectName = computed(() => {
  const raw = route.params.name
  if (typeof raw !== 'string') return ''
  return decodeURIComponent(raw).trim()
})

const workbenchComposeSupported = computed(() => (detail.value?.runtime.composeFiles?.length ?? 0) > 0)
const workbenchQueryEnabled = computed(
  () =>
    Boolean(projectName.value) &&
    Boolean(detail.value) &&
    workbenchComposeSupported.value,
)
const workbenchBackupsQueryEnabled = computed(() => workbenchQueryEnabled.value && isAdmin.value)
const workbenchSnapshotQuery = useWorkbenchSnapshotQuery(projectName, {
  enabled: workbenchQueryEnabled,
})
const workbenchCatalogQuery = useWorkbenchCatalogQuery(projectName, {
  enabled: workbenchQueryEnabled,
})
const workbenchComposeBackupsQuery = useWorkbenchComposeBackupsQuery(projectName, {
  enabled: workbenchBackupsQueryEnabled,
})

const workbenchQueryStatus = (
  enabled: boolean,
  pending: boolean,
  hasError: boolean,
): WorkbenchRequestStatus => {
  if (!enabled) return 'idle'
  if (pending) return 'loading'
  if (hasError) return 'error'
  return 'ready'
}

const workbenchSnapshot = computed(() => workbenchSnapshotQuery.data.value ?? null)
const workbenchStatus = computed<WorkbenchRequestStatus>(() =>
  workbenchQueryStatus(
    workbenchQueryEnabled.value,
    workbenchSnapshotQuery.isPending.value,
    workbenchSnapshotQuery.isError.value,
  ),
)
const workbenchError = computed<ApiError | null>(() => {
  if (workbenchStatus.value !== 'error') return null
  return parseApiError(workbenchSnapshotQuery.error.value)
})
const workbenchCatalog = computed(() => workbenchCatalogQuery.data.value ?? null)
const workbenchCatalogStatus = computed<WorkbenchRequestStatus>(() =>
  workbenchQueryStatus(
    workbenchQueryEnabled.value,
    workbenchCatalogQuery.isPending.value,
    workbenchCatalogQuery.isError.value,
  ),
)
const workbenchCatalogError = computed<ApiError | null>(() => {
  if (workbenchCatalogStatus.value !== 'error') return null
  return parseApiError(workbenchCatalogQuery.error.value)
})
const workbenchOptionalServiceMutationStatus = computed(
  () => workbenchStore.optionalServiceMutationStatus,
)
const workbenchOptionalServiceMutationError = computed(
  () => workbenchStore.optionalServiceMutationError,
)
const workbenchActiveOptionalServiceMutationEntryKey = computed(
  () => workbenchStore.activeOptionalServiceMutationEntryKey,
)
const workbenchLastOptionalServiceMutationResult = computed(
  () => workbenchStore.lastOptionalServiceMutationResult,
)
const workbenchImportStatus = computed(() => workbenchStore.importStatus)
const workbenchImportError = computed(() => workbenchStore.importError)
const workbenchLastImportResult = computed(() => workbenchStore.lastImportResult)
const workbenchResolveStatus = computed(() => workbenchStore.resolveStatus)
const workbenchResolveError = computed(() => workbenchStore.resolveError)
const workbenchLastResolveResult = computed(() => workbenchStore.lastResolveResult)
const workbenchPortMutationStatus = computed(() => workbenchStore.portMutationStatus)
const workbenchPortMutationError = computed(() => workbenchStore.portMutationError)
const workbenchActivePortMutationSelectorKey = computed(
  () => workbenchStore.activePortMutationSelectorKey,
)
const workbenchLastPortMutationResult = computed(() => workbenchStore.lastPortMutationResult)
const workbenchResourceMutationStatus = computed(() => workbenchStore.resourceMutationStatus)
const workbenchResourceMutationError = computed(() => workbenchStore.resourceMutationError)
const workbenchActiveResourceMutationServiceName = computed(
  () => workbenchStore.activeResourceMutationServiceName,
)
const workbenchLastResourceMutationResult = computed(() => workbenchStore.lastResourceMutationResult)
const workbenchPreviewStatus = computed(() => workbenchStore.previewStatus)
const workbenchPreviewError = computed(() => workbenchStore.previewError)
const workbenchLastPreviewResult = computed(() => workbenchStore.lastPreviewResult)
const workbenchApplyStatus = computed(() => workbenchStore.applyStatus)
const workbenchApplyError = computed(() => workbenchStore.applyError)
const workbenchLastApplyResult = computed(() => workbenchStore.lastApplyResult)
const workbenchComposeBackups = computed(() => workbenchComposeBackupsQuery.data.value ?? [])
const workbenchBackupInventoryStatus = computed<WorkbenchRequestStatus>(() =>
  workbenchQueryStatus(
    workbenchBackupsQueryEnabled.value,
    workbenchComposeBackupsQuery.isPending.value,
    workbenchComposeBackupsQuery.isError.value,
  ),
)
const workbenchBackupInventoryError = computed<ApiError | null>(() => {
  if (workbenchBackupInventoryStatus.value !== 'error') return null
  return parseApiError(workbenchComposeBackupsQuery.error.value)
})
const workbenchRestoreStatus = computed(() => workbenchStore.restoreStatus)
const workbenchRestoreError = computed(() => workbenchStore.restoreError)
const workbenchLastRestoreResult = computed(() => workbenchStore.lastRestoreResult)
const workbenchPortSuggestionStatusByKey = computed(() => workbenchStore.portSuggestionStatusByKey)
const workbenchPortSuggestionErrorByKey = computed(() => workbenchStore.portSuggestionErrorByKey)
const workbenchPortSuggestionResultByKey = computed(() => workbenchStore.portSuggestionResultByKey)
const workbenchAccessLabel = computed(() =>
  isAdmin.value ? 'Admin edits enabled' : 'Read-only visibility',
)
const workbenchSnapshotReady = computed(() => {
  const snapshot = workbenchSnapshot.value
  if (!snapshot) return false

  return Boolean(snapshot.sourceFingerprint?.trim()) || [
    snapshot.services.length,
    snapshot.dependencies.length,
    snapshot.ports.length,
    snapshot.resources.length,
    snapshot.networkRefs.length,
    snapshot.volumeRefs.length,
    snapshot.envRefs.length,
    snapshot.modules.length,
    snapshot.warnings.length,
  ].some((count) => count > 0)
})
const workbenchImportLabel = computed(() => {
  if (workbenchImportStatus.value === 'loading') return 'Importing compose...'
  return workbenchSnapshotReady.value ? 'Re-import compose' : 'Import compose'
})
const workbenchResolveLabel = computed(() => {
  if (workbenchResolveStatus.value === 'loading') return 'Resolving ports...'
  return 'Auto-resolve ports'
})
const workbenchStatusLabel = computed(() => {
  if (detail.value && !workbenchComposeSupported.value) return 'Unsupported'
  switch (workbenchStatus.value) {
    case 'loading':
      return 'Loading'
    case 'error':
      return 'Error'
    case 'ready':
      return workbenchSnapshotReady.value ? 'Ready' : 'Empty'
    default:
      return 'Idle'
  }
})
const workbenchStatusTone = computed<BadgeTone>(() => {
  if (detail.value && !workbenchComposeSupported.value) return 'neutral'
  switch (workbenchStatus.value) {
    case 'loading':
      return 'warn'
    case 'error':
      return 'error'
    case 'ready':
      return workbenchSnapshotReady.value ? 'ok' : 'neutral'
    default:
      return 'neutral'
  }
})
const workbenchAccessTone = computed<BadgeTone>(() => (isAdmin.value ? 'ok' : 'neutral'))
const workbenchErrorMessage = computed(() => {
  const parsedError = workbenchError.value
  if (!parsedError) return 'Workbench snapshot could not be loaded.'
  if (parsedError.code) return `[${parsedError.code}] ${parsedError.message}`
  return parsedError.message
})
const workbenchCatalogErrorMessage = computed(() => {
  const parsedError = workbenchCatalogError.value
  if (!parsedError) return 'Optional-service catalog could not be loaded.'
  if (parsedError.code) return `[${parsedError.code}] ${parsedError.message}`
  return parsedError.message
})
const workbenchImportFeedbackTone = computed<'ok' | 'warn'>(() => {
  const result = workbenchLastImportResult.value
  if (!result) return 'ok'
  return result.changed ? 'ok' : 'warn'
})
const workbenchImportFeedbackState = computed<WorkbenchInlineFeedbackState | null>(() => {
  if (workbenchImportError.value) {
    return workbenchMutationFeedbackFromError(workbenchImportError.value, 'import')
  }

  const result = workbenchLastImportResult.value
  if (!result) return null

  return {
    tone: workbenchImportFeedbackTone.value,
    message: workbenchImportFeedback.value,
  }
})
const workbenchImportFeedback = computed(() => {
  const result = workbenchLastImportResult.value
  if (!result) return ''
  if (result.changed) {
    return `Workbench snapshot imported at revision ${result.revision}.`
  }
  return `Workbench snapshot already matched the current compose at revision ${result.revision}.`
})
const workbenchResolveFeedback = computed<WorkbenchInlineFeedbackState | null>(() => {
  const parsedError = workbenchResolveError.value
  if (parsedError) {
    return workbenchMutationFeedbackFromError(parsedError, 'port resolution')
  }

  const result = workbenchLastResolveResult.value
  if (!result) return null

  if (!result.changed) {
    return {
      tone: result.conflict > 0 || result.unavailable > 0 ? 'warn' : 'ok',
      message: `Resolver found no persisted changes at revision ${result.revision}. Assigned ${result.assigned}, conflicts ${result.conflict}, unavailable ${result.unavailable}.`,
    }
  }

  return {
    tone: result.conflict > 0 || result.unavailable > 0 ? 'warn' : 'ok',
    message: `Resolver saved revision ${result.revision}. Assigned ${result.assigned}, conflicts ${result.conflict}, unavailable ${result.unavailable}.`,
  }
})
const workbenchPreviewFeedback = computed<WorkbenchInlineFeedbackState | null>(() => {
  if (workbenchComposeImportRequired.value) {
    return {
      tone: 'warn',
      message:
        'A restore changed the compose file on disk, so the stored Workbench snapshot is now stale. Re-import the restored compose before preview/apply.',
    }
  }

  if (workbenchPreviewError.value) {
    return workbenchComposeFeedbackFromError(workbenchPreviewError.value, 'preview')
  }

  const preview = workbenchLastPreviewResult.value
  if (!preview) return null

  if (!workbenchPreviewMatchesSnapshot.value) {
    return {
      tone: 'warn',
      message:
        'The last generated preview no longer matches the visible snapshot revision or source fingerprint. Generate a fresh preview before apply.',
    }
  }

  return {
    tone: 'ok',
    message: `Generated compose preview from revision ${preview.revision}. Apply remains enabled while the stored snapshot revision and fingerprint stay unchanged.`,
  }
})
const workbenchApplyFeedback = computed<WorkbenchInlineFeedbackState | null>(() => {
  if (workbenchApplyError.value) {
    return workbenchComposeFeedbackFromError(workbenchApplyError.value, 'apply')
  }

  const result = workbenchLastApplyResult.value
  if (!result) return null

  return {
    tone: 'ok',
    message: `Applied ${result.composeBytes}-byte compose to ${result.composePath}. Backup ${result.backupId} is retained (${result.retainedBackups} total, ${result.prunedBackups} pruned).`,
  }
})
const workbenchLatestComposeBackup = computed<WorkbenchComposeBackupMetadata | null>(() => {
  const backups = workbenchComposeBackups.value
  const latestBackup = backups.length > 0 ? backups[backups.length - 1] : null
  return latestBackup ?? null
})
const workbenchSelectedComposeBackup = computed<WorkbenchComposeBackupMetadata | null>(() => {
  const selectedBackupId = workbenchRestoreSelectedBackupId.value.trim().toLowerCase()
  if (!selectedBackupId) return null

  return (
    workbenchComposeBackups.value.find((backup) => backup.backupId === selectedBackupId) ?? null
  )
})
const workbenchComposeImportRequired = computed(() => {
  const restore = workbenchLastRestoreResult.value
  if (!restore?.requiresImport) return false

  const snapshotFingerprint = workbenchSnapshot.value?.sourceFingerprint?.trim() || ''
  return snapshotFingerprint !== restore.restoredFingerprint.trim()
})
const workbenchBackupInventoryErrorMessage = computed(() => {
  const parsedError = workbenchBackupInventoryError.value
  if (!parsedError) return ''
  return parsedError.code ? `[${parsedError.code}] ${parsedError.message}` : parsedError.message
})
const workbenchRestoreConfirmationPhrase = computed(() => {
  const selectedBackup = workbenchSelectedComposeBackup.value
  return selectedBackup ? `RESTORE ${selectedBackup.backupId}` : 'RESTORE BACKUP'
})
const workbenchRestoreLabel = computed(() => {
  if (workbenchRestoreStatus.value === 'loading') return 'Restoring compose...'
  return 'Restore selected backup'
})
const workbenchRestoreActionDisabled = computed(() => {
  if (!isAdmin.value) return true
  return (
    workbenchStatus.value === 'loading' ||
    workbenchImportStatus.value === 'loading' ||
    workbenchResolveStatus.value === 'loading' ||
    workbenchPortMutationStatus.value === 'loading' ||
    workbenchResourceMutationStatus.value === 'loading' ||
    workbenchOptionalServiceMutationStatus.value === 'loading' ||
    workbenchPreviewStatus.value === 'loading' ||
    workbenchApplyStatus.value === 'loading' ||
    workbenchRestoreStatus.value === 'loading' ||
    workbenchBackupInventoryStatus.value === 'loading' ||
    !workbenchSnapshotReady.value ||
    !workbenchSelectedComposeBackup.value
  )
})
const canRestoreWorkbenchCompose = computed(() => {
  if (workbenchRestoreActionDisabled.value) return false
  return workbenchRestoreConfirmInput.value.trim() === workbenchRestoreConfirmationPhrase.value
})
const workbenchRestoreFeedback = computed<WorkbenchInlineFeedbackState | null>(() => {
  if (workbenchRestoreError.value) {
    return workbenchRestoreFeedbackFromError(workbenchRestoreError.value)
  }

  const result = workbenchLastRestoreResult.value
  if (!result) return null

  if (result.requiresImport) {
    return {
      tone: 'warn',
      message: `Restored backup ${result.backupId} (${result.composeBytes} bytes). The compose file now differs from the stored Workbench snapshot, so re-import before preview/apply.`,
    }
  }

  return {
    tone: 'ok',
    message: `Restored backup ${result.backupId} to ${result.composePath}. Preview/apply state was reset to avoid using stale compose output.`,
  }
})
const workbenchComposeRemediationState = computed<WorkbenchRemediationState | null>(() => {
  if (workbenchComposeImportRequired.value) {
    return {
      tone: 'warn',
      sourceLabel: 'Restore follow-up',
      title: 'Re-import restored compose',
      message:
        'The last restore changed the on-disk compose artifact. Preview and apply remain blocked until the stored Workbench snapshot is imported from that restored file.',
      primaryAction: 'import',
      details: {
        sourceFingerprint: workbenchSnapshot.value?.sourceFingerprint?.trim() || undefined,
        currentSourceFingerprint:
          workbenchLastRestoreResult.value?.restoredFingerprint?.trim() || undefined,
      },
    }
  }

  const applyError = workbenchApplyError.value
  if (applyError?.code === 'WB-409-DRIFT-DETECTED') {
    return {
      tone: 'warn',
      sourceLabel: 'Apply guard',
      title: 'Compose drift detected',
      message:
        'The compose file on disk no longer matches the stored Workbench snapshot. Import the current compose source, then generate a fresh preview before applying again.',
      primaryAction: 'import',
      secondaryAction: 'refresh',
      details: workbenchStructuredErrorDetailsFromUnknown(applyError.details),
    }
  }

  if (applyError?.code === 'WB-409-STALE-REVISION') {
    return {
      tone: 'warn',
      sourceLabel: 'Apply guard',
      title: 'Snapshot revision changed',
      message:
        'Apply was submitted against an older Workbench revision. Refresh the shell to load the current snapshot, then generate a fresh preview before retrying apply.',
      primaryAction: 'refresh',
      details: workbenchStructuredErrorDetailsFromUnknown(applyError.details),
    }
  }

  const previewError = workbenchPreviewError.value
  if (
    previewError?.code === 'WB-422-VALIDATION' &&
    workbenchIssueHasCode(previewError.details, 'WB-VAL-EXPECTED-REVISION-MISMATCH')
  ) {
    return {
      tone: 'warn',
      sourceLabel: 'Preview guard',
      title: 'Preview requested from a stale revision',
      message:
        'The preview request used a revision that no longer matches the stored Workbench snapshot. Refresh the shell first so preview runs against the latest revision.',
      primaryAction: 'refresh',
      details: workbenchStructuredErrorDetailsFromUnknown(previewError.details),
    }
  }

  if (previewError?.code === 'WB-409-LOCKED' || applyError?.code === 'WB-409-LOCKED') {
    return {
      tone: 'warn',
      sourceLabel: previewError?.code === 'WB-409-LOCKED' ? 'Preview guard' : 'Apply guard',
      title: 'Workbench is busy',
      message:
        'Another Workbench operation still holds the project lock. Wait for it to finish, then retry the blocked action from this shell.',
      primaryAction: 'refresh',
    }
  }

  if (
    (previewError?.code === 'WB-422-VALIDATION' &&
      !workbenchIssueHasCode(previewError.details, 'WB-VAL-EXPECTED-REVISION-MISMATCH')) ||
    applyError?.code === 'WB-422-VALIDATION'
  ) {
    return {
      tone: 'warn',
      sourceLabel: applyError?.code === 'WB-422-VALIDATION' ? 'Apply guard' : 'Preview guard',
      title: 'Validation blockers are active',
      message:
        'The stored Workbench model still has validation issues. Resolve the listed blockers below, then generate a fresh preview before applying again.',
    }
  }

  return null
})
const workbenchRestoreRemediationState = computed<WorkbenchRemediationState | null>(() => {
  if (workbenchComposeImportRequired.value) {
    return {
      tone: 'warn',
      sourceLabel: 'Restore follow-up',
      title: 'Import is still required',
      message:
        'This restored backup left the Workbench snapshot out of sync with the compose file on disk. Import the restored compose before preview/apply resumes.',
      primaryAction: 'import',
      details: {
        sourceFingerprint: workbenchSnapshot.value?.sourceFingerprint?.trim() || undefined,
        currentSourceFingerprint:
          workbenchLastRestoreResult.value?.restoredFingerprint?.trim() || undefined,
      },
    }
  }

  const parsedError = workbenchRestoreError.value
  if (!parsedError) return null

  if (
    parsedError.code === 'WB-404-BACKUP' ||
    parsedError.code === 'WB-409-BACKUP-INTEGRITY'
  ) {
    return {
      tone: 'warn',
      sourceLabel: 'Restore guard',
      title: 'Backup target needs refresh',
      message:
        'The selected restore target could not be used safely. Refresh the retained backup inventory before choosing another restore target.',
      primaryAction: 'refresh_backups',
    }
  }

  if (parsedError.code === 'WB-409-LOCKED') {
    return {
      tone: 'warn',
      sourceLabel: 'Restore guard',
      title: 'Restore is waiting on another operation',
      message:
        'Another Workbench mutation still owns the project lock. Wait for it to finish, then retry the restore from this panel.',
      primaryAction: 'refresh_backups',
    }
  }

  return null
})
const workbenchFingerprintLabel = computed(() => {
  const fingerprint = workbenchSnapshot.value?.sourceFingerprint?.trim()
  return fingerprint || 'Not imported yet'
})
const workbenchServiceInventory = computed<WorkbenchServiceInventoryRow[]>(() => {
  const snapshot = workbenchSnapshot.value
  if (!snapshot) return []

  const dependenciesByService = new Map<string, string[]>()
  const portCountByService = new Map<string, number>()
  const networkCountByService = new Map<string, number>()
  const managedEntryKeysByService = new Map<string, string[]>()
  const legacyModuleTypesByService = new Map<string, string[]>()

  const incrementCount = (targetMap: Map<string, number>, serviceName: string) => {
    targetMap.set(serviceName, (targetMap.get(serviceName) ?? 0) + 1)
  }

  const appendUnique = (targetMap: Map<string, string[]>, serviceName: string, value: string) => {
    const currentValues = targetMap.get(serviceName)
    if (currentValues) {
      if (!currentValues.includes(value)) currentValues.push(value)
      return
    }
    targetMap.set(serviceName, [value])
  }

  for (const dependency of snapshot.dependencies) {
    const serviceDependencies = dependenciesByService.get(dependency.serviceName)
    if (serviceDependencies) {
      serviceDependencies.push(dependency.dependsOn)
      continue
    }
    dependenciesByService.set(dependency.serviceName, [dependency.dependsOn])
  }

  for (const port of snapshot.ports) {
    incrementCount(portCountByService, port.serviceName)
  }

  for (const networkRef of snapshot.networkRefs) {
    incrementCount(networkCountByService, networkRef.serviceName)
  }

  for (const managedService of snapshot.managedServices) {
    appendUnique(managedEntryKeysByService, managedService.serviceName, managedService.entryKey)
  }

  for (const module of snapshot.modules) {
    const serviceName = module.serviceName?.trim()
    const moduleType = module.moduleType?.trim()
    if (!serviceName || !moduleType) continue
    appendUnique(legacyModuleTypesByService, serviceName, moduleType)
  }

  return snapshot.services.map((service) => {
    const managedEntryKeys = managedEntryKeysByService.get(service.serviceName) ?? []
    const legacyModuleTypes = legacyModuleTypesByService.get(service.serviceName) ?? []

    return {
      serviceName: service.serviceName,
      image: service.image?.trim() || null,
      buildSource: service.buildSource?.trim() || null,
      restartPolicy: service.restartPolicy?.trim() || null,
      dependencies: dependenciesByService.get(service.serviceName) ?? [],
      portCount: portCountByService.get(service.serviceName) ?? 0,
      networkCount: networkCountByService.get(service.serviceName) ?? 0,
      managedEntryKeys,
      legacyModuleTypes,
      originLabel:
        managedEntryKeys.length > 0
          ? 'Catalog-managed'
          : legacyModuleTypes.length > 0
            ? 'Legacy metadata'
            : 'Imported compose',
      originTone:
        managedEntryKeys.length > 0 ? 'ok' : legacyModuleTypes.length > 0 ? 'warn' : 'neutral',
    }
  })
})
const workbenchSelectedService = computed<WorkbenchServiceInventoryRow | null>(() => {
  const inventory = workbenchServiceInventory.value
  if (inventory.length === 0) return null

  const selectedServiceName = workbenchSelectedServiceName.value.trim().toLowerCase()
  if (!selectedServiceName) return inventory[0] ?? null

  return (
    inventory.find((service) => service.serviceName.trim().toLowerCase() === selectedServiceName) ??
    inventory[0] ??
    null
  )
})
const workbenchSelectedServiceTopology = computed<WorkbenchTopologyInventoryRow | null>(() => {
  const serviceName = workbenchSelectedService.value?.serviceName
  if (!serviceName) return null
  return workbenchTopologyInventory.value.find((row) => row.serviceName === serviceName) ?? null
})
const workbenchSelectedServicePorts = computed<WorkbenchPortInventoryRow[]>(() => {
  const serviceName = workbenchSelectedService.value?.serviceName
  if (!serviceName) return []
  return workbenchPortInventory.value.filter((port) => port.serviceName === serviceName)
})
const workbenchSelectedServiceResource = computed<WorkbenchResourceInventoryRow | null>(() => {
  const serviceName = workbenchSelectedService.value?.serviceName
  if (!serviceName) return null
  return workbenchResourceInventory.value.find((resource) => resource.serviceName === serviceName) ?? null
})
const workbenchWarningsList = computed(() => workbenchSnapshot.value?.warnings ?? [])
const workbenchPreviewIssues = computed(() =>
  workbenchPreviewError.value ? workbenchIssueListFromDetails(workbenchPreviewError.value.details) : [],
)
const workbenchApplyIssues = computed(() =>
  workbenchApplyError.value ? workbenchIssueListFromDetails(workbenchApplyError.value.details) : [],
)
const workbenchComposeIssueInventory = computed<WorkbenchComposeIssueInventoryRow[]>(() => [
  ...workbenchPreviewIssues.value.map((issue, index) => ({
    key: `preview-${issue.code}-${issue.path}-${index}`,
    source: 'preview' as const,
    sourceLabel: 'Preview',
    issue,
  })),
  ...workbenchApplyIssues.value.map((issue, index) => ({
    key: `apply-${issue.code}-${issue.path}-${index}`,
    source: 'apply' as const,
    sourceLabel: 'Apply',
    issue,
  })),
])
const workbenchPreviewMatchesSnapshot = computed(() => {
  const preview = workbenchLastPreviewResult.value
  const snapshot = workbenchSnapshot.value
  if (!preview || !snapshot) return false
  return (
    preview.revision === snapshot.revision &&
    preview.sourceFingerprint.trim() === snapshot.sourceFingerprint.trim()
  )
})
const workbenchHasCleanPreview = computed(() => {
  const preview = workbenchLastPreviewResult.value
  return (
    Boolean(preview?.compose) &&
    !workbenchPreviewError.value &&
    !workbenchComposeImportRequired.value &&
    workbenchPreviewMatchesSnapshot.value
  )
})
const workbenchPreviewLabel = computed(() => {
  if (workbenchComposeImportRequired.value) return 'Import required before preview'
  if (workbenchPreviewStatus.value === 'loading') return 'Generating preview...'
  return 'Generate preview'
})
const workbenchApplyLabel = computed(() => {
  if (workbenchApplyStatus.value === 'loading') return 'Applying compose...'
  return 'Apply compose'
})
const workbenchPreviewStatusLabel = computed(() => {
  if (workbenchComposeImportRequired.value) return 'Import required'
  if (workbenchPreviewStatus.value === 'loading') return 'Generating'
  if (workbenchHasCleanPreview.value) return 'Clean preview'
  if (workbenchPreviewError.value) return 'Preview blocked'
  if (workbenchLastPreviewResult.value) return 'Preview stale'
  return 'Preview required'
})
const workbenchPreviewStatusTone = computed<BadgeTone>(() => {
  if (workbenchComposeImportRequired.value) return 'warn'
  if (workbenchPreviewStatus.value === 'loading') return 'warn'
  if (workbenchHasCleanPreview.value) return 'ok'
  if (workbenchPreviewError.value) {
    return isWorkbenchComposeBlockedError(workbenchPreviewError.value, 'preview') ? 'warn' : 'error'
  }
  if (workbenchLastPreviewResult.value) return 'warn'
  return 'neutral'
})
const workbenchPreviewComposeLineCount = computed(
  () => workbenchLastPreviewResult.value?.compose.split('\n').length ?? 0,
)
const workbenchPreviewFingerprintLabel = computed(() => {
  const fingerprint = workbenchLastPreviewResult.value?.sourceFingerprint?.trim()
  return fingerprint || 'No preview generated'
})
const workbenchComposeActionBusy = computed(() =>
  workbenchPreviewStatus.value === 'loading' ||
  workbenchApplyStatus.value === 'loading' ||
  workbenchRestoreStatus.value === 'loading',
)
const workbenchApplyActionDisabled = computed(() => {
  if (!isAdmin.value) return true
  return (
    workbenchComposeActionBusy.value ||
    workbenchComposeImportRequired.value ||
    workbenchImportStatus.value === 'loading' ||
    workbenchResolveStatus.value === 'loading' ||
    workbenchPortMutationStatus.value === 'loading' ||
    workbenchResourceMutationStatus.value === 'loading' ||
    workbenchOptionalServiceMutationStatus.value === 'loading' ||
    !workbenchHasCleanPreview.value
  )
})
const workbenchPortInventory = computed<WorkbenchPortInventoryRow[]>(() => {
  const snapshot = workbenchSnapshot.value
  if (!snapshot) return []

  return snapshot.ports.map((port, index) => {
    const normalizedProtocol = port.protocol?.trim().toLowerCase() || 'tcp'
    const hostIp = port.hostIp?.trim() || '0.0.0.0'
    const selector = {
      serviceName: port.serviceName,
      containerPort: port.containerPort,
      protocol: normalizedProtocol,
      hostIp,
    } satisfies WorkbenchPortSelector
    const assignmentStrategy = port.assignmentStrategy?.trim().toLowerCase() || 'auto'
    const requestedHostPort = port.hostPortRaw?.trim() || null
    const effectiveHostPort = port.hostPort != null ? String(port.hostPort) : null
    const allocationStatus =
      port.allocationStatus?.trim().toLowerCase() ||
      (effectiveHostPort ? 'assigned' : requestedHostPort ? 'unresolved' : 'unavailable')
    const visibleHostPort = effectiveHostPort || requestedHostPort || 'unassigned'
    let guidance = 'Compose-declared mapping is available and tracked read-only.'

    if (allocationStatus === 'conflict') {
      guidance = 'This host binding conflicts with another reservation and needs operator review in a later slice.'
    } else if (allocationStatus === 'unresolved') {
      guidance =
        'This mapping preserves a raw compose host-port expression, so Workbench keeps it neutral until a resolver or env-backed runtime pass assigns a concrete binding.'
    } else if (allocationStatus === 'unavailable') {
      guidance = 'No host binding could be assigned from the current resolver candidates.'
    } else if (assignmentStrategy === 'manual') {
      guidance = 'This mapping is pinned manually and bypasses sequential fallback changes.'
    } else if (requestedHostPort) {
      guidance = 'Auto allocation prefers the compose-declared host port before trying the next sequential candidate.'
    } else if (effectiveHostPort) {
      guidance = 'Auto allocation resolved this host binding from the current candidate sequence.'
    }

    return {
      key: buildWorkbenchPortSelectorKey(selector) || `${port.serviceName}-${port.containerPort}-${normalizedProtocol}-${hostIp}-${index}`,
      selector,
      serviceName: port.serviceName,
      containerPort: port.containerPort,
      protocol: normalizedProtocol,
      hostIp,
      assignmentStrategy,
      assignmentStrategyLabel: assignmentStrategy === 'manual' ? 'Manual' : 'Auto',
      assignmentStrategyTone: assignmentStrategy === 'manual' ? 'ok' : 'neutral',
      allocationStatus,
      allocationStatusLabel:
        allocationStatus === 'conflict'
          ? 'Conflict'
          : allocationStatus === 'unavailable'
            ? 'Unavailable'
            : allocationStatus === 'unresolved'
              ? 'Unresolved'
              : 'Assigned',
      allocationStatusTone:
        allocationStatus === 'conflict'
          ? 'warn'
          : allocationStatus === 'unavailable'
            ? 'error'
            : allocationStatus === 'unresolved'
              ? 'neutral'
              : 'ok',
      requestedHostPort,
      effectiveHostPort,
      effectiveHostPortLabel:
        effectiveHostPort || (allocationStatus === 'unresolved' ? 'Pending resolution' : 'Unavailable'),
      mappingLabel: `${hostIp}:${visibleHostPort} -> ${port.containerPort}/${normalizedProtocol}`,
      guidance,
    }
  })
})
const workbenchResourceInventory = computed<WorkbenchResourceInventoryRow[]>(() => {
  const snapshot = workbenchSnapshot.value
  if (!snapshot) return []

  const resourcesByService = new Map(snapshot.resources.map((resource) => [resource.serviceName, resource]))

  return snapshot.services.map((service) => {
    const resource = resourcesByService.get(service.serviceName)
    const limitCpus = resource?.limitCpus?.trim() || null
    const limitMemory = resource?.limitMemory?.trim() || null
    const reservationCpus = resource?.reservationCpus?.trim() || null
    const reservationMemory = resource?.reservationMemory?.trim() || null

    return {
      key: service.serviceName,
      serviceName: service.serviceName,
      tracked: Boolean(resource),
      limitCpus,
      limitMemory,
      reservationCpus,
      reservationMemory,
      hasLimits: Boolean(limitCpus || limitMemory),
      hasReservations: Boolean(reservationCpus || reservationMemory),
    }
  })
})
function workbenchOptionalServiceAvailabilityTone(status: string): BadgeTone {
  switch (status.trim().toLowerCase()) {
    case 'catalog_managed':
    case 'catalog_managed_with_compose':
      return 'ok'
    case 'catalog_managed_with_legacy_module':
    case 'catalog_managed_with_compose_and_legacy_module':
    case 'compose_present_with_legacy_module':
    case 'legacy_module_only':
      return 'warn'
    case 'compose_present':
      return 'neutral'
    default:
      return 'neutral'
  }
}

function workbenchOptionalServiceAvailabilityLabel(status: string): string {
  switch (status.trim().toLowerCase()) {
    case 'catalog_managed':
      return 'Catalog-managed'
    case 'catalog_managed_with_compose':
      return 'Managed + compose match'
    case 'catalog_managed_with_legacy_module':
      return 'Managed + legacy metadata'
    case 'catalog_managed_with_compose_and_legacy_module':
      return 'Managed + compose + legacy'
    case 'compose_present':
      return 'Detected in compose'
    case 'compose_present_with_legacy_module':
      return 'Compose + legacy metadata'
    case 'legacy_module_only':
      return 'Legacy metadata only'
    default:
      return 'Available'
  }
}

function workbenchOptionalServiceStateLabel(state: string): string {
  switch (state.trim().toLowerCase()) {
    case 'catalog_managed':
      return 'Catalog-managed'
    case 'legacy_modules':
      return 'Legacy transition metadata'
    default:
      return 'Unmanaged'
  }
}

function workbenchOptionalServiceStateTone(state: string): BadgeTone {
  switch (state.trim().toLowerCase()) {
    case 'catalog_managed':
      return 'ok'
    case 'legacy_modules':
      return 'warn'
    default:
      return 'neutral'
  }
}

function workbenchOptionalServiceTargetLabel(state: string): string {
  if (state.trim().toLowerCase() === 'catalog_managed') return 'Catalog-managed'
  return 'Unchanged'
}

function workbenchOptionalServicePortLabel(containerPort: number): string {
  if (!Number.isFinite(containerPort) || containerPort <= 0) return 'Not declared'
  return `${containerPort}/tcp baseline`
}

const workbenchCurrentComposeSummary = computed<WorkbenchComposeContextSummary>(() => ({
  importedServices: workbenchSnapshot.value?.services.length ?? 0,
  catalogManagedServices: workbenchSnapshot.value?.managedServices.length ?? 0,
}))
const workbenchOptionalServiceInventory = computed<WorkbenchOptionalServiceCatalogRow[]>(() => {
  const catalog = workbenchCatalog.value
  if (!catalog) return []

  return catalog.entries.map((entry: WorkbenchOptionalServiceCatalogEntry) => ({
    key: entry.key,
    displayName: entry.displayName,
    description: entry.description,
    category: entry.category,
    defaultServiceName: entry.defaultServiceName,
    suggestedImage: entry.suggestedImage?.trim() || null,
    defaultContainerPortLabel: workbenchOptionalServicePortLabel(entry.defaultContainerPort),
    availabilityLabel: workbenchOptionalServiceAvailabilityLabel(entry.availability.status),
    availabilityTone: workbenchOptionalServiceAvailabilityTone(entry.availability.status),
    composeServices: entry.availability.composeServices,
    managedServices: entry.availability.managedServices,
    legacyModules: entry.availability.legacyModules,
    currentStateLabel: workbenchOptionalServiceStateLabel(entry.transition.currentState),
    currentStateTone: workbenchOptionalServiceStateTone(entry.transition.currentState),
    targetStateLabel: workbenchOptionalServiceTargetLabel(entry.transition.targetState),
    mutationReady: entry.transition.mutationReady,
    composeGenerationReady: entry.transition.composeGenerationReady,
    legacyModuleType: entry.transition.legacyModuleType?.trim() || null,
    legacyMutationPath: entry.transition.legacyMutationPath?.trim() || null,
    notes: entry.transition.notes,
  }))
})

function workbenchOptionalServiceManagedServiceName(
  entry: WorkbenchOptionalServiceCatalogRow,
): string | null {
  return entry.managedServices[0]?.serviceName?.trim() || null
}

function workbenchOptionalServicePendingAction(
  entry: WorkbenchOptionalServiceCatalogRow,
): WorkbenchOptionalServiceMutationAction {
  return entry.managedServices.length > 0 ? 'remove' : 'add'
}

function workbenchOptionalServicePendingLabel(entry: WorkbenchOptionalServiceCatalogRow): string {
  return workbenchOptionalServicePendingAction(entry) === 'remove' ? 'Remove service' : 'Add service'
}

function workbenchOptionalServiceBusy(entry: WorkbenchOptionalServiceCatalogRow): boolean {
  return (
    workbenchOptionalServiceMutationStatus.value === 'loading' &&
    workbenchActiveOptionalServiceMutationEntryKey.value === entry.key
  )
}

function workbenchOptionalServiceActionDisabled(entry: WorkbenchOptionalServiceCatalogRow): boolean {
  return (
    workbenchCatalogStatus.value === 'loading' ||
    workbenchOptionalServiceMutationStatus.value === 'loading' ||
    workbenchImportStatus.value === 'loading' ||
    workbenchResolveStatus.value === 'loading' ||
    workbenchPortMutationStatus.value === 'loading' ||
    workbenchResourceMutationStatus.value === 'loading' ||
    workbenchPreviewStatus.value === 'loading' ||
    workbenchApplyStatus.value === 'loading' ||
    workbenchRestoreStatus.value === 'loading' ||
    !entry.mutationReady
  )
}

function workbenchOptionalServicePendingConfirmation(
  entry: WorkbenchOptionalServiceCatalogRow,
): boolean {
  return workbenchPendingOptionalServiceMutation.value?.entryKey === entry.key
}

function workbenchOptionalServiceFeedback(
  entry: WorkbenchOptionalServiceCatalogRow,
): WorkbenchInlineFeedbackState | null {
  const successfulResult = workbenchLastOptionalServiceMutationResult.value
  if (successfulResult?.entryKey === entry.key) {
    if (!successfulResult.changed) {
      return {
        tone: 'warn',
        message: 'No optional-service mutation changes were required.',
      }
    }

    const actionLabel =
      successfulResult.action === 'remove'
        ? `Removed ${successfulResult.serviceName || entry.displayName} at revision ${successfulResult.revision}.`
        : `Added ${successfulResult.serviceName || entry.defaultServiceName} at revision ${successfulResult.revision}.`
    const notes = successfulResult.notes.filter((note) => note.trim())
    return {
      tone: successfulResult.composeGenerationReady ? 'ok' : 'warn',
      message: notes.length > 0 ? `${actionLabel} ${notes[0]}` : actionLabel,
    }
  }

  const parsedError = workbenchOptionalServiceMutationError.value
  if (!parsedError) return null

  const summary = workbenchOptionalServiceMutationSummaryFromDetails(parsedError.details)
  if (!summary) return null

  const normalizedEntryKey = summary.entryKey?.trim().toLowerCase() || ''
  const normalizedServiceName = summary.serviceName?.trim().toLowerCase() || ''
  const rowManagedServiceName = workbenchOptionalServiceManagedServiceName(entry)?.toLowerCase() || ''
  const rowDefaultServiceName = entry.defaultServiceName.trim().toLowerCase()

  if (
    normalizedEntryKey !== entry.key &&
    normalizedServiceName !== rowManagedServiceName &&
    normalizedServiceName !== rowDefaultServiceName
  ) {
    return null
  }

  return workbenchMutationFeedbackFromError(parsedError, 'optional-service')
}

const workbenchTopologyInventory = computed<WorkbenchTopologyInventoryRow[]>(() => {
  const snapshot = workbenchSnapshot.value
  if (!snapshot) return []

  const serviceNames: string[] = []
  const seenServiceNames = new Set<string>()
  const dependsOnByService = new Map<string, string[]>()
  const dependedByByService = new Map<string, string[]>()
  const networkNamesByService = new Map<string, string[]>()
  const moduleTypesByService = new Map<string, string[]>()

  const trackServiceName = (value?: string | null) => {
    const normalized = value?.trim()
    if (!normalized || seenServiceNames.has(normalized)) return
    seenServiceNames.add(normalized)
    serviceNames.push(normalized)
  }

  const appendUnique = (targetMap: Map<string, string[]>, serviceName: string, value: string) => {
    const normalizedServiceName = serviceName.trim()
    const normalizedValue = value.trim()
    if (!normalizedServiceName || !normalizedValue) return

    const currentValues = targetMap.get(normalizedServiceName)
    if (currentValues) {
      if (!currentValues.includes(normalizedValue)) currentValues.push(normalizedValue)
      return
    }

    targetMap.set(normalizedServiceName, [normalizedValue])
  }

  for (const service of snapshot.services) {
    trackServiceName(service.serviceName)
  }

  for (const dependency of snapshot.dependencies) {
    trackServiceName(dependency.serviceName)
    trackServiceName(dependency.dependsOn)
    appendUnique(dependsOnByService, dependency.serviceName, dependency.dependsOn)
    appendUnique(dependedByByService, dependency.dependsOn, dependency.serviceName)
  }

  for (const networkRef of snapshot.networkRefs) {
    trackServiceName(networkRef.serviceName)
    appendUnique(networkNamesByService, networkRef.serviceName, networkRef.networkName)
  }

  for (const module of snapshot.modules) {
    const serviceName = module.serviceName?.trim()
    if (!serviceName) continue
    trackServiceName(serviceName)
    appendUnique(moduleTypesByService, serviceName, module.moduleType)
  }

  return serviceNames.map((serviceName) => ({
    key: serviceName,
    serviceName,
    dependsOn: dependsOnByService.get(serviceName) ?? [],
    dependedBy: dependedByByService.get(serviceName) ?? [],
    networkNames: networkNamesByService.get(serviceName) ?? [],
    moduleTypes: moduleTypesByService.get(serviceName) ?? [],
  }))
})
const workbenchResourceFieldOrder: WorkbenchResourceField[] = [
  'limitCpus',
  'limitMemory',
  'reservationCpus',
  'reservationMemory',
]

const workbenchResourceFieldLabels: Record<WorkbenchResourceField, string> = {
  limitCpus: 'limit CPU',
  limitMemory: 'limit memory',
  reservationCpus: 'reservation CPU',
  reservationMemory: 'reservation memory',
}

const workbenchResourceEditorFields: WorkbenchResourceEditorField[] = [
  {
    key: 'limitCpus',
    label: 'Limit CPU',
    placeholder: '0.50 or ${LIMIT_CPUS}',
    section: 'limits',
  },
  {
    key: 'limitMemory',
    label: 'Limit memory',
    placeholder: '512M or ${LIMIT_MEMORY}',
    section: 'limits',
  },
  {
    key: 'reservationCpus',
    label: 'Reservation CPU',
    placeholder: '0.25 or ${RESERVE_CPUS}',
    section: 'reservations',
  },
  {
    key: 'reservationMemory',
    label: 'Reservation memory',
    placeholder: '256M or ${RESERVE_MEMORY}',
    section: 'reservations',
  },
]

function createWorkbenchResourceInputState(resource?: {
  limitCpus?: string | null
  limitMemory?: string | null
  reservationCpus?: string | null
  reservationMemory?: string | null
} | null): WorkbenchResourceInputState {
  return {
    limitCpus: resource?.limitCpus?.trim() || '',
    limitMemory: resource?.limitMemory?.trim() || '',
    reservationCpus: resource?.reservationCpus?.trim() || '',
    reservationMemory: resource?.reservationMemory?.trim() || '',
  }
}

function workbenchIssueListFromDetails(details: unknown): WorkbenchMutationIssue[] {
  if (!details || typeof details !== 'object') return []
  const rawIssues = (details as Record<string, unknown>).issues
  if (!Array.isArray(rawIssues)) return []
  return rawIssues.filter(
    (issue): issue is WorkbenchMutationIssue =>
      Boolean(issue) &&
      typeof issue === 'object' &&
      typeof (issue as Record<string, unknown>).code === 'string' &&
      typeof (issue as Record<string, unknown>).message === 'string',
  )
}

function workbenchIssueHasCode(details: unknown, code: string): boolean {
  return workbenchIssueListFromDetails(details).some((issue) => issue.code === code)
}

function workbenchStructuredErrorDetailsFromUnknown(
  details: unknown,
): WorkbenchStructuredErrorDetails | undefined {
  if (!details || typeof details !== 'object') return undefined
  const record = details as Record<string, unknown>
  const result: WorkbenchStructuredErrorDetails = {}

  if (typeof record.revision === 'number' && Number.isFinite(record.revision)) {
    result.revision = record.revision
  }
  if (typeof record.expectedRevision === 'number' && Number.isFinite(record.expectedRevision)) {
    result.expectedRevision = record.expectedRevision
  }
  if (typeof record.sourceFingerprint === 'string' && record.sourceFingerprint.trim()) {
    result.sourceFingerprint = record.sourceFingerprint.trim()
  }
  if (
    typeof record.expectedSourceFingerprint === 'string' &&
    record.expectedSourceFingerprint.trim()
  ) {
    result.expectedSourceFingerprint = record.expectedSourceFingerprint.trim()
  }
  if (
    typeof record.currentSourceFingerprint === 'string' &&
    record.currentSourceFingerprint.trim()
  ) {
    result.currentSourceFingerprint = record.currentSourceFingerprint.trim()
  }
  if (typeof record.composePath === 'string' && record.composePath.trim()) {
    result.composePath = record.composePath.trim()
  }

  return Object.keys(result).length > 0 ? result : undefined
}

function workbenchMutationSummaryFromDetails(details: unknown): WorkbenchPortMutationSummary | null {
  if (!details || typeof details !== 'object') return null
  const summary = (details as Record<string, unknown>).summary
  if (!summary || typeof summary !== 'object') return null
  if (
    typeof (summary as Record<string, unknown>).action !== 'string' ||
    !('selector' in (summary as Record<string, unknown>))
  ) {
    return null
  }
  return summary as WorkbenchPortMutationSummary
}

function workbenchResourceMutationSummaryFromDetails(details: unknown): WorkbenchResourceMutationSummary | null {
  if (!details || typeof details !== 'object') return null
  const summary = (details as Record<string, unknown>).summary
  if (!summary || typeof summary !== 'object') return null
  if (
    typeof (summary as Record<string, unknown>).action !== 'string' ||
    !('selector' in (summary as Record<string, unknown>))
  ) {
    return null
  }
  return summary as WorkbenchResourceMutationSummary
}

function workbenchOptionalServiceMutationSummaryFromDetails(
  details: unknown,
): WorkbenchOptionalServiceMutationSummary | null {
  if (!details || typeof details !== 'object') return null
  const summary = (details as Record<string, unknown>).summary
  if (!summary || typeof summary !== 'object') return null
  if (typeof (summary as Record<string, unknown>).action !== 'string') {
    return null
  }
  return summary as WorkbenchOptionalServiceMutationSummary
}

function isWorkbenchComposeBlockedCode(code?: string): boolean {
  return (
    code === 'WB-409-LOCKED' ||
    code === 'WB-409-STALE-REVISION' ||
    code === 'WB-409-DRIFT-DETECTED' ||
    code === 'WB-422-VALIDATION'
  )
}

function isWorkbenchComposeBlockedError(
  parsedError: { code?: string; details?: unknown } | null | undefined,
  operation: 'preview' | 'apply',
): boolean {
  if (!parsedError?.code) return false
  if (
    operation === 'preview' &&
    parsedError.code === 'WB-422-VALIDATION' &&
    workbenchIssueHasCode(parsedError.details, 'WB-VAL-EXPECTED-REVISION-MISMATCH')
  ) {
    return true
  }
  return isWorkbenchComposeBlockedCode(parsedError.code)
}

function workbenchComposeErrorGuidance(
  parsedError: { code?: string; details?: unknown } | null,
  operation: 'preview' | 'apply',
): string {
  const code = parsedError?.code
  switch (code) {
    case 'WB-409-LOCKED':
      return 'Another Workbench operation is already active for this project. Wait for it to finish, then retry.'
    case 'WB-409-STALE-REVISION':
      return operation === 'preview'
        ? 'Refresh the Workbench shell to load the latest revision, then generate a new preview.'
        : 'Refresh the Workbench shell to load the latest revision, then preview again before apply.'
    case 'WB-409-DRIFT-DETECTED':
      return 'Re-import the current compose source, then generate a fresh preview before retrying apply.'
    case 'WB-422-VALIDATION':
      if (
        operation === 'preview' &&
        workbenchIssueHasCode(parsedError?.details, 'WB-VAL-EXPECTED-REVISION-MISMATCH')
      ) {
        return 'Refresh the Workbench shell to load the latest revision before generating another preview.'
      }
      return 'Resolve the listed Workbench validation issues in the stored model before retrying.'
    case 'WB-500-STORAGE':
      return 'Retry the operation. If it persists, inspect backend diagnostics because the stored snapshot could not be updated safely.'
    default:
      return ''
  }
}

function workbenchComposeFeedbackFromError(
  parsedError: { code?: string; message: string; details?: unknown } | null,
  operation: 'preview' | 'apply',
): WorkbenchInlineFeedbackState | null {
  if (!parsedError) return null

  const issues = workbenchIssueListFromDetails(parsedError.details)
  const issueMessage = issues[0]?.message?.trim()
  const message = parsedError.code
    ? issueMessage
      ? `[${parsedError.code}] ${parsedError.message} ${issueMessage}`
      : `[${parsedError.code}] ${parsedError.message}`
    : parsedError.message
  const guidance = workbenchComposeErrorGuidance(parsedError, operation)

  return {
    tone: isWorkbenchComposeBlockedError(parsedError, operation) ? 'warn' : 'error',
    message: guidance ? `${message} ${guidance}` : message,
  }
}

function isWorkbenchMutationWarnCode(code?: string): boolean {
  return code === 'WB-409-LOCKED' || code === 'WB-422-VALIDATION'
}

function workbenchMutationErrorGuidance(context: string, code?: string): string {
  switch (code) {
    case 'WB-409-LOCKED':
      return 'Another Workbench operation is already active for this project. Wait for it to finish, then retry.'
    case 'WB-422-VALIDATION':
      return `Update the stored Workbench model to resolve the listed ${context} validation issue, then retry.`
    case 'WB-500-STORAGE':
      return 'Retry the operation. If it persists, inspect backend diagnostics because the Workbench snapshot could not be updated safely.'
    default:
      return ''
  }
}

function workbenchMutationFeedbackFromError(
  parsedError: { code?: string; message: string; details?: unknown } | null,
  context: string,
): WorkbenchInlineFeedbackState | null {
  if (!parsedError) return null

  const issues = workbenchIssueListFromDetails(parsedError.details)
  const issueMessage = issues[0]?.message?.trim()
  const message = parsedError.code
    ? issueMessage
      ? `[${parsedError.code}] ${parsedError.message} ${issueMessage}`
      : `[${parsedError.code}] ${parsedError.message}`
    : parsedError.message
  const guidance = workbenchMutationErrorGuidance(context, parsedError.code)

  return {
    tone: isWorkbenchMutationWarnCode(parsedError.code) ? 'warn' : 'error',
    message: guidance ? `${message} ${guidance}` : message,
  }
}

function isWorkbenchRestoreBlockedCode(code?: string): boolean {
  return (
    code === 'WB-404-BACKUP' ||
    code === 'WB-409-LOCKED' ||
    code === 'WB-409-BACKUP-INTEGRITY'
  )
}

function workbenchRestoreErrorGuidance(code?: string): string {
  switch (code) {
    case 'WB-404-BACKUP':
      return 'Refresh the backup inventory and choose a retained backup target before retrying.'
    case 'WB-409-LOCKED':
      return 'Another Workbench operation is already active for this project. Wait for it to finish, then retry.'
    case 'WB-409-BACKUP-INTEGRITY':
      return 'The selected backup could not be trusted. Inspect the stored backup history on disk before attempting another restore.'
    case 'WB-500-RESTORE':
    case 'WB-500-STORAGE':
      return 'Retry the restore. If it persists, inspect backend diagnostics because the compose artifact could not be replaced safely.'
    default:
      return ''
  }
}

function workbenchRestoreFeedbackFromError(
  parsedError: { code?: string; message: string; details?: unknown } | null,
): WorkbenchInlineFeedbackState | null {
  if (!parsedError) return null

  const message = parsedError.code
    ? `[${parsedError.code}] ${parsedError.message}`
    : parsedError.message
  const guidance = workbenchRestoreErrorGuidance(parsedError.code)

  return {
    tone: isWorkbenchRestoreBlockedCode(parsedError.code) ? 'warn' : 'error',
    message: guidance ? `${message} ${guidance}` : message,
  }
}

function workbenchPortInputValue(port: WorkbenchPortInventoryRow): string {
  return workbenchPortManualInputs.value[port.key] ?? ''
}

function syncWorkbenchPortManualInputs(ports: WorkbenchPortInventoryRow[]) {
  const nextValues: Record<string, string> = {}
  for (const port of ports) {
    nextValues[port.key] =
      workbenchPortManualInputs.value[port.key] ??
      port.effectiveHostPort ??
      port.requestedHostPort ??
      ''
  }
  workbenchPortManualInputs.value = nextValues
}

function setWorkbenchPortInputValue(key: string, value: string) {
  workbenchPortManualInputs.value = {
    ...workbenchPortManualInputs.value,
    [key]: value,
  }
}

function workbenchResourceInputValue(
  resource: WorkbenchResourceInventoryRow,
  field: WorkbenchResourceField,
): string {
  return workbenchResourceInputs.value[resource.key]?.[field] ?? ''
}

function syncWorkbenchResourceInputs(resources: WorkbenchResourceInventoryRow[]) {
  const nextValues: Record<string, WorkbenchResourceInputState> = {}
  for (const resource of resources) {
    nextValues[resource.key] =
      workbenchResourceInputs.value[resource.key] ??
      createWorkbenchResourceInputState(resource)
  }
  workbenchResourceInputs.value = nextValues
}

function syncWorkbenchResourceInputForService(
  serviceName: string,
  resource?: {
    limitCpus?: string | null
    limitMemory?: string | null
    reservationCpus?: string | null
    reservationMemory?: string | null
  } | null,
) {
  workbenchResourceInputs.value = {
    ...workbenchResourceInputs.value,
    [serviceName]: createWorkbenchResourceInputState(resource),
  }
}

function setWorkbenchResourceInputValue(
  serviceName: string,
  field: WorkbenchResourceField,
  value: string,
) {
  workbenchResourceInputs.value = {
    ...workbenchResourceInputs.value,
    [serviceName]: {
      ...createWorkbenchResourceInputState(workbenchResourceInputs.value[serviceName]),
      [field]: value,
    },
  }
}

function resetWorkbenchResourceInputs(resource: WorkbenchResourceInventoryRow) {
  syncWorkbenchResourceInputForService(resource.serviceName, resource)
}

function workbenchPortMutationBusy(port: WorkbenchPortInventoryRow): boolean {
  return (
    workbenchPortMutationStatus.value === 'loading' &&
    workbenchActivePortMutationSelectorKey.value === port.key
  )
}

function workbenchPortSuggestionStatus(port: WorkbenchPortInventoryRow): WorkbenchRequestStatus {
  return workbenchPortSuggestionStatusByKey.value[port.key] ?? 'idle'
}

function workbenchPortMutationFeedback(port: WorkbenchPortInventoryRow): WorkbenchInlineFeedbackState | null {
  const successfulResult = workbenchLastPortMutationResult.value
  if (
    successfulResult &&
    buildWorkbenchPortSelectorKey(successfulResult.selector) === port.key
  ) {
    if (!successfulResult.changed) {
      return {
        tone: 'warn',
        message: successfulResult.message || 'No port mutation changes were required.',
      }
    }

    const label =
      successfulResult.action === 'clear_manual'
        ? `Returned to auto allocation at revision ${successfulResult.revision}.`
        : `Manual host port saved at revision ${successfulResult.revision}.`
    return {
      tone:
        successfulResult.status === 'conflict' || successfulResult.status === 'unavailable'
          ? 'warn'
          : 'ok',
      message: successfulResult.message ? `${label} ${successfulResult.message}` : label,
    }
  }

  const parsedError = workbenchPortMutationError.value
  if (!parsedError) return null

  const summary = workbenchMutationSummaryFromDetails(parsedError.details)
  if (!summary || buildWorkbenchPortSelectorKey(summary.selector) !== port.key) {
    return null
  }

  return workbenchMutationFeedbackFromError(parsedError, 'port')
}

function workbenchPortSuggestionFeedback(port: WorkbenchPortInventoryRow): WorkbenchInlineFeedbackState | null {
  const parsedError = workbenchPortSuggestionErrorByKey.value[port.key]
  if (parsedError) {
    const feedback = workbenchMutationFeedbackFromError(parsedError, 'suggestion')
    if (feedback) return feedback
  }

  const result = workbenchPortSuggestionResultByKey.value[port.key]
  if (!result) return null

  if (result.suggestionCount === 0) {
    return {
      tone: 'warn',
      message: 'No available host-port suggestions were found for this mapping.',
    }
  }

  return {
    tone: 'ok',
    message: `Loaded ${result.suggestionCount} candidate host port${result.suggestionCount === 1 ? '' : 's'} starting at ${result.preferredHostPort ?? 'the current resolver preference'}.`,
  }
}

function workbenchResourceMutationBusy(resource: WorkbenchResourceInventoryRow): boolean {
  return (
    workbenchResourceMutationStatus.value === 'loading' &&
    workbenchActiveResourceMutationServiceName.value === resource.serviceName
  )
}

function workbenchResourceActionDisabled(resource: WorkbenchResourceInventoryRow): boolean {
  return (
    workbenchImportStatus.value === 'loading' ||
    workbenchResolveStatus.value === 'loading' ||
    workbenchPortMutationStatus.value === 'loading' ||
    workbenchResourceMutationStatus.value === 'loading' ||
    workbenchOptionalServiceMutationStatus.value === 'loading' ||
    workbenchPreviewStatus.value === 'loading' ||
    workbenchApplyStatus.value === 'loading' ||
    workbenchRestoreStatus.value === 'loading' ||
    workbenchResourceMutationBusy(resource)
  )
}

function workbenchResourceMutationFeedback(
  resource: WorkbenchResourceInventoryRow,
): WorkbenchInlineFeedbackState | null {
  const successfulResult = workbenchLastResourceMutationResult.value
  if (successfulResult && successfulResult.serviceName === resource.serviceName) {
    if (!successfulResult.changed) {
      return {
        tone: 'warn',
        message: 'No resource mutation changes were required.',
      }
    }

    if (successfulResult.action === 'clear') {
      const cleared = successfulResult.clearedFields.map((field) => workbenchResourceFieldLabels[field])
      return {
        tone: 'ok',
        message: `Cleared ${cleared.join(', ')} at revision ${successfulResult.revision}.`,
      }
    }

    const updated = successfulResult.updatedFields.map((field) => workbenchResourceFieldLabels[field])
    return {
      tone: 'ok',
      message: `Saved ${updated.join(', ')} at revision ${successfulResult.revision}.`,
    }
  }

  const parsedError = workbenchResourceMutationError.value
  if (!parsedError) return null

  const summary = workbenchResourceMutationSummaryFromDetails(parsedError.details)
  if (!summary || summary.selector.serviceName !== resource.serviceName) {
    return null
  }

  return workbenchMutationFeedbackFromError(parsedError, 'resource')
}

function workbenchRemediationActionLabel(action: WorkbenchRemediationAction): string {
  switch (action) {
    case 'refresh':
      return 'Refresh shell'
    case 'import':
      return 'Re-import compose'
    case 'refresh_backups':
      return 'Refresh backups'
  }
}

function workbenchRemediationActionDisabled(action: WorkbenchRemediationAction): boolean {
  switch (action) {
    case 'refresh':
      return workbenchStatus.value === 'loading'
    case 'import':
      return (
        !isAdmin.value ||
        workbenchImportStatus.value === 'loading' ||
        workbenchResolveStatus.value === 'loading' ||
        workbenchPortMutationStatus.value === 'loading' ||
        workbenchResourceMutationStatus.value === 'loading' ||
        workbenchOptionalServiceMutationStatus.value === 'loading' ||
        workbenchPreviewStatus.value === 'loading' ||
        workbenchApplyStatus.value === 'loading' ||
        workbenchRestoreStatus.value === 'loading'
      )
    case 'refresh_backups':
      return (
        !isAdmin.value ||
        workbenchBackupInventoryStatus.value === 'loading' ||
        workbenchRestoreStatus.value === 'loading'
      )
  }
}

function workbenchResourceSetPayload(
  resource: WorkbenchResourceInventoryRow,
): WorkbenchResourceMutationRequest | null {
  const payload: WorkbenchResourceMutationRequest = {
    action: 'set',
  }

  let changedFieldCount = 0
  for (const field of workbenchResourceFieldOrder) {
    const rawValue = workbenchResourceInputValue(resource, field).trim()
    const currentValue = resource[field]?.trim() || ''
    if (!rawValue || rawValue === currentValue) continue
    payload[field] = rawValue
    changedFieldCount += 1
  }

  return changedFieldCount > 0 ? payload : null
}

const statusTone = (status: string): BadgeTone => {
  const normalized = status.trim().toLowerCase()
  if (!normalized) return 'neutral'
  if (normalized === 'running' || normalized === 'up' || normalized.includes('running')) return 'ok'
  if (normalized.includes('failed') || normalized.includes('error')) return 'error'
  if (normalized.includes('pending') || normalized.includes('building')) return 'warn'
  return 'neutral'
}

const fmtDate = (value?: string | null) => {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

const refreshWorkbenchReadState = async (selection: {
  snapshot?: boolean
  catalog?: boolean
  backups?: boolean
} = {}) => {
  const name = projectName.value
  if (!name || !workbenchComposeSupported.value) return
  await refetchWorkbenchReadQueries(queryClient, name, selection)
}

const load = async () => {
  const name = projectName.value
  if (!name) {
    error.value = 'Invalid project name.'
    detail.value = null
    workbenchStore.reset()
    return
  }

  loading.value = true
  error.value = null
  pageLoading.start(`Loading project ${name}...`)
  try {
    const { data } = await projectsApi.getDetail(name)
    detail.value = data
    if (!workbenchComposeSupported.value) {
      workbenchStore.reset()
    } else {
      void refreshWorkbenchReadState({
        snapshot: true,
        catalog: true,
        backups: isAdmin.value,
      })
    }
  } catch (err) {
    detail.value = null
    error.value = apiErrorMessage(err)
    workbenchStore.reset()
  } finally {
    loading.value = false
    pageLoading.stop()
  }
}

const refreshWorkbench = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value) return
  workbenchPendingOptionalServiceMutation.value = null
  await refreshWorkbenchReadState({
    snapshot: true,
    catalog: true,
    backups: isAdmin.value,
  })
}

const importWorkbench = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value) {
    toastStore.warn('Workbench import is only available for compose-backed projects.', 'Workbench')
    return
  }
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench import blocked')
    return
  }

  const result = await workbenchStore.runImport(name, 'manual')
  if (!result) {
    const parsedError = workbenchImportError.value
    toastStore.error(parsedError?.message ?? 'Workbench import failed.', 'Workbench')
    return
  }

  if (result.changed) {
    toastStore.success(`Workbench snapshot imported (revision ${result.revision}).`, 'Workbench')
  } else {
    toastStore.warn(
      `Workbench snapshot already matches the current compose (revision ${result.revision}).`,
      'Workbench',
    )
  }
}

const queueWorkbenchOptionalServiceMutation = (
  entry: WorkbenchOptionalServiceCatalogRow,
  action: WorkbenchOptionalServiceMutationAction = workbenchOptionalServicePendingAction(entry),
) => {
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench optional services')
    return
  }
  if (workbenchOptionalServiceActionDisabled(entry)) return

  const serviceName = workbenchOptionalServiceManagedServiceName(entry) ?? entry.defaultServiceName
  if (action === 'remove' && !serviceName.trim()) {
    toastStore.error('No catalog-managed service is available to remove for this entry.', 'Workbench optional services')
    return
  }

  workbenchPendingOptionalServiceMutation.value = {
    entryKey: entry.key,
    action,
    serviceName,
    displayName: entry.displayName,
  }
}

const cancelWorkbenchOptionalServiceMutation = (entryKey?: string) => {
  if (!entryKey || workbenchPendingOptionalServiceMutation.value?.entryKey === entryKey) {
    workbenchPendingOptionalServiceMutation.value = null
  }
}

const confirmWorkbenchOptionalServiceMutation = async (entry: WorkbenchOptionalServiceCatalogRow) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench optional services')
    return
  }

  const pendingMutation = workbenchPendingOptionalServiceMutation.value
  if (!pendingMutation || pendingMutation.entryKey !== entry.key) return

  const actionLabel = pendingMutation.action === 'remove' ? 'remove' : 'add'
  workbenchPendingOptionalServiceMutation.value = null

  const result =
    pendingMutation.action === 'remove'
      ? await workbenchStore.removeOptionalService(name, entry.key, pendingMutation.serviceName)
      : await workbenchStore.addOptionalService(name, entry.key)

  if (!result) {
    const parsedError = workbenchOptionalServiceMutationError.value
    if (parsedError?.code === 'WB-409-LOCKED' || parsedError?.code === 'WB-422-VALIDATION') {
      toastStore.warn(
        parsedError?.message ?? `Workbench optional-service ${actionLabel} was blocked.`,
        'Workbench optional services',
      )
    } else {
      toastStore.error(
        parsedError?.message ?? `Workbench optional-service ${actionLabel} failed.`,
        'Workbench optional services',
      )
    }
    return
  }

  toastStore.success(
    pendingMutation.action === 'remove'
      ? `Removed ${result.serviceName || pendingMutation.displayName} from the stored Workbench snapshot.`
      : `Added ${result.serviceName || pendingMutation.displayName} to the stored Workbench snapshot.`,
    'Workbench optional services',
  )
}

const resolveWorkbenchPorts = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value || !workbenchSnapshotReady.value) {
    toastStore.warn('Import a Workbench snapshot before resolving ports.', 'Workbench')
    return
  }
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench resolve blocked')
    return
  }

  const result = await workbenchStore.resolvePorts(name)
  if (!result) {
    const parsedError = workbenchResolveError.value
    toastStore.error(parsedError?.message ?? 'Workbench port resolution failed.', 'Workbench')
    return
  }

  if (result.changed) {
    toastStore.success(`Workbench ports resolved at revision ${result.revision}.`, 'Workbench')
  } else {
    toastStore.warn(`Workbench ports already matched the resolver output at revision ${result.revision}.`, 'Workbench')
  }
}

const setManualWorkbenchPort = async (port: WorkbenchPortInventoryRow) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench edit blocked')
    return
  }

  const rawValue = workbenchPortInputValue(port).trim()
  const manualHostPort = Number(rawValue)
  if (!rawValue || Number.isNaN(manualHostPort) || !Number.isInteger(manualHostPort)) {
    toastStore.error('Enter a valid integer host port.', 'Workbench port')
    return
  }

  const result = await workbenchStore.mutatePort(name, {
    selector: port.selector,
    action: 'set_manual',
    manualHostPort,
  })
  if (!result) {
    const parsedError = workbenchPortMutationError.value
    toastStore.error(parsedError?.message ?? 'Workbench port update failed.', 'Workbench')
    return
  }

  setWorkbenchPortInputValue(port.key, String(result.assignedHostPort ?? manualHostPort))
  toastStore.success(`Manual host port saved for ${port.serviceName}.`, 'Workbench port')
}

const resetWorkbenchPortToAuto = async (port: WorkbenchPortInventoryRow) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench edit blocked')
    return
  }

  const result = await workbenchStore.mutatePort(name, {
    selector: port.selector,
    action: 'clear_manual',
  })
  if (!result) {
    const parsedError = workbenchPortMutationError.value
    toastStore.error(parsedError?.message ?? 'Workbench auto-reset failed.', 'Workbench')
    return
  }

  setWorkbenchPortInputValue(port.key, String(result.assignedHostPort ?? result.preferredHostPort ?? ''))
  toastStore.success(`Auto allocation restored for ${port.serviceName}.`, 'Workbench port')
}

const loadWorkbenchPortSuggestions = async (port: WorkbenchPortInventoryRow) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench suggestions blocked')
    return
  }

  const result = await workbenchStore.loadPortSuggestions(name, port.selector, 5)
  if (!result) {
    const parsedError = workbenchPortSuggestionErrorByKey.value[port.key]
    toastStore.error(parsedError?.message ?? 'Workbench suggestions failed.', 'Workbench')
    return
  }

  if (result.suggestionCount === 0) {
    toastStore.warn(`No open host-port suggestions found for ${port.serviceName}.`, 'Workbench')
    return
  }

  toastStore.success(`Loaded ${result.suggestionCount} host-port suggestion(s) for ${port.serviceName}.`, 'Workbench')
}

const saveWorkbenchResource = async (resource: WorkbenchResourceInventoryRow) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench edit blocked')
    return
  }

  const payload = workbenchResourceSetPayload(resource)
  if (!payload) {
    toastStore.warn(
      'Enter at least one changed CPU or memory value before saving. Use clear on an existing field to remove it.',
      'Workbench resources',
    )
    return
  }

  const result = await workbenchStore.mutateResource(name, resource.serviceName, payload)
  if (!result) {
    const parsedError = workbenchResourceMutationError.value
    toastStore.error(parsedError?.message ?? 'Workbench resource update failed.', 'Workbench')
    return
  }

  syncWorkbenchResourceInputForService(resource.serviceName, result.currentResource)
  toastStore.success(`Stored resources updated for ${resource.serviceName}.`, 'Workbench resources')
}

const clearWorkbenchResourceFields = async (
  resource: WorkbenchResourceInventoryRow,
  fields: WorkbenchResourceField[],
) => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench edit blocked')
    return
  }

  const clearFields = fields.filter((field) => Boolean(resource[field]))
  if (clearFields.length === 0) {
    toastStore.warn('No stored CPU or memory values are available to clear on this service.', 'Workbench resources')
    return
  }

  const result = await workbenchStore.mutateResource(name, resource.serviceName, {
    action: 'clear',
    clearFields,
  })
  if (!result) {
    const parsedError = workbenchResourceMutationError.value
    toastStore.error(parsedError?.message ?? 'Workbench resource clear failed.', 'Workbench')
    return
  }

  syncWorkbenchResourceInputForService(resource.serviceName, result.currentResource)
  toastStore.success(`Stored resources cleared for ${resource.serviceName}.`, 'Workbench resources')
}

const previewWorkbenchCompose = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value || !workbenchSnapshotReady.value) {
    toastStore.warn('Import a Workbench snapshot before generating a compose preview.', 'Workbench preview')
    return
  }
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench preview blocked')
    return
  }
  if (workbenchComposeImportRequired.value) {
    toastStore.warn(
      'Re-import the restored compose before generating a new preview.',
      'Workbench preview',
    )
    return
  }

  const result = await workbenchStore.previewCompose(name, {
    expectedRevision: workbenchSnapshot.value?.revision,
  })
  if (!result) {
    const parsedError = workbenchPreviewError.value
    if (isWorkbenchComposeBlockedError(parsedError, 'preview')) {
      toastStore.warn(parsedError?.message ?? 'Workbench compose preview was blocked.', 'Workbench preview')
    } else {
      toastStore.error(parsedError?.message ?? 'Workbench compose preview failed.', 'Workbench preview')
    }
    return
  }

  toastStore.success(`Compose preview generated from revision ${result.revision}.`, 'Workbench preview')
}

const applyWorkbenchCompose = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value || !workbenchSnapshotReady.value) {
    toastStore.warn('Import a Workbench snapshot before applying compose changes.', 'Workbench apply')
    return
  }
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench apply blocked')
    return
  }
  if (workbenchComposeImportRequired.value) {
    toastStore.warn(
      'Re-import the restored compose before preview/apply.',
      'Workbench apply',
    )
    return
  }
  if (!workbenchHasCleanPreview.value) {
    toastStore.warn(
      'Generate a clean compose preview from the current snapshot before apply.',
      'Workbench apply',
    )
    return
  }

  const result = await workbenchStore.applyCompose(name, {
    expectedRevision: workbenchSnapshot.value?.revision,
    expectedSourceFingerprint: workbenchSnapshot.value?.sourceFingerprint || undefined,
  })
  if (!result) {
    const parsedError = workbenchApplyError.value
    if (isWorkbenchComposeBlockedError(parsedError, 'apply')) {
      toastStore.warn(parsedError?.message ?? 'Workbench compose apply was blocked.', 'Workbench apply')
    } else {
      toastStore.error(parsedError?.message ?? 'Workbench compose apply failed.', 'Workbench apply')
    }
    return
  }

  toastStore.success(`Compose applied. Backup ${result.backupId} was recorded.`, 'Workbench apply')
}

const refreshWorkbenchComposeBackups = async () => {
  if (!isAdmin.value) return
  await refreshWorkbenchReadState({
    snapshot: false,
    catalog: false,
    backups: true,
  })
}

const restoreWorkbenchCompose = async () => {
  const name = projectName.value
  if (!name) return
  if (!workbenchComposeSupported.value || !workbenchSnapshotReady.value) {
    toastStore.warn('Import a Workbench snapshot before restoring compose backups.', 'Workbench restore')
    return
  }
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Workbench restore blocked')
    return
  }

  const selectedBackup = workbenchSelectedComposeBackup.value
  if (!selectedBackup) {
    toastStore.warn('Choose a retained compose backup before restoring.', 'Workbench restore')
    return
  }
  if (workbenchRestoreConfirmInput.value.trim() !== workbenchRestoreConfirmationPhrase.value) {
    toastStore.warn('Type the restore confirmation phrase exactly before continuing.', 'Workbench restore')
    return
  }

  const result = await workbenchStore.restoreCompose(name, {
    backupId: selectedBackup.backupId,
  })
  if (!result) {
    const parsedError = workbenchRestoreError.value
    if (isWorkbenchRestoreBlockedCode(parsedError?.code)) {
      toastStore.warn(parsedError?.message ?? 'Workbench compose restore was blocked.', 'Workbench restore')
    } else {
      toastStore.error(parsedError?.message ?? 'Workbench compose restore failed.', 'Workbench restore')
    }
    return
  }

  workbenchRestoreConfirmInput.value = ''
  if (result.requiresImport) {
    toastStore.warn(
      `Backup ${result.backupId} restored. Re-import the restored compose before preview/apply.`,
      'Workbench restore',
    )
    return
  }

  toastStore.success(`Backup ${result.backupId} restored to ${result.composePath}.`, 'Workbench restore')
}

const runWorkbenchRemediationAction = async (action?: WorkbenchRemediationAction) => {
  if (!action) return

  switch (action) {
    case 'refresh':
      await refreshWorkbench()
      return
    case 'import':
      await importWorkbench()
      return
    case 'refresh_backups':
      await refreshWorkbenchComposeBackups()
      return
  }
}

const copyTextToClipboard = async (payload: string) => {
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(payload)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = payload
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

const copyWorkbenchPreviewCompose = async () => {
  const compose = workbenchLastPreviewResult.value?.compose ?? ''
  if (!compose) {
    toastStore.warn('Generate a compose preview before copying it.', 'Workbench preview')
    return
  }

  try {
    await copyTextToClipboard(compose)
    toastStore.success('Compose preview copied to clipboard.', 'Workbench preview')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const restartStack = async () => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Restart blocked')
    return
  }
  if (stackRestarting.value) return

  stackRestartError.value = null
  stackRestarting.value = true
  try {
    const { data } = await projectsApi.restartStack(name)
    toastStore.success(`Project "${name}" restart queued (job #${data.job.id}).`, 'Docker compose')
  } catch (err) {
    const message = apiErrorMessage(err)
    stackRestartError.value = message
    toastStore.error(message, 'Queue failed')
  } finally {
    stackRestarting.value = false
  }
}

watch(workbenchPortInventory, (ports) => {
  syncWorkbenchPortManualInputs(ports)
}, { immediate: true })

watch(workbenchResourceInventory, (resources) => {
  syncWorkbenchResourceInputs(resources)
}, { immediate: true })

watch(workbenchServiceInventory, (services) => {
  const selectedServiceName = workbenchSelectedServiceName.value.trim().toLowerCase()
  if (services.length === 0) {
    workbenchSelectedServiceName.value = ''
    return
  }

  if (
    selectedServiceName &&
    services.some((service) => service.serviceName.trim().toLowerCase() === selectedServiceName)
  ) {
    return
  }

  workbenchSelectedServiceName.value = services[0]?.serviceName ?? ''
}, { immediate: true })

watch(workbenchComposeBackups, (backups) => {
  const selectedBackupId = workbenchRestoreSelectedBackupId.value.trim().toLowerCase()
  if (selectedBackupId && backups.some((backup) => backup.backupId === selectedBackupId)) {
    return
  }

  const latestBackup = backups.length > 0 ? backups[backups.length - 1] : null
  workbenchRestoreSelectedBackupId.value = latestBackup?.backupId ?? ''
  workbenchRestoreConfirmInput.value = ''
}, { immediate: true })

watch(workbenchRestoreSelectedBackupId, () => {
  workbenchRestoreConfirmInput.value = ''
})

onMounted(load)
watch(projectName, () => {
  activeSectionTab.value = 'workbench'
  stackRestartError.value = null
  workbenchRestoreSelectedBackupId.value = ''
  workbenchRestoreConfirmInput.value = ''
  workbenchPortManualInputs.value = {}
  workbenchResourceInputs.value = {}
  workbenchSelectedServiceName.value = ''
  workbenchStore.reset()
  void load()
})
</script>

<template>
  <section class="page">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div class="mb-4">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project workspace</p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">{{ projectName || 'Project detail' }}</h1>
      </div>
      <div class="flex items-center gap-2">
        <RouterLink to="/projects" class="btn btn-ghost px-3 py-2 text-xs font-semibold">
          <span class="inline-flex items-center gap-2">
            <NavIcon name="arrow-left" class="h-3.5 w-3.5" />
            Back
          </span>
        </RouterLink>
        <UiButton variant="ghost" size="sm" @click="load">
          <span class="inline-flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh
          </span>
        </UiButton>
      </div>
    </header>

    <UiState v-if="loading" :loading="true">Loading project detail...</UiState>
    <UiState v-else-if="error" tone="error">{{ error }}</UiState>

    <template v-else-if="detail">
      <UiPanel
        variant="soft"
        class="flex flex-row justify-between items-start gap-4 p-4 mb-4 flex-wrap"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Workspace guidance</p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            Read access is available to all authenticated users. Restart actions require admin permissions.
          </p>
        </div>
        <div class="flex flex-row gap-2 items-center">
        <UiButton
              variant="ghost"
              size="sm"
              :disabled="stackRestarting || !isAdmin"
              @click="restartStack"
            >
              <span class="inline-flex items-center gap-2">
                <NavIcon name="restart" class="h-3.5 w-3.5" />
                <UiInlineSpinner v-if="stackRestarting" />
                {{ stackRestarting ? 'Restarting stack...' : 'Restart stack' }}
              </span>
            </UiButton>
        <UiBadge :tone="statusTone(detail.project.record?.status || '')">
          {{ detail.project.record?.status || 'unknown' }}
        </UiBadge>

        </div>
      </UiPanel>
      <UiInlineFeedback v-if="stackRestartError" tone="error">
        {{ stackRestartError }}
      </UiInlineFeedback>

      <hr />

      <div class="flex flex-col gap-2">
        <UiPanel class="flex p-2 mb-2">
          <div class="flex flex-row flex-wrap items-start gap-4 sm:gap-6">
            <div class="p-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Last updated</p>
              <p class="text-sm text-[color:var(--muted)]">{{ fmtDate(detail.project.record?.updatedAt) }}</p>
            </div>
            <div class="p-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Source</p>
              <p class="text-sm text-[color:var(--text)]">{{ detail.runtime.source || 'unknown' }}</p>
            </div>
            <div class="p-2 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Path</p>
              <p class="font-mono text-xs text-[color:var(--muted)] break-all">{{ detail.runtime.path }}</p>
            </div>
            <div class="p-2 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">.env File</p>
              <p class="font-mono text-xs text-[color:var(--muted)] break-all">{{ detail.runtime.envPath }}</p>
            </div>
            <div class="p-2 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Repository</p>
              <p class="text-sm text-[color:var(--muted)] break-all">
                {{ detail.project.record?.repoUrl || 'No repository URL recorded' }}
              </p>
            </div>

          </div>
        </UiPanel>
      </div>

      <ProjectSectionTabs
        v-model="activeSectionTab"
        class="mb-4"
      />

      <UiPanel
        v-if="activeSectionTab === 'workbench'"
        class="space-y-5 p-6"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Workbench</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Read/import state for the stored Workbench model plus admin-only optional-service controls, a service-first inspector, and the existing compose preview/apply workflow.
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <UiBadge :tone="workbenchStatusTone">
              {{ workbenchStatusLabel }}
            </UiBadge>
            <UiBadge :tone="workbenchAccessTone">
              {{ workbenchAccessLabel }}
            </UiBadge>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <template v-if="workbenchComposeSupported">
            <UiButton
              variant="ghost"
              size="sm"
              :disabled="
                workbenchStatus === 'loading' ||
                workbenchCatalogStatus === 'loading' ||
                workbenchImportStatus === 'loading' ||
                workbenchResolveStatus === 'loading' ||
                workbenchResourceMutationStatus === 'loading' ||
                workbenchOptionalServiceMutationStatus === 'loading' ||
                workbenchPreviewStatus === 'loading' ||
                workbenchApplyStatus === 'loading' ||
                workbenchRestoreStatus === 'loading'
              "
              @click="refreshWorkbench"
            >
              <span class="inline-flex items-center gap-2">
                <NavIcon name="refresh" class="h-3.5 w-3.5" />
                <UiInlineSpinner v-if="workbenchStatus === 'loading'" />
                Refresh shell
              </span>
            </UiButton>
            <UiButton
              v-if="isAdmin && workbenchSnapshotReady"
              variant="ghost"
              size="sm"
              :disabled="
                workbenchResolveStatus === 'loading' ||
                workbenchCatalogStatus === 'loading' ||
                workbenchPortMutationStatus === 'loading' ||
                workbenchResourceMutationStatus === 'loading' ||
                workbenchOptionalServiceMutationStatus === 'loading' ||
                workbenchPreviewStatus === 'loading' ||
                workbenchApplyStatus === 'loading' ||
                workbenchRestoreStatus === 'loading'
              "
              @click="resolveWorkbenchPorts"
            >
              <span class="inline-flex items-center gap-2">
                <UiInlineSpinner v-if="workbenchResolveStatus === 'loading'" />
                {{ workbenchResolveLabel }}
              </span>
            </UiButton>
            <UiButton
              v-if="isAdmin"
              variant="primary"
              size="sm"
              :disabled="
                workbenchImportStatus === 'loading' ||
                workbenchCatalogStatus === 'loading' ||
                workbenchResolveStatus === 'loading' ||
                workbenchPortMutationStatus === 'loading' ||
                workbenchResourceMutationStatus === 'loading' ||
                workbenchOptionalServiceMutationStatus === 'loading' ||
                workbenchPreviewStatus === 'loading' ||
                workbenchApplyStatus === 'loading' ||
                workbenchRestoreStatus === 'loading'
              "
              @click="importWorkbench"
            >
              <span class="inline-flex items-center gap-2">
                <UiInlineSpinner v-if="workbenchImportStatus === 'loading'" />
                {{ workbenchImportLabel }}
              </span>
            </UiButton>
          </template>
        </div>

        <UiInlineFeedback
          v-if="workbenchImportFeedbackState"
          :tone="workbenchImportFeedbackState.tone"
        >
          {{ workbenchImportFeedbackState.message }}
        </UiInlineFeedback>
        <UiInlineFeedback
          v-if="workbenchResolveFeedback"
          :tone="workbenchResolveFeedback.tone"
        >
          {{ workbenchResolveFeedback.message }}
        </UiInlineFeedback>

        <UiState v-if="workbenchStatus === 'loading'" loading>
          Loading Workbench snapshot...
        </UiState>
        <UiState v-else-if="workbenchStatus === 'error'" tone="error">
          {{ workbenchErrorMessage }}
        </UiState>
        <UiState v-else-if="!workbenchComposeSupported">
          This project does not expose any compose source files, so the Workbench shell and import flow are unavailable here.
        </UiState>
        <template v-else-if="workbenchSnapshot">
          <UiInlineFeedback
            v-if="!workbenchSnapshotReady"
            :tone="isAdmin ? 'warn' : 'neutral'"
          >
            {{
              isAdmin
                ? 'No imported Workbench snapshot is stored for this project yet. Import the current compose to initialize the model shell. The catalog below still shows available catalog-managed services and any legacy transition metadata.'
                : 'No imported Workbench snapshot is stored for this project yet. An admin must import the project compose before current-compose inventory becomes visible here. The catalog below stays read-only while still showing available services and legacy transition metadata.'
            }}
          </UiInlineFeedback>

          <template v-if="workbenchSnapshotReady">
          <div class="workbench-shell">

          <WorkbenchCatalogControlsPanel
            :is-admin="isAdmin"
            :compose-path="workbenchSnapshot?.composePath || 'No compose path recorded'"
            :fingerprint-label="workbenchFingerprintLabel"
            :current-compose-summary="workbenchCurrentComposeSummary"
            :optional-service-inventory="workbenchOptionalServiceInventory"
            :catalog-status="workbenchCatalogStatus"
            :catalog-error-message="workbenchCatalogErrorMessage"
            :pending-optional-service-mutation="workbenchPendingOptionalServiceMutation"
            :optional-service-pending-confirmation="workbenchOptionalServicePendingConfirmation"
            :optional-service-pending-action="workbenchOptionalServicePendingAction"
            :optional-service-action-disabled="workbenchOptionalServiceActionDisabled"
            :queue-optional-service-mutation="queueWorkbenchOptionalServiceMutation"
            :optional-service-busy="workbenchOptionalServiceBusy"
            :optional-service-pending-label="workbenchOptionalServicePendingLabel"
            :optional-service-feedback="workbenchOptionalServiceFeedback"
            :confirm-optional-service-mutation="confirmWorkbenchOptionalServiceMutation"
            :cancel-optional-service-mutation="cancelWorkbenchOptionalServiceMutation"
          />

          <WorkbenchServiceInspectorPanel
            :is-admin="isAdmin"
            :optional-service-mutation-status="workbenchOptionalServiceMutationStatus"
            :preview-status="workbenchPreviewStatus"
            :apply-status="workbenchApplyStatus"
            :restore-status="workbenchRestoreStatus"
            :resolve-status="workbenchResolveStatus"
            :service-inventory="workbenchServiceInventory"
            :selected-service="workbenchSelectedService"
            :selected-service-topology="workbenchSelectedServiceTopology"
            :selected-service-ports="workbenchSelectedServicePorts"
            :selected-service-resource="workbenchSelectedServiceResource"
            :resource-editor-fields="workbenchResourceEditorFields"
            :port-suggestion-result-by-key="workbenchPortSuggestionResultByKey"
            :select-service="(serviceName) => (workbenchSelectedServiceName = serviceName)"
            :port-input-value="workbenchPortInputValue"
            :set-port-input-value="setWorkbenchPortInputValue"
            :port-mutation-busy="workbenchPortMutationBusy"
            :set-manual-port="setManualWorkbenchPort"
            :reset-port-to-auto="resetWorkbenchPortToAuto"
            :port-suggestion-status="workbenchPortSuggestionStatus"
            :load-port-suggestions="loadWorkbenchPortSuggestions"
            :port-mutation-feedback="workbenchPortMutationFeedback"
            :port-suggestion-feedback="workbenchPortSuggestionFeedback"
            :resource-input-value="workbenchResourceInputValue"
            :set-resource-input-value="setWorkbenchResourceInputValue"
            :resource-action-disabled="workbenchResourceActionDisabled"
            :clear-resource-fields="clearWorkbenchResourceFields"
            :save-resource="saveWorkbenchResource"
            :resource-mutation-busy="workbenchResourceMutationBusy"
            :reset-resource-inputs="resetWorkbenchResourceInputs"
            :resource-mutation-feedback="workbenchResourceMutationFeedback"
          />

          <div class="workbench-shell-grid grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel
              variant="soft"
              class="workbench-shell-card workbench-shell-card--secondary space-y-4 p-4 text-sm text-[color:var(--muted)]"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose workflow</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Preview and apply</h3>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <UiBadge :tone="workbenchPreviewStatusTone">
                    {{ workbenchPreviewStatusLabel }}
                  </UiBadge>
                  <UiBadge :tone="workbenchHasCleanPreview ? 'ok' : workbenchComposeImportRequired ? 'warn' : 'neutral'">
                    {{
                      workbenchHasCleanPreview
                        ? 'Apply ready'
                        : workbenchComposeImportRequired
                          ? 'Import required'
                          : 'Preview required'
                    }}
                  </UiBadge>
                  <UiBadge :tone="workbenchComposeIssueInventory.length > 0 ? 'warn' : 'ok'">
                    {{ workbenchComposeIssueInventory.length }} blockers
                  </UiBadge>
                </div>
              </div>

              <p class="text-sm">
                Selected-service port edits, budget changes, and catalog-managed service mutations all feed this one compose path. Generate a fresh preview from the stored snapshot, review blockers and non-blocking warnings here, then apply the exact artifact while the snapshot revision and fingerprint still match.
              </p>

              <div v-if="isAdmin" class="flex flex-wrap gap-2">
                <UiButton
                  variant="ghost"
                  size="sm"
                  class="w-full justify-center sm:w-auto"
                  :disabled="
                    workbenchComposeActionBusy ||
                    workbenchComposeImportRequired ||
                    workbenchImportStatus === 'loading' ||
                    workbenchResolveStatus === 'loading' ||
                    workbenchPortMutationStatus === 'loading' ||
                    workbenchResourceMutationStatus === 'loading' ||
                    workbenchOptionalServiceMutationStatus === 'loading'
                  "
                  @click="previewWorkbenchCompose"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="workbenchPreviewStatus === 'loading'" />
                    {{ workbenchPreviewLabel }}
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  class="w-full justify-center sm:w-auto"
                  :disabled="!workbenchLastPreviewResult?.compose || workbenchComposeActionBusy"
                  @click="copyWorkbenchPreviewCompose"
                >
                  Copy preview
                </UiButton>
                <UiButton
                  variant="primary"
                  size="sm"
                  class="w-full justify-center sm:w-auto"
                  :disabled="workbenchApplyActionDisabled"
                  @click="applyWorkbenchCompose"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="workbenchApplyStatus === 'loading'" />
                    {{ workbenchApplyLabel }}
                  </span>
                </UiButton>
              </div>
              <p v-else class="text-xs text-[color:var(--muted)]">
                Read-only access: admin permissions are required to generate compose preview output and apply stored Workbench changes.
              </p>

              <UiInlineFeedback
                v-if="workbenchPreviewFeedback"
                :tone="workbenchPreviewFeedback.tone"
              >
                {{ workbenchPreviewFeedback.message }}
              </UiInlineFeedback>
              <UiInlineFeedback
                v-if="workbenchApplyFeedback"
                :tone="workbenchApplyFeedback.tone"
              >
                {{ workbenchApplyFeedback.message }}
              </UiInlineFeedback>
              <UiPanel
                v-if="workbenchComposeRemediationState"
                variant="soft"
                class="space-y-3 border border-[color:var(--line)] p-3"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      {{ workbenchComposeRemediationState.sourceLabel }}
                    </p>
                    <h4 class="mt-1 font-semibold text-[color:var(--text)]">
                      {{ workbenchComposeRemediationState.title }}
                    </h4>
                  </div>
                  <UiBadge :tone="workbenchComposeRemediationState.tone">
                    Needs attention
                  </UiBadge>
                </div>
                <p>{{ workbenchComposeRemediationState.message }}</p>

                <div
                  v-if="
                    workbenchComposeRemediationState.details?.expectedRevision != null ||
                    workbenchComposeRemediationState.details?.revision != null ||
                    workbenchComposeRemediationState.details?.sourceFingerprint ||
                    workbenchComposeRemediationState.details?.currentSourceFingerprint
                  "
                  class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4"
                >
                  <UiPanel
                    v-if="workbenchComposeRemediationState.details?.expectedRevision != null"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      Expected revision
                    </p>
                    <p class="text-sm font-semibold text-[color:var(--text)]">
                      {{ workbenchComposeRemediationState.details?.expectedRevision }}
                    </p>
                  </UiPanel>
                  <UiPanel
                    v-if="workbenchComposeRemediationState.details?.revision != null"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      Current revision
                    </p>
                    <p class="text-sm font-semibold text-[color:var(--text)]">
                      {{ workbenchComposeRemediationState.details?.revision }}
                    </p>
                  </UiPanel>
                  <UiPanel
                    v-if="workbenchComposeRemediationState.details?.sourceFingerprint"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      Stored fingerprint
                    </p>
                    <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                      {{ workbenchComposeRemediationState.details?.sourceFingerprint }}
                    </p>
                  </UiPanel>
                  <UiPanel
                    v-if="workbenchComposeRemediationState.details?.currentSourceFingerprint"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      On-disk fingerprint
                    </p>
                    <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                      {{ workbenchComposeRemediationState.details?.currentSourceFingerprint }}
                    </p>
                  </UiPanel>
                </div>
                <div
                  v-if="
                    isAdmin &&
                    (workbenchComposeRemediationState.primaryAction ||
                      workbenchComposeRemediationState.secondaryAction)
                  "
                  class="flex flex-wrap gap-2"
                >
                  <UiButton
                    v-if="workbenchComposeRemediationState.primaryAction"
                    variant="primary"
                    size="sm"
                    :disabled="
                      workbenchRemediationActionDisabled(
                        workbenchComposeRemediationState.primaryAction,
                      )
                    "
                    @click="
                      runWorkbenchRemediationAction(workbenchComposeRemediationState.primaryAction)
                    "
                  >
                    {{
                      workbenchRemediationActionLabel(
                        workbenchComposeRemediationState.primaryAction,
                      )
                    }}
                  </UiButton>
                  <UiButton
                    v-if="workbenchComposeRemediationState.secondaryAction"
                    variant="ghost"
                    size="sm"
                    :disabled="
                      workbenchRemediationActionDisabled(
                        workbenchComposeRemediationState.secondaryAction,
                      )
                    "
                    @click="
                      runWorkbenchRemediationAction(workbenchComposeRemediationState.secondaryAction)
                    "
                  >
                    {{
                      workbenchRemediationActionLabel(
                        workbenchComposeRemediationState.secondaryAction,
                      )
                    }}
                  </UiButton>
                </div>
              </UiPanel>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Current revision</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.revision }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Preview revision</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchLastPreviewResult?.revision ?? '—' }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose lines</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchPreviewComposeLineCount }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Snapshot match</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchPreviewMatchesSnapshot ? 'Yes' : 'No' }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Import warnings</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchWarningsList.length }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Blocking diagnostics</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchComposeIssueInventory.length }}
                  </p>
                </UiPanel>
              </div>

              <div class="grid gap-3 lg:grid-cols-2">
                <UiPanel variant="raise" class="space-y-2 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Current fingerprint</p>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                    {{ workbenchFingerprintLabel }}
                  </p>
                </UiPanel>
                <UiPanel variant="raise" class="space-y-2 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Preview fingerprint</p>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                    {{ workbenchPreviewFingerprintLabel }}
                  </p>
                </UiPanel>
              </div>

              <UiState v-if="isAdmin && workbenchComposeImportRequired" tone="warn">
                Restore changed the compose file on disk. Re-import the restored compose before preview/apply so the stored Workbench snapshot matches the file again.
              </UiState>
              <UiState v-else-if="isAdmin && !workbenchHasCleanPreview" tone="warn">
                Run a clean preview from the current snapshot before apply. Validation blockers, stale revisions, compose drift, or any stored model change will keep apply disabled.
              </UiState>
              <UiState v-else-if="isAdmin" tone="ok">
                The latest preview matches the visible snapshot revision and fingerprint, so apply is enabled.
              </UiState>
              <p v-else class="text-xs text-[color:var(--muted)]">
                Non-admin users can inspect stored warnings and diagnostics here, but preview/apply stays admin-only.
              </p>

              <div class="grid gap-3 lg:grid-cols-2">
                <div class="space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Blocking diagnostics</p>
                    <UiBadge :tone="workbenchComposeIssueInventory.length > 0 ? 'warn' : 'ok'">
                      {{ workbenchComposeIssueInventory.length }}
                    </UiBadge>
                  </div>
                  <UiState v-if="workbenchComposeIssueInventory.length === 0" tone="ok">
                    No preview/apply blockers are active right now.
                  </UiState>
                  <div v-else class="space-y-2">
                    <UiPanel
                      v-for="row in workbenchComposeIssueInventory"
                      :key="row.key"
                      variant="soft"
                      class="space-y-2 p-3"
                    >
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <div class="flex flex-wrap items-center gap-2">
                          <UiBadge :tone="row.source === 'preview' ? 'warn' : 'error'">
                            {{ row.sourceLabel }}
                          </UiBadge>
                          <UiBadge tone="warn">{{ row.issue.code }}</UiBadge>
                        </div>
                        <span class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                          {{ row.issue.path || 'compose' }}
                        </span>
                      </div>
                      <p class="text-xs text-[color:var(--text)]">{{ row.issue.message }}</p>
                    </UiPanel>
                  </div>
                </div>

                <div class="space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Snapshot warnings</p>
                    <UiBadge :tone="workbenchWarningsList.length > 0 ? 'warn' : 'ok'">
                      {{ workbenchWarningsList.length }}
                    </UiBadge>
                  </div>
                  <UiState v-if="workbenchWarningsList.length === 0" tone="ok">
                    No non-blocking import or pass-through warnings are recorded.
                  </UiState>
                  <div v-else class="max-h-[18rem] space-y-2 overflow-auto pr-1">
                    <UiPanel
                      v-for="warning in workbenchWarningsList"
                      :key="`compose-warning-${warning.code}-${warning.path}-${warning.message}`"
                      variant="soft"
                      class="space-y-2 p-3"
                    >
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <UiBadge tone="warn">{{ warning.code }}</UiBadge>
                        <span class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                          {{ warning.path || 'compose' }}
                        </span>
                      </div>
                      <p class="text-xs text-[color:var(--text)]">{{ warning.message }}</p>
                    </UiPanel>
                  </div>
                </div>
              </div>

              <UiState
                v-if="!workbenchLastPreviewResult && !workbenchPreviewError"
                :tone="isAdmin ? 'neutral' : 'warn'"
              >
                {{
                  workbenchComposeImportRequired
                    ? 'Re-import the restored compose before generating another preview or applying stored Workbench changes.'
                    : isAdmin
                      ? 'Generate a compose preview to inspect the exact YAML output for the current catalog-managed service mix before apply.'
                      : 'An admin must generate a compose preview before compose apply becomes available.'
                }}
              </UiState>
              <template v-else-if="workbenchLastPreviewResult">
                <div class="grid gap-3 sm:grid-cols-3">
                  <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Preview revision</p>
                    <p class="text-lg font-semibold text-[color:var(--text)]">
                      {{ workbenchLastPreviewResult.revision }}
                    </p>
                  </UiPanel>
                  <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose lines</p>
                    <p class="text-lg font-semibold text-[color:var(--text)]">
                      {{ workbenchPreviewComposeLineCount }}
                    </p>
                  </UiPanel>
                  <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Snapshot match</p>
                    <p class="text-lg font-semibold text-[color:var(--text)]">
                      {{ workbenchPreviewMatchesSnapshot ? 'Yes' : 'No' }}
                    </p>
                  </UiPanel>
                </div>

                <UiPanel variant="raise" class="overflow-hidden p-0">
                  <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-xs text-[color:var(--muted)]">
                    <div>
                      <p class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Generated compose</p>
                      <p class="mt-1 font-mono text-[11px] text-[color:var(--text)] break-all">
                        {{ workbenchPreviewFingerprintLabel }}
                      </p>
                    </div>
                    <UiBadge :tone="workbenchPreviewMatchesSnapshot ? 'ok' : 'warn'">
                      {{ workbenchPreviewMatchesSnapshot ? 'Current snapshot' : 'Needs refresh' }}
                    </UiBadge>
                  </div>
                  <pre
                    class="max-h-[32rem] overflow-auto border-t border-[color:var(--line)] px-4 py-3 font-mono text-[12px] leading-5 text-[color:var(--text)] whitespace-pre"
                  ><code>{{ workbenchLastPreviewResult.compose }}</code></pre>
                </UiPanel>
              </template>
            </UiPanel>
          </div>

          <div class="workbench-shell-grid grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel
              variant="soft"
              class="workbench-shell-card workbench-shell-card--secondary space-y-4 p-4"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose restore</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Retained backups</h3>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <UiBadge :tone="workbenchComposeBackups.length > 0 ? 'ok' : 'neutral'">
                    {{ workbenchComposeBackups.length }} retained
                  </UiBadge>
                  <UiBadge :tone="workbenchComposeImportRequired ? 'warn' : 'neutral'">
                    {{ workbenchComposeImportRequired ? 'Import required' : 'Model unchanged' }}
                  </UiBadge>
                </div>
              </div>

              <p class="text-sm text-[color:var(--muted)]">
                Restore replaces the on-disk compose artifact from a retained backup. The stored Workbench model, including catalog-managed service selections, does not change automatically, so older backups can require an import before preview/apply resumes.
              </p>

              <UiInlineFeedback
                v-if="workbenchRestoreFeedback"
                :tone="workbenchRestoreFeedback.tone"
              >
                {{ workbenchRestoreFeedback.message }}
              </UiInlineFeedback>
              <UiInlineFeedback
                v-else-if="workbenchBackupInventoryError"
                :tone="workbenchBackupInventoryError.code === 'WB-409-BACKUP-INTEGRITY' ? 'warn' : 'error'"
              >
                {{ workbenchBackupInventoryErrorMessage }}
              </UiInlineFeedback>
              <UiPanel
                v-if="workbenchRestoreRemediationState"
                variant="soft"
                class="space-y-3 border border-[color:var(--line)] p-3 text-sm text-[color:var(--muted)]"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      {{ workbenchRestoreRemediationState.sourceLabel }}
                    </p>
                    <h4 class="mt-1 font-semibold text-[color:var(--text)]">
                      {{ workbenchRestoreRemediationState.title }}
                    </h4>
                  </div>
                  <UiBadge :tone="workbenchRestoreRemediationState.tone">
                    Follow-up required
                  </UiBadge>
                </div>
                <p>{{ workbenchRestoreRemediationState.message }}</p>

                <div
                  v-if="
                    workbenchRestoreRemediationState.details?.sourceFingerprint ||
                    workbenchRestoreRemediationState.details?.currentSourceFingerprint
                  "
                  class="grid gap-2 sm:grid-cols-2"
                >
                  <UiPanel
                    v-if="workbenchRestoreRemediationState.details?.sourceFingerprint"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      Stored fingerprint
                    </p>
                    <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                      {{ workbenchRestoreRemediationState.details?.sourceFingerprint }}
                    </p>
                  </UiPanel>
                  <UiPanel
                    v-if="workbenchRestoreRemediationState.details?.currentSourceFingerprint"
                    variant="raise"
                    class="space-y-1 p-3"
                  >
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                      Restored fingerprint
                    </p>
                    <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                      {{ workbenchRestoreRemediationState.details?.currentSourceFingerprint }}
                    </p>
                  </UiPanel>
                </div>

                <div
                  v-if="isAdmin && workbenchRestoreRemediationState.primaryAction"
                  class="flex flex-wrap gap-2"
                >
                  <UiButton
                    variant="ghost"
                    size="sm"
                    :disabled="
                      workbenchRemediationActionDisabled(
                        workbenchRestoreRemediationState.primaryAction,
                      )
                    "
                    @click="
                      runWorkbenchRemediationAction(workbenchRestoreRemediationState.primaryAction)
                    "
                  >
                    {{
                      workbenchRemediationActionLabel(
                        workbenchRestoreRemediationState.primaryAction,
                      )
                    }}
                  </UiButton>
                </div>
              </UiPanel>

              <UiState v-if="!isAdmin" tone="warn">
                Read-only access: admin permissions are required to inspect retained compose backups and trigger restore.
              </UiState>
              <UiState v-else-if="workbenchBackupInventoryStatus === 'loading'" loading>
                Loading retained compose backups...
              </UiState>
              <UiState v-else-if="workbenchComposeBackups.length === 0">
                No retained compose backups are available yet. The first successful compose apply creates the initial restore target.
              </UiState>
              <div v-else class="space-y-3">
                <button
                  v-for="backup in workbenchComposeBackups"
                  :key="backup.backupId"
                  type="button"
                  class="w-full rounded-2xl border px-4 py-3 text-left transition"
                  :class="
                    backup.backupId === workbenchSelectedComposeBackup?.backupId
                      ? 'border-[color:var(--accent)] bg-[color:var(--panel)]'
                      : 'border-[color:var(--line)] bg-[color:var(--panel-soft)] hover:border-[color:var(--accent)]/60'
                  "
                  :disabled="workbenchRestoreStatus === 'loading'"
                  @click="workbenchRestoreSelectedBackupId = backup.backupId"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Backup</p>
                      <p class="mt-2 font-mono text-sm text-[color:var(--text)]">{{ backup.backupId }}</p>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <UiBadge :tone="backup.backupId === workbenchLatestComposeBackup?.backupId ? 'ok' : 'neutral'">
                        {{ backup.backupId === workbenchLatestComposeBackup?.backupId ? 'Newest' : `Rev ${backup.revision}` }}
                      </UiBadge>
                      <UiBadge :tone="backup.backupId === workbenchSelectedComposeBackup?.backupId ? 'ok' : 'neutral'">
                        {{ backup.backupId === workbenchSelectedComposeBackup?.backupId ? 'Selected' : 'Available' }}
                      </UiBadge>
                    </div>
                  </div>
                  <div class="mt-3 grid gap-2 text-xs text-[color:var(--muted)] sm:grid-cols-3">
                    <p><span class="text-[color:var(--muted-2)]">Created</span> {{ fmtDate(backup.createdAt) }}</p>
                    <p><span class="text-[color:var(--muted-2)]">Bytes</span> {{ backup.composeBytes }}</p>
                    <p class="truncate"><span class="text-[color:var(--muted-2)]">Fingerprint</span> {{ backup.sourceFingerprint || '—' }}</p>
                  </div>
                </button>
              </div>
            </UiPanel>

            <UiPanel
              variant="raise"
              class="workbench-shell-card workbench-shell-card--secondary space-y-4 p-4 text-sm text-[color:var(--muted)]"
            >
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Restore execution</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Target and confirmation</h3>
              </div>
              <p>
                Confirmation phrase:
                <span class="font-mono text-[color:var(--text)]">{{ workbenchRestoreConfirmationPhrase }}</span>
              </p>

              <UiState v-if="!workbenchSelectedComposeBackup" tone="neutral">
                Select a retained backup to inspect its metadata and enable restore.
              </UiState>
              <template v-else>
                <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                  <UiPanel variant="soft" class="space-y-1 p-3">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Selected backup</p>
                    <p class="font-mono text-sm text-[color:var(--text)]">{{ workbenchSelectedComposeBackup.backupId }}</p>
                  </UiPanel>
                  <UiPanel variant="soft" class="space-y-1 p-3">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Stored revision</p>
                    <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSelectedComposeBackup.revision }}</p>
                  </UiPanel>
                  <UiPanel variant="soft" class="space-y-1 p-3">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Created</p>
                    <p class="text-sm font-semibold text-[color:var(--text)]">{{ fmtDate(workbenchSelectedComposeBackup.createdAt) }}</p>
                  </UiPanel>
                  <UiPanel variant="soft" class="space-y-2 p-3">
                    <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Backup fingerprint</p>
                    <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                      {{ workbenchSelectedComposeBackup.sourceFingerprint || 'No fingerprint recorded' }}
                    </p>
                  </UiPanel>
                </div>

                <UiState
                  v-if="workbenchLastRestoreResult?.requiresImport"
                  tone="warn"
                >
                  The last restore left compose drift against the stored Workbench snapshot. Safe recovery path: import, preview, then apply the desired catalog-managed state.
                </UiState>
              </template>

              <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                Confirmation phrase
              </label>
              <UiInput
                v-model="workbenchRestoreConfirmInput"
                :disabled="!isAdmin || workbenchRestoreActionDisabled"
                autocomplete="off"
                spellcheck="false"
                placeholder="Type the phrase exactly"
              />

              <div class="flex flex-wrap items-center gap-3">
                <UiButton
                  variant="danger"
                  size="sm"
                  class="w-full justify-center sm:w-auto"
                  :disabled="!canRestoreWorkbenchCompose"
                  @click="restoreWorkbenchCompose"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="workbenchRestoreStatus === 'loading'" />
                    {{ workbenchRestoreLabel }}
                  </span>
                </UiButton>
                <UiButton
                  v-if="isAdmin"
                  variant="ghost"
                  size="sm"
                  class="w-full justify-center sm:w-auto"
                  :disabled="workbenchRestoreStatus === 'loading' || workbenchBackupInventoryStatus === 'loading'"
                  @click="refreshWorkbenchComposeBackups"
                >
                  Refresh backups
                </UiButton>
                <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
                  Read-only access: admin permissions are required to restore compose from retained backups.
                </p>
              </div>
            </UiPanel>
          </div>
          </div>
          </template>
        </template>
      </UiPanel>

      <ProjectRuntimeUnitsSection
        v-else-if="activeSectionTab === 'runtime'"
        :containers="detail.containers"
      />
      <ProjectArchiveExecutionSection
        v-else-if="activeSectionTab === 'archive'"
        :project-name="projectName"
        :project-display-name="detail.project.normalizedName"
        :is-admin="isAdmin"
        @queued="load"
      />
      <ProjectActivityTimelineSection
        v-else
        :project-name="projectName"
        :project-display-name="detail.project.normalizedName"
      />
    </template>
  </section>
</template>
<style scoped>
.workbench-shell {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.workbench-shell-card {
  min-width: 0;
  border-radius: 1.25rem;
}

.workbench-shell-grid {
  gap: 0.875rem;
}

.workbench-shell-card--left,
.workbench-shell-card--right,
.workbench-shell-card--secondary {
  padding: 0.875rem;
}

@media (min-width: 640px) {
  .workbench-shell-card--left,
  .workbench-shell-card--right,
  .workbench-shell-card--secondary {
    padding: 1rem;
  }
}

@media (min-width: 1100px) {
  .workbench-shell {
    display: grid;
    grid-template-columns: minmax(0, 1.52fr) minmax(19rem, 0.98fr);
    align-items: start;
    column-gap: 1rem;
    row-gap: 0.875rem;
  }

  .workbench-shell-grid {
    display: contents;
  }

  .workbench-shell-card--left {
    grid-column: 1;
  }

  .workbench-shell-card--right {
    grid-column: 2;
  }

  .workbench-shell-card--secondary {
    grid-column: 1 / -1;
  }
}

</style>
