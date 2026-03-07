<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import NavIcon from '@/components/NavIcon.vue'
import { jobsApi } from '@/services/jobs'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import { useWorkbenchStore, type WorkbenchRequestStatus } from '@/stores/workbench'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { Job, JobDetail, JobListResponse } from '@/types/jobs'
import type {
  ProjectArchiveOptions,
  ProjectArchivePlan,
  ProjectContainer,
  ProjectDetail,
} from '@/types/projects'
import {
  buildWorkbenchPortSelectorKey,
  type WorkbenchComposeBackupMetadata,
  type WorkbenchManagedService,
  type WorkbenchMutationIssue,
  type WorkbenchOptionalServiceCatalogEntry,
  type WorkbenchOptionalServiceComposeMatch,
  type WorkbenchOptionalServiceMutationAction,
  type WorkbenchOptionalServiceMutationSummary,
  type WorkbenchPortMutationSummary,
  type WorkbenchPortSelector,
  type WorkbenchResourceField,
  type WorkbenchResourceMutationRequest,
  type WorkbenchResourceMutationSummary,
  type WorkbenchStackModule,
} from '@/types/workbench'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

interface WorkbenchServiceInventoryRow {
  serviceName: string
  image: string | null
  buildSource: string | null
  restartPolicy: string | null
  dependencies: string[]
}

interface WorkbenchPortInventoryRow {
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

interface WorkbenchResourceInventoryRow {
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

interface WorkbenchResourceInputState {
  limitCpus: string
  limitMemory: string
  reservationCpus: string
  reservationMemory: string
}

interface WorkbenchOptionalServiceCatalogRow {
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

interface WorkbenchComposeContextSummary {
  importedServices: number
  catalogManagedServices: number
  legacyModules: number
  matchedCatalogEntries: number
}

interface WorkbenchTopologyInventoryRow {
  key: string
  serviceName: string
  dependsOn: string[]
  dependedBy: string[]
  networkNames: string[]
  moduleTypes: string[]
}

interface WorkbenchInlineFeedbackState {
  tone: BadgeTone
  message: string
}

interface WorkbenchPendingOptionalServiceMutation {
  entryKey: string
  action: WorkbenchOptionalServiceMutationAction
  serviceName: string
  displayName: string
}

interface WorkbenchComposeIssueInventoryRow {
  key: string
  source: 'preview' | 'apply'
  sourceLabel: string
  issue: WorkbenchMutationIssue
}

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
const archivePlan = ref<ProjectArchivePlan | null>(null)
const archivePlanLoading = ref(false)
const archivePlanError = ref<string | null>(null)
const archiveExecuting = ref(false)
const archiveExecuteError = ref<string | null>(null)
const archiveExecutedWithWarnings = ref(false)
const archiveOptions = ref<ProjectArchiveOptions>({
  removeContainers: true,
  removeVolumes: false,
  removeIngress: true,
  removeDns: true,
})
const archiveConfirmInput = ref('')
const workbenchRestoreSelectedBackupId = ref('')
const workbenchRestoreConfirmInput = ref('')
const workbenchPendingOptionalServiceMutation = ref<WorkbenchPendingOptionalServiceMutation | null>(null)
const workbenchPortManualInputs = ref<Record<string, string>>({})
const workbenchResourceInputs = ref<Record<string, WorkbenchResourceInputState>>({})
const isAdmin = computed(() => authStore.isAdmin)

const projectJobs = ref<Job[]>([])
const jobsLoading = ref(false)
const jobsError = ref<string | null>(null)
const jobsPage = ref(1)
const jobsTotal = ref(0)
const jobsTotalPages = ref(0)
const jobsPageSize = 8

const jobLogsPanelOpen = ref(false)
const selectedJobId = ref<number | null>(null)
const selectedJob = ref<JobDetail | null>(null)
const selectedJobLoading = ref(false)
const selectedJobError = ref<string | null>(null)
const projectLogFontSizes = [11, 12, 13, 14] as const
const projectJobLogFontSize = ref<number>(12)

const projectName = computed(() => {
  const raw = route.params.name
  if (typeof raw !== 'string') return ''
  return decodeURIComponent(raw).trim()
})

const canGoJobsBack = computed(() => jobsPage.value > 1)
const canGoJobsForward = computed(() => jobsTotalPages.value > 0 && jobsPage.value < jobsTotalPages.value)
const selectedJobLogOutput = computed(() => selectedJob.value?.logLines?.join('\n') ?? '')
const archiveConfirmationPhrase = computed(() => {
  const normalized = (detail.value?.project.normalizedName || projectName.value || '').toLowerCase().trim()
  if (!normalized) return 'ARCHIVE PROJECT'
  return `ARCHIVE ${normalized}`
})
const canSubmitArchive = computed(() => {
  if (!isAdmin.value || archiveExecuting.value) return false
  if (archiveOptions.value.removeVolumes && !archiveOptions.value.removeContainers) return false
  return archiveConfirmInput.value.trim() === archiveConfirmationPhrase.value
})
const workbenchComposeSupported = computed(() => (detail.value?.runtime.composeFiles?.length ?? 0) > 0)
const workbenchSnapshot = computed(() => workbenchStore.snapshot)
const workbenchStatus = computed(() => workbenchStore.snapshotStatus)
const workbenchError = computed(() => workbenchStore.snapshotError)
const workbenchCatalog = computed(() => workbenchStore.catalog)
const workbenchCatalogStatus = computed(() => workbenchStore.catalogStatus)
const workbenchCatalogError = computed(() => workbenchStore.catalogError)
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
const workbenchComposeBackups = computed(() => workbenchStore.composeBackups)
const workbenchBackupInventoryStatus = computed(() => workbenchStore.backupInventoryStatus)
const workbenchBackupInventoryError = computed(() => workbenchStore.backupInventoryError)
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
  for (const dependency of snapshot.dependencies) {
    const serviceDependencies = dependenciesByService.get(dependency.serviceName)
    if (serviceDependencies) {
      serviceDependencies.push(dependency.dependsOn)
      continue
    }
    dependenciesByService.set(dependency.serviceName, [dependency.dependsOn])
  }

  return snapshot.services.map((service) => ({
    serviceName: service.serviceName,
    image: service.image?.trim() || null,
    buildSource: service.buildSource?.trim() || null,
    restartPolicy: service.restartPolicy?.trim() || null,
    dependencies: dependenciesByService.get(service.serviceName) ?? [],
  }))
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
const workbenchPortSummary = computed(() => {
  const summary = {
    total: workbenchPortInventory.value.length,
    assigned: 0,
    conflict: 0,
    unresolved: 0,
    unavailable: 0,
  }

  for (const port of workbenchPortInventory.value) {
    if (port.allocationStatus === 'conflict') {
      summary.conflict += 1
      continue
    }
    if (port.allocationStatus === 'unresolved') {
      summary.unresolved += 1
      continue
    }
    if (port.allocationStatus === 'unavailable') {
      summary.unavailable += 1
      continue
    }
    summary.assigned += 1
  }

  return summary
})
const workbenchPortBadgeTone = computed<BadgeTone>(() => {
  if (workbenchPortSummary.value.unavailable > 0) return 'error'
  if (workbenchPortSummary.value.conflict > 0) return 'warn'
  if (workbenchPortSummary.value.total > 0) return 'ok'
  return 'neutral'
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
const workbenchResourceSummary = computed(() => {
  const summary = {
    total: workbenchResourceInventory.value.length,
    tracked: 0,
    withLimits: 0,
    withReservations: 0,
    unconstrained: 0,
  }

  for (const resource of workbenchResourceInventory.value) {
    if (resource.tracked) summary.tracked += 1
    if (resource.hasLimits) summary.withLimits += 1
    if (resource.hasReservations) summary.withReservations += 1
    if (!resource.hasLimits && !resource.hasReservations) summary.unconstrained += 1
  }

  return summary
})
const workbenchResourceBadgeTone = computed<BadgeTone>(() => {
  if (workbenchResourceSummary.value.total > 0) return 'ok'
  return 'neutral'
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

function workbenchOptionalServiceMatchReasonLabel(reason: string): string {
  switch (reason.trim().toLowerCase()) {
    case 'service_name':
      return 'service name'
    case 'image_repository':
      return 'image repository'
    default:
      return 'catalog hint'
  }
}

const workbenchCurrentComposeSummary = computed<WorkbenchComposeContextSummary>(() => ({
  importedServices: workbenchSnapshot.value?.services.length ?? 0,
  catalogManagedServices: workbenchSnapshot.value?.managedServices.length ?? 0,
  legacyModules: workbenchCatalog.value?.legacyModules.records.length ?? 0,
  matchedCatalogEntries:
    workbenchCatalog.value?.entries.filter((entry) => entry.availability.composeServices.length > 0).length ?? 0,
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

const workbenchOptionalServiceBadgeTone = computed<BadgeTone>(() => {
  if (workbenchCatalogStatus.value === 'error') return 'error'
  if (workbenchCurrentComposeSummary.value.catalogManagedServices > 0) return 'ok'
  if (workbenchCurrentComposeSummary.value.legacyModules > 0) return 'warn'
  if (workbenchOptionalServiceInventory.value.length > 0) return 'neutral'
  return 'neutral'
})
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
const workbenchTopologySummary = computed(() => {
  const summary = {
    services: workbenchTopologyInventory.value.length,
    dependencyEdges: workbenchSnapshot.value?.dependencies.length ?? 0,
    connectedServices: 0,
    isolatedServices: 0,
    networks: [] as string[],
    moduleTypes: [] as string[],
  }

  const seenNetworks = new Set<string>()
  const seenModuleTypes = new Set<string>()

  for (const row of workbenchTopologyInventory.value) {
    if (row.dependsOn.length > 0 || row.dependedBy.length > 0) {
      summary.connectedServices += 1
    } else {
      summary.isolatedServices += 1
    }

    for (const networkName of row.networkNames) {
      if (seenNetworks.has(networkName)) continue
      seenNetworks.add(networkName)
      summary.networks.push(networkName)
    }

    for (const moduleType of row.moduleTypes) {
      if (seenModuleTypes.has(moduleType)) continue
      seenModuleTypes.add(moduleType)
      summary.moduleTypes.push(moduleType)
    }
  }

  return summary
})
const workbenchTopologyBadgeTone = computed<BadgeTone>(() => {
  if (workbenchTopologySummary.value.services > 0) return 'ok'
  return 'neutral'
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

const containerTone = (container: ProjectContainer): BadgeTone => {
  const normalized = container.status.trim().toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  if (normalized.includes('paused') || normalized.includes('restarting')) return 'warn'
  return 'neutral'
}

const fmtDate = (value?: string | null) => {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

const loadWorkbench = () => {
  const name = projectName.value
  workbenchPendingOptionalServiceMutation.value = null
  if (!name) {
    workbenchStore.reset()
    return
  }
  if (!workbenchComposeSupported.value) {
    workbenchStore.reset()
    return
  }
  void workbenchStore.loadSnapshot(name)
  void workbenchStore.loadCatalog(name)
  if (isAdmin.value) {
    void workbenchStore.loadComposeBackups(name)
  }
}

const applyProjectJobsResponse = (data: JobListResponse) => {
  projectJobs.value = data.jobs ?? []
  jobsPage.value = data.page ?? 1
  jobsTotal.value = data.total ?? 0
  jobsTotalPages.value = data.totalPages ?? 0
}

const loadProjectJobs = async (page = 1) => {
  const name = projectName.value
  if (!name) {
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = 1
    jobsError.value = 'Invalid project name.'
    return
  }

  jobsLoading.value = true
  jobsError.value = null
  try {
    const { data } = await projectsApi.listJobs(name, { page, limit: jobsPageSize })
    applyProjectJobsResponse(data)
  } catch (err) {
    jobsError.value = apiErrorMessage(err)
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = page
  } finally {
    jobsLoading.value = false
  }
}

const applyArchiveDefaults = (plan: ProjectArchivePlan) => {
  archiveOptions.value = {
    removeContainers: plan.defaults.removeContainers,
    removeVolumes: plan.defaults.removeVolumes,
    removeIngress: plan.defaults.removeIngress,
    removeDns: plan.defaults.removeDns,
  }
}

const loadArchivePlan = async () => {
  const name = projectName.value
  if (!name) {
    archivePlan.value = null
    archivePlanError.value = 'Invalid project name.'
    return
  }
  if (!isAdmin.value) {
    archivePlan.value = null
    archivePlanError.value = null
    return
  }

  archivePlanLoading.value = true
  archivePlanError.value = null
  try {
    const { data } = await projectsApi.getArchivePlan(name)
    archivePlan.value = data.plan
    applyArchiveDefaults(data.plan)
  } catch (err) {
    archivePlan.value = null
    archivePlanError.value = apiErrorMessage(err)
  } finally {
    archivePlanLoading.value = false
  }
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
    loadWorkbench()
    await loadProjectJobs(1)
    await loadArchivePlan()
  } catch (err) {
    detail.value = null
    error.value = apiErrorMessage(err)
    workbenchStore.reset()
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = 1
    archivePlan.value = null
    archivePlanError.value = null
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
  await Promise.all([
    workbenchStore.loadSnapshot(name),
    workbenchStore.loadCatalog(name),
  ])
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

  await workbenchStore.loadCatalog(name)

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

  const result = await workbenchStore.previewCompose(name)
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

  const result = await workbenchStore.applyCompose(name)
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
  const name = projectName.value
  if (!name || !isAdmin.value || !workbenchComposeSupported.value) return
  await workbenchStore.loadComposeBackups(name)
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
    await loadProjectJobs(1)
  } catch (err) {
    const message = apiErrorMessage(err)
    stackRestartError.value = message
    toastStore.error(message, 'Queue failed')
  } finally {
    stackRestarting.value = false
  }
}

const queueArchive = async () => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Archive blocked')
    return
  }
  if (!canSubmitArchive.value) return

  archiveExecuteError.value = null
  archiveExecuting.value = true
  try {
    const payload = {
      removeContainers: archiveOptions.value.removeContainers,
      removeVolumes: archiveOptions.value.removeVolumes,
      removeIngress: archiveOptions.value.removeIngress,
      removeDns: archiveOptions.value.removeDns,
    }
    const { data } = await projectsApi.archiveProject(name, payload)
    archivePlan.value = data.plan
    archiveExecutedWithWarnings.value = (data.plan.warnings?.length ?? 0) > 0
    archiveConfirmInput.value = ''

    if (archiveExecutedWithWarnings.value) {
      toastStore.warn(
        `Archive queued (job #${data.job.id}) with ${data.plan.warnings.length} warning(s) in plan preview.`,
        'Archive queued',
      )
    } else {
      toastStore.success(`Archive queued (job #${data.job.id}).`, 'Project cleanup')
    }
    await load()
  } catch (err) {
    const message = apiErrorMessage(err)
    archiveExecuteError.value = message
    toastStore.error(message, 'Archive queue failed')
  } finally {
    archiveExecuting.value = false
  }
}

const openJobLogs = async (jobId: number) => {
  selectedJobId.value = jobId
  selectedJob.value = null
  selectedJobError.value = null
  jobLogsPanelOpen.value = true
  await refreshSelectedJobLogs()
}

const refreshSelectedJobLogs = async () => {
  if (!selectedJobId.value) return

  selectedJobLoading.value = true
  selectedJobError.value = null
  try {
    const { data } = await jobsApi.get(selectedJobId.value)
    selectedJob.value = data
  } catch (err) {
    selectedJobError.value = apiErrorMessage(err)
    selectedJob.value = null
  } finally {
    selectedJobLoading.value = false
  }
}

const copySelectedJobLogs = async () => {
  const output = selectedJobLogOutput.value
  if (!output) {
    toastStore.warn('No logs to copy yet.', 'Nothing to copy')
    return
  }

  try {
    await copyTextToClipboard(output)
    toastStore.success('Logs copied to clipboard.', 'Copied')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const cycleProjectJobLogFontSize = () => {
  const currentIndex = projectLogFontSizes.findIndex((size) => size === projectJobLogFontSize.value)
  const nextIndex = currentIndex === -1 ? 0 : (currentIndex + 1) % projectLogFontSizes.length
  projectJobLogFontSize.value = projectLogFontSizes[nextIndex] ?? projectLogFontSizes[0]
}

const goToJobsPage = async (nextPage: number) => {
  if (nextPage < 1) return
  if (jobsTotalPages.value > 0 && nextPage > jobsTotalPages.value) return
  await loadProjectJobs(nextPage)
}

watch(workbenchPortInventory, (ports) => {
  syncWorkbenchPortManualInputs(ports)
}, { immediate: true })

watch(workbenchResourceInventory, (resources) => {
  syncWorkbenchResourceInputs(resources)
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
  stackRestartError.value = null
  archivePlan.value = null
  archivePlanError.value = null
  archiveExecuteError.value = null
  archiveExecuting.value = false
  archiveExecutedWithWarnings.value = false
  archiveConfirmInput.value = ''
  jobLogsPanelOpen.value = false
  workbenchRestoreSelectedBackupId.value = ''
  workbenchRestoreConfirmInput.value = ''
  workbenchPortManualInputs.value = {}
  workbenchResourceInputs.value = {}
  workbenchStore.reset()
  void load()
})

watch(jobLogsPanelOpen, (open) => {
  if (open) return
  selectedJobId.value = null
  selectedJob.value = null
  selectedJobError.value = null
  selectedJobLoading.value = false
})

watch(
  () => archiveOptions.value.removeContainers,
  (enabled) => {
    if (enabled) return
    archiveOptions.value.removeVolumes = false
  },
)
</script>

<template>
  <section class="page space-y-8">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project workspace</p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">{{ projectName || 'Project detail' }}</h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Runtime metadata, containers, and job history for this deployment.
        </p>
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
        class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Workspace guidance</p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            Read access is available to all authenticated users. Restart actions require admin permissions.
          </p>
        </div>
        <UiBadge :tone="statusTone(detail.project.record?.status || '')">
          {{ detail.project.record?.status || 'unknown' }}
        </UiBadge>
      </UiPanel>

      <hr />

      <div class="grid gap-5 xl:grid-cols-3">
        <UiPanel class="space-y-5 p-6 xl:col-span-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project profile</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">General</h2>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Project</p>
              <p class="text-base font-semibold text-[color:var(--text)]">{{ detail.project.name }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Normalized</p>
              <p class="font-mono text-sm text-[color:var(--text)]">{{ detail.project.normalizedName }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Last updated</p>
              <p class="text-sm text-[color:var(--muted)]">{{ fmtDate(detail.project.record?.updatedAt) }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Source</p>
              <p class="text-sm text-[color:var(--text)]">{{ detail.runtime.source || 'unknown' }}</p>
            </div>
            <div class="space-y-1 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Path</p>
              <p class="font-mono text-xs text-[color:var(--muted)] break-all">{{ detail.runtime.path }}</p>
            </div>
            <div class="space-y-1 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Repository</p>
              <p class="text-sm text-[color:var(--muted)] break-all">
                {{ detail.project.record?.repoUrl || 'No repository URL recorded' }}
              </p>
            </div>
          </div>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-4 p-5">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Runtime</p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Compose and env</h2>
          </div>
          <div class="space-y-2 text-sm text-[color:var(--muted)]">
            <div class="flex items-center justify-between gap-2">
              <span>Compose files</span>
              <span class="font-semibold text-[color:var(--text)]">{{ detail.runtime.composeFiles.length }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>.env</span>
              <span class="font-semibold text-[color:var(--text)]">{{ detail.runtime.envExists ? 'present' : 'missing' }}</span>
            </div>
            <p class="break-all font-mono text-xs text-[color:var(--muted-2)]">{{ detail.runtime.envPath }}</p>
          </div>
          <UiPanel variant="raise" class="space-y-3 p-4">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Stack action</p>
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
            <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
              Read-only access: admin permissions are required to restart this project stack.
            </p>
            <UiInlineFeedback v-if="stackRestartError" tone="error">
              {{ stackRestartError }}
            </UiInlineFeedback>
          </UiPanel>
        </UiPanel>
      </div>

      <div class="grid gap-5 xl:grid-cols-2">
        <UiPanel class="space-y-5 p-6">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Network</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Published ports</h2>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <UiPanel variant="soft" class="space-y-2 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Proxy</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ detail.network.proxyPort || '—' }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-2 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Database</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ detail.network.dbPort || '—' }}</p>
            </UiPanel>
          </div>
          <div v-if="detail.network.publishedPorts.length === 0" class="text-sm text-[color:var(--muted)]">
            No published container ports detected.
          </div>
          <div v-else class="space-y-2">
            <UiPanel
              v-for="binding in detail.network.publishedPorts"
              :key="`${binding.container}-${binding.hostPort}-${binding.containerPort}-${binding.proto}`"
              variant="soft"
              class="space-y-1 p-3 text-sm text-[color:var(--muted)]"
            >
              <p class="font-semibold text-[color:var(--text)]">{{ binding.container }}</p>
              <p class="font-mono text-xs text-[color:var(--muted-2)]">
                {{ binding.hostIp || '0.0.0.0' }}:{{ binding.hostPort }} -> {{ binding.containerPort }}/{{ binding.proto || 'tcp' }}
              </p>
            </UiPanel>
          </div>
        </UiPanel>

        <UiPanel class="space-y-5 p-6">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Compose</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Detected files</h2>
          </div>
          <div v-if="detail.runtime.composeFiles.length > 0" class="space-y-2">
            <UiPanel
              v-for="file in detail.runtime.composeFiles"
              :key="file"
              variant="soft"
              class="p-3 font-mono text-xs text-[color:var(--muted)] break-all"
            >
              {{ file }}
            </UiPanel>
          </div>
          <p v-else class="text-sm text-[color:var(--muted)]">No compose files discovered in the project directory.</p>
        </UiPanel>
      </div>

      <UiPanel class="space-y-5 p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Workbench</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Compose authority shell</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Read/import state for the stored Workbench model plus current compose context, admin-only optional-service add/remove controls, and the existing port, resource, and compose preview/apply surfaces.
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
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Revision</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.revision }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Model version</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.modelVersion }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Services</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.services.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Warnings</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.warnings.length }}</p>
            </UiPanel>
          </div>

          <div class="grid gap-4 xl:grid-cols-2">
            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Stored metadata</p>
              <div class="space-y-2 text-xs text-[color:var(--muted)]">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span>Ports tracked</span>
                  <span class="font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.ports.length }}</span>
                </div>
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span>Resources tracked</span>
                  <span class="font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.resources.length }}</span>
                </div>
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span>Managed services tracked</span>
                  <span class="font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.managedServices.length }}</span>
                </div>
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span>Environment refs</span>
                  <span class="font-semibold text-[color:var(--text)]">{{ workbenchSnapshot.envRefs.length }}</span>
                </div>
              </div>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Source metadata</p>
              <div class="space-y-2 text-xs text-[color:var(--muted)]">
                <div class="space-y-1">
                  <span class="text-[color:var(--muted-2)]">Compose path</span>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">{{ workbenchSnapshot.composePath }}</p>
                </div>
                <div class="space-y-1">
                  <span class="text-[color:var(--muted-2)]">Project directory</span>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">{{ workbenchSnapshot.projectDir }}</p>
                </div>
                <div class="space-y-1">
                  <span class="text-[color:var(--muted-2)]">Source fingerprint</span>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">{{ workbenchFingerprintLabel }}</p>
                </div>
              </div>
            </UiPanel>
          </div>

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

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Optional services</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Catalog controls</h3>
                </div>
                <UiBadge :tone="workbenchOptionalServiceBadgeTone">
                  {{ workbenchOptionalServiceInventory.length }} entries
                </UiBadge>
              </div>

              <p class="text-sm text-[color:var(--muted)]">
                Backend ordering is preserved here: the current compose context comes from the stored Workbench snapshot, and each catalog card shows compose matches, catalog-managed state, and legacy transition metadata alongside the live add/remove path.
              </p>

              <UiState v-if="workbenchCatalogStatus === 'loading'" loading>
                Loading optional-service catalog...
              </UiState>
              <UiState v-else-if="workbenchCatalogStatus === 'error'" tone="error">
                {{ workbenchCatalogErrorMessage }}
              </UiState>
              <UiState v-else-if="workbenchOptionalServiceInventory.length === 0">
                No optional-service catalog entries are available for this project yet.
              </UiState>
              <div v-else class="grid gap-3 md:grid-cols-2">
                <UiPanel
                  v-for="entry in workbenchOptionalServiceInventory"
                  :key="entry.key"
                  variant="raise"
                  class="space-y-4 p-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">{{ entry.category }}</p>
                      <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">{{ entry.displayName }}</h4>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <UiBadge :tone="entry.availabilityTone">
                        {{ entry.availabilityLabel }}
                      </UiBadge>
                      <UiBadge :tone="entry.currentStateTone">
                        {{ entry.currentStateLabel }}
                      </UiBadge>
                    </div>
                  </div>

                  <p class="text-sm text-[color:var(--muted)]">{{ entry.description }}</p>

                  <div class="grid gap-3 text-xs text-[color:var(--muted)] sm:grid-cols-2">
                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Default service</span>
                        <span class="font-semibold text-[color:var(--text)]">{{ entry.defaultServiceName }}</span>
                      </div>
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Suggested image</span>
                        <span class="max-w-full break-all text-right text-[color:var(--text)]">
                          {{ entry.suggestedImage || 'Not declared' }}
                        </span>
                      </div>
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Baseline port</span>
                        <span class="font-mono text-[color:var(--text)]">{{ entry.defaultContainerPortLabel }}</span>
                      </div>
                    </UiPanel>

                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Target state</span>
                        <span class="font-semibold text-[color:var(--text)]">{{ entry.targetStateLabel }}</span>
                      </div>
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Mutation path</span>
                        <span class="font-semibold text-[color:var(--text)]">
                          {{ entry.mutationReady ? 'Catalog-managed' : 'Read-only' }}
                        </span>
                      </div>
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <span>Preview/apply path</span>
                        <span class="font-semibold text-[color:var(--text)]">
                          {{ entry.composeGenerationReady ? 'Ready' : 'Not ready' }}
                        </span>
                      </div>
                    </UiPanel>
                  </div>

                  <div class="grid gap-3 text-xs text-[color:var(--muted)] xl:grid-cols-3">
                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <span class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose matches</span>
                        <UiBadge :tone="entry.composeServices.length > 0 ? 'neutral' : 'neutral'">
                          {{ entry.composeServices.length }}
                        </UiBadge>
                      </div>
                      <div v-if="entry.composeServices.length > 0" class="space-y-2">
                        <div
                          v-for="composeService in entry.composeServices"
                          :key="`${entry.key}-${composeService.serviceName}-${composeService.matchReason}`"
                          class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-2"
                        >
                          <p class="font-semibold text-[color:var(--text)]">{{ composeService.serviceName }}</p>
                          <p class="mt-1 break-all">{{ composeService.image || 'Image not declared' }}</p>
                          <p class="mt-1 text-[color:var(--muted-2)]">
                            Matched by {{ workbenchOptionalServiceMatchReasonLabel(composeService.matchReason) }}.
                          </p>
                        </div>
                      </div>
                      <p v-else>No imported compose match is currently visible.</p>
                    </UiPanel>

                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <span class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Managed services</span>
                        <UiBadge :tone="entry.managedServices.length > 0 ? 'ok' : 'neutral'">
                          {{ entry.managedServices.length }}
                        </UiBadge>
                      </div>
                      <div v-if="entry.managedServices.length > 0" class="space-y-2">
                        <div
                          v-for="managedService in entry.managedServices"
                          :key="`${entry.key}-managed-${managedService.serviceName}`"
                          class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-2"
                        >
                          <p class="font-semibold text-[color:var(--text)]">{{ managedService.serviceName }}</p>
                          <p class="mt-1 text-[color:var(--muted-2)]">Tracked as {{ managedService.entryKey }}.</p>
                        </div>
                      </div>
                      <p v-else>No catalog-managed service is stored for this entry yet.</p>
                    </UiPanel>

                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-center justify-between gap-2">
                        <span class="uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Legacy metadata</span>
                        <UiBadge :tone="entry.legacyModules.length > 0 ? 'warn' : 'neutral'">
                          {{ entry.legacyModules.length }}
                        </UiBadge>
                      </div>
                      <div v-if="entry.legacyModules.length > 0" class="space-y-2">
                        <div
                          v-for="legacyModule in entry.legacyModules"
                          :key="`${entry.key}-legacy-${legacyModule.serviceName}-${legacyModule.moduleType}`"
                          class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-2"
                        >
                          <p class="font-semibold text-[color:var(--text)]">{{ legacyModule.serviceName || 'Unknown service' }}</p>
                          <p class="mt-1 text-[color:var(--muted-2)]">
                            Legacy {{ legacyModule.moduleType }} metadata remains separate from catalog-managed records.
                          </p>
                        </div>
                      </div>
                      <p v-else>No legacy transition metadata is attached to this entry.</p>
                    </UiPanel>
                  </div>

                  <div class="space-y-2">
                    <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Transition notes</p>
                    <div class="space-y-2">
                      <UiPanel
                        v-for="note in entry.notes"
                        :key="`${entry.key}-${note}`"
                        variant="soft"
                        class="p-3 text-xs text-[color:var(--muted)]"
                      >
                        {{ note }}
                      </UiPanel>
                    </div>
                    <p
                      v-if="entry.legacyModuleType && entry.legacyMutationPath"
                      class="text-xs text-[color:var(--muted)]"
                    >
                      Legacy transition path: <span class="font-mono text-[color:var(--text)]">{{ entry.legacyMutationPath }}</span>
                      still reports <span class="font-semibold text-[color:var(--text)]">{{ entry.legacyModuleType }}</span> metadata separately.
                    </p>
                  </div>

                  <UiPanel variant="soft" class="space-y-3 p-3 text-xs text-[color:var(--muted)]">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Catalog mutation</p>
                        <p class="mt-1">
                          {{
                            workbenchOptionalServicePendingAction(entry) === 'remove'
                              ? 'Remove the stored catalog-managed service record. Imported compose matches and legacy metadata stay visible separately.'
                              : 'Add the catalog-managed service to the stored Workbench snapshot. Preview/apply is still required before compose changes hit disk.'
                          }}
                        </p>
                      </div>
                      <UiBadge :tone="entry.mutationReady ? 'ok' : 'neutral'">
                        {{ entry.mutationReady ? 'Mutation ready' : 'Read-only' }}
                      </UiBadge>
                    </div>

                    <div v-if="isAdmin" class="space-y-3">
                      <UiInlineFeedback
                        v-if="workbenchOptionalServiceFeedback(entry)"
                        :tone="workbenchOptionalServiceFeedback(entry)?.tone || 'neutral'"
                      >
                        {{ workbenchOptionalServiceFeedback(entry)?.message }}
                      </UiInlineFeedback>

                      <div v-if="workbenchOptionalServicePendingConfirmation(entry)" class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
                        <p class="text-[color:var(--text)]">
                          {{
                            workbenchPendingOptionalServiceMutation?.action === 'remove'
                              ? `Remove ${workbenchPendingOptionalServiceMutation.serviceName} from the stored Workbench snapshot?`
                              : `Add ${entry.displayName} to the stored Workbench snapshot as ${entry.defaultServiceName}?`
                          }}
                        </p>
                        <p>
                          {{
                            workbenchPendingOptionalServiceMutation?.action === 'remove'
                              ? 'This only removes the catalog-managed record. Existing compose-owned services and legacy transition metadata remain visible after the shell refresh.'
                              : 'This only updates the stored Workbench model. Run preview/apply later if you want the generated compose artifact to change on disk.'
                          }}
                        </p>
                        <div class="flex flex-wrap gap-2">
                          <UiButton
                            variant="primary"
                            size="sm"
                            :disabled="workbenchOptionalServiceActionDisabled(entry)"
                            @click="confirmWorkbenchOptionalServiceMutation(entry)"
                          >
                            <span class="inline-flex items-center gap-2">
                              <UiInlineSpinner v-if="workbenchOptionalServiceBusy(entry)" />
                              {{ workbenchPendingOptionalServiceMutation?.action === 'remove' ? 'Confirm remove' : 'Confirm add' }}
                            </span>
                          </UiButton>
                          <UiButton
                            variant="ghost"
                            size="sm"
                            :disabled="workbenchOptionalServiceBusy(entry)"
                            @click="cancelWorkbenchOptionalServiceMutation(entry.key)"
                          >
                            Cancel
                          </UiButton>
                        </div>
                      </div>
                      <UiButton
                        v-else
                        :variant="workbenchOptionalServicePendingAction(entry) === 'remove' ? 'ghost' : 'primary'"
                        size="sm"
                        :disabled="workbenchOptionalServiceActionDisabled(entry)"
                        @click="queueWorkbenchOptionalServiceMutation(entry)"
                      >
                        <span class="inline-flex items-center gap-2">
                          <UiInlineSpinner v-if="workbenchOptionalServiceBusy(entry)" />
                          {{ workbenchOptionalServicePendingLabel(entry) }}
                        </span>
                      </UiButton>
                    </div>
                    <p v-else>
                      Read-only access: admin permissions are required to add or remove catalog-managed services from the stored Workbench snapshot.
                    </p>
                  </UiPanel>
                </UiPanel>
              </div>
            </UiPanel>

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Current compose context</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Snapshot-backed summary</h3>
              </div>
              <p>
                Current compose visibility stays read-only here. Admins can add or remove catalog-managed services from the cards on the left, then use preview/apply or restore without leaving Project Detail. Non-admin users keep the same snapshot-backed visibility with explicit read-only rationale.
              </p>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Imported services</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchCurrentComposeSummary.importedServices }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Catalog-managed</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchCurrentComposeSummary.catalogManagedServices }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Catalog matches</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchCurrentComposeSummary.matchedCatalogEntries }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Legacy transition records</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchCurrentComposeSummary.legacyModules }}</p>
                </UiPanel>
              </div>

              <div class="space-y-2">
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Imported compose services</p>
                <div v-if="workbenchSnapshot.services.length > 0" class="flex flex-wrap gap-2">
                  <UiBadge
                    v-for="service in workbenchSnapshot.services"
                    :key="`compose-context-${service.serviceName}`"
                    tone="neutral"
                  >
                    {{ service.serviceName }}
                  </UiBadge>
                </div>
                <p v-else class="text-xs">Import the current compose to populate tracked service names here.</p>
              </div>

              <div class="space-y-2">
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Managed service records</p>
                <div v-if="workbenchSnapshot.managedServices.length > 0" class="space-y-2">
                  <UiPanel
                    v-for="managedService in workbenchSnapshot.managedServices"
                    :key="`managed-context-${managedService.serviceName}`"
                    variant="soft"
                    class="space-y-1 p-3 text-xs"
                  >
                    <p class="font-semibold text-[color:var(--text)]">{{ managedService.serviceName }}</p>
                    <p class="text-[color:var(--muted-2)]">Entry key: {{ managedService.entryKey }}</p>
                  </UiPanel>
                </div>
                <p v-else class="text-xs">No catalog-managed service records are stored in this snapshot yet.</p>
              </div>

              <div class="space-y-2">
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Legacy transition records</p>
                <div
                  v-if="workbenchCatalog?.legacyModules.records.length"
                  class="space-y-2"
                >
                  <UiPanel
                    v-for="legacyModule in workbenchCatalog.legacyModules.records"
                    :key="`legacy-context-${legacyModule.serviceName}-${legacyModule.moduleType}`"
                    variant="soft"
                    class="space-y-1 p-3 text-xs"
                  >
                    <p class="font-semibold text-[color:var(--text)]">{{ legacyModule.serviceName || 'Unknown service' }}</p>
                    <p class="text-[color:var(--muted-2)]">Legacy metadata type: {{ legacyModule.moduleType }}</p>
                  </UiPanel>
                </div>
                <p v-else class="text-xs">No legacy transition records are currently attached to this project.</p>
              </div>

              <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
                Read-only access: non-admin users can inspect current compose context and transition metadata, but snapshot mutations remain restricted to admin workflows.
              </p>
            </UiPanel>
          </div>

          <template v-if="workbenchSnapshotReady">

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Services</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Stored inventory</h3>
                </div>
                <UiBadge :tone="workbenchServiceInventory.length > 0 ? 'ok' : 'neutral'">
                  {{ workbenchServiceInventory.length }} tracked
                </UiBadge>
              </div>

              <UiState v-if="workbenchServiceInventory.length === 0">
                No Workbench service rows are stored for this snapshot yet.
              </UiState>
              <div v-else class="grid gap-3 md:grid-cols-2">
                <UiPanel
                  v-for="service in workbenchServiceInventory"
                  :key="service.serviceName"
                  variant="raise"
                  class="space-y-4 p-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service</p>
                      <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">{{ service.serviceName }}</h4>
                    </div>
                    <UiBadge :tone="service.dependencies.length > 0 ? 'ok' : 'neutral'">
                      {{ service.dependencies.length > 0 ? `${service.dependencies.length} deps` : 'No deps' }}
                    </UiBadge>
                  </div>

                  <div class="space-y-2 text-xs text-[color:var(--muted)]">
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Image</span>
                      <span class="max-w-full break-all text-right text-[color:var(--text)]">
                        {{ service.image || 'Not declared' }}
                      </span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Build source</span>
                      <span class="max-w-full break-all text-right text-[color:var(--text)]">
                        {{ service.buildSource || 'Not declared' }}
                      </span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Restart policy</span>
                      <span class="max-w-full break-all text-right text-[color:var(--text)]">
                        {{ service.restartPolicy || 'Default compose behavior' }}
                      </span>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Depends on</p>
                    <div v-if="service.dependencies.length > 0" class="flex flex-wrap gap-2">
                      <UiBadge
                        v-for="dependency in service.dependencies"
                        :key="`${service.serviceName}-${dependency}`"
                        tone="neutral"
                      >
                        {{ dependency }}
                      </UiBadge>
                    </div>
                    <p v-else class="text-xs text-[color:var(--muted)]">No declared service dependencies.</p>
                  </div>
                </UiPanel>
              </div>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Warnings</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Import and pass-through</h3>
                </div>
                <UiBadge :tone="workbenchWarningsList.length > 0 ? 'warn' : 'ok'">
                  {{ workbenchWarningsList.length }} visible
                </UiBadge>
              </div>

              <UiState v-if="workbenchWarningsList.length === 0" tone="ok">
                No Workbench import warnings are recorded for this snapshot.
              </UiState>
              <div v-else class="space-y-3">
                <UiPanel
                  v-for="warning in workbenchWarningsList"
                  :key="`${warning.code}-${warning.path}-${warning.message}`"
                  variant="raise"
                  class="space-y-3 p-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-2">
                    <UiBadge tone="warn">{{ warning.code }}</UiBadge>
                    <span class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                      {{ warning.path || 'compose' }}
                    </span>
                  </div>
                  <p class="text-sm text-[color:var(--text)]">{{ warning.message }}</p>
                </UiPanel>
              </div>
            </UiPanel>
          </div>

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
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

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
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

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ports</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Stored mappings</h3>
                </div>
                <UiBadge :tone="workbenchPortBadgeTone">
                  {{ workbenchPortInventory.length }} tracked
                </UiBadge>
              </div>

              <UiState v-if="workbenchPortInventory.length === 0">
                No Workbench port rows are stored for this snapshot yet.
              </UiState>
              <div v-else class="space-y-3">
                <UiListRow
                  v-for="port in workbenchPortInventory"
                  :key="port.key"
                  as="article"
                  class="space-y-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service</p>
                      <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">{{ port.serviceName }}</h4>
                      <p class="mt-1 font-mono text-[11px] text-[color:var(--muted-2)]">{{ port.mappingLabel }}</p>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <UiBadge :tone="port.assignmentStrategyTone">
                        {{ port.assignmentStrategyLabel }}
                      </UiBadge>
                      <UiBadge :tone="port.allocationStatusTone">
                        {{ port.allocationStatusLabel }}
                      </UiBadge>
                    </div>
                  </div>

                  <div class="grid gap-2 text-xs text-[color:var(--muted)] sm:grid-cols-2">
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Container port</span>
                      <span class="font-mono text-[color:var(--text)]">{{ port.containerPort }}</span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Protocol</span>
                      <span class="font-mono uppercase text-[color:var(--text)]">{{ port.protocol }}</span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Host IP</span>
                      <span class="font-mono text-[color:var(--text)]">{{ port.hostIp }}</span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Requested host port</span>
                      <span class="font-mono text-[color:var(--text)]">
                        {{ port.requestedHostPort || 'Not declared' }}
                      </span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Effective host port</span>
                      <span class="font-mono text-[color:var(--text)]">
                        {{ port.effectiveHostPortLabel }}
                      </span>
                    </div>
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <span>Strategy</span>
                      <span class="text-[color:var(--text)]">{{ port.assignmentStrategyLabel }}</span>
                    </div>
                  </div>

                  <p
                    class="text-xs"
                    :class="
                      port.allocationStatus === 'unavailable'
                        ? 'text-[color:var(--danger)]'
                        : port.allocationStatus === 'conflict'
                          ? 'text-[color:var(--warn)]'
                          : 'text-[color:var(--muted)]'
                    "
                  >
                    {{ port.guidance }}
                  </p>

                  <div
                    v-if="isAdmin"
                    class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/45 p-3"
                  >
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                          Manual override
                        </p>
                        <p class="mt-1 text-xs text-[color:var(--muted)]">
                          Pin a host port for this mapping or clear manual mode to hand control back to the sequential resolver.
                        </p>
                      </div>
                      <UiBadge :tone="port.assignmentStrategy === 'manual' ? 'ok' : 'neutral'">
                        {{ port.assignmentStrategy === 'manual' ? 'Manual active' : 'Auto managed' }}
                      </UiBadge>
                    </div>

                    <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto] lg:items-end">
                      <label class="space-y-2">
                        <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                          Host port
                        </span>
                        <UiInput
                          :model-value="workbenchPortInputValue(port)"
                          type="number"
                          min="1"
                          max="65535"
                          step="1"
                          placeholder="8080"
                          :disabled="
                            workbenchPortMutationBusy(port) ||
                            workbenchOptionalServiceMutationStatus === 'loading' ||
                            workbenchPreviewStatus === 'loading' ||
                            workbenchApplyStatus === 'loading' ||
                            workbenchRestoreStatus === 'loading'
                          "
                          @update:model-value="setWorkbenchPortInputValue(port.key, $event)"
                        />
                      </label>
                      <UiButton
                        variant="primary"
                        size="sm"
                        :disabled="
                          workbenchPortMutationBusy(port) ||
                          workbenchResolveStatus === 'loading' ||
                          workbenchOptionalServiceMutationStatus === 'loading' ||
                          workbenchPreviewStatus === 'loading' ||
                          workbenchApplyStatus === 'loading' ||
                          workbenchRestoreStatus === 'loading'
                        "
                        @click="setManualWorkbenchPort(port)"
                      >
                        <span class="inline-flex items-center gap-2">
                          <UiInlineSpinner v-if="workbenchPortMutationBusy(port)" />
                          Set manual
                        </span>
                      </UiButton>
                      <UiButton
                        variant="ghost"
                        size="sm"
                        :disabled="
                          workbenchPortMutationBusy(port) ||
                          workbenchResolveStatus === 'loading' ||
                          workbenchOptionalServiceMutationStatus === 'loading' ||
                          workbenchPreviewStatus === 'loading' ||
                          workbenchApplyStatus === 'loading' ||
                          workbenchRestoreStatus === 'loading'
                        "
                        @click="resetWorkbenchPortToAuto(port)"
                      >
                        Reset to auto
                      </UiButton>
                      <UiButton
                        variant="ghost"
                        size="sm"
                        :disabled="
                          workbenchPortSuggestionStatus(port) === 'loading' ||
                          workbenchPortMutationBusy(port) ||
                          workbenchOptionalServiceMutationStatus === 'loading' ||
                          workbenchPreviewStatus === 'loading' ||
                          workbenchApplyStatus === 'loading' ||
                          workbenchRestoreStatus === 'loading'
                        "
                        @click="loadWorkbenchPortSuggestions(port)"
                      >
                        <span class="inline-flex items-center gap-2">
                          <UiInlineSpinner v-if="workbenchPortSuggestionStatus(port) === 'loading'" />
                          Suggestions
                        </span>
                      </UiButton>
                    </div>

                    <UiInlineFeedback
                      v-if="workbenchPortMutationFeedback(port)"
                      :tone="workbenchPortMutationFeedback(port)?.tone || 'neutral'"
                    >
                      {{ workbenchPortMutationFeedback(port)?.message }}
                    </UiInlineFeedback>
                    <UiInlineFeedback
                      v-if="workbenchPortSuggestionFeedback(port)"
                      :tone="workbenchPortSuggestionFeedback(port)?.tone || 'neutral'"
                    >
                      {{ workbenchPortSuggestionFeedback(port)?.message }}
                    </UiInlineFeedback>

                    <div
                      v-if="workbenchPortSuggestionResultByKey[port.key]?.suggestions?.length"
                      class="space-y-2"
                    >
                      <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                        Suggested host ports
                      </p>
                      <div class="flex flex-wrap gap-2">
                        <UiButton
                          v-for="suggestion in workbenchPortSuggestionResultByKey[port.key]?.suggestions || []"
                          :key="`${port.key}-suggestion-${suggestion.rank}-${suggestion.hostPort}`"
                          variant="ghost"
                          size="sm"
                          :disabled="
                            workbenchPortMutationBusy(port) ||
                            workbenchOptionalServiceMutationStatus === 'loading' ||
                            workbenchPreviewStatus === 'loading' ||
                            workbenchApplyStatus === 'loading' ||
                            workbenchRestoreStatus === 'loading'
                          "
                          @click="setWorkbenchPortInputValue(port.key, String(suggestion.hostPort))"
                        >
                          #{{ suggestion.rank }} · {{ suggestion.hostPort }}
                        </UiButton>
                      </div>
                    </div>
                  </div>
                  <p v-else class="text-xs text-[color:var(--muted)]">
                    Read-only access: admin permissions are required to re-run the resolver or change stored host-port assignments.
                  </p>
                </UiListRow>
              </div>
            </UiPanel>

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Resolver guidance</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Allocation contract</h3>
              </div>
              <p>
                Auto resolution prefers the compose-declared host port first. If that binding is busy, the resolver walks upward sequentially until it finds an open host port.
              </p>
              <p>
                Conflict means a requested binding is already reserved. Unresolved means the snapshot kept a raw compose host-port expression, and unavailable means no host binding could be assigned from the current candidate range.
              </p>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Assigned</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchPortSummary.assigned }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Conflict</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchPortSummary.conflict }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Unresolved</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchPortSummary.unresolved }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Unavailable</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchPortSummary.unavailable }}</p>
                </UiPanel>
              </div>

              <p class="text-xs text-[color:var(--muted)]">
                Non-admin users stay read-only. Admin users can re-run auto resolution, inspect deterministic suggestions, and pin or clear manual host-port assignments directly from the stored Workbench snapshot.
              </p>
            </UiPanel>
          </div>

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Resources</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Stored service budgets</h3>
                </div>
                <UiBadge :tone="workbenchResourceBadgeTone">
                  {{ workbenchResourceSummary.tracked }} rows / {{ workbenchResourceSummary.total }} services
                </UiBadge>
              </div>

              <UiState v-if="workbenchResourceInventory.length === 0">
                No Workbench services are stored for this snapshot yet.
              </UiState>
              <div v-else class="grid gap-3 md:grid-cols-2">
                <UiPanel
                  v-for="resource in workbenchResourceInventory"
                  :key="resource.key"
                  variant="raise"
                  class="space-y-4 p-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service</p>
                      <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                        {{ resource.serviceName }}
                      </h4>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <UiBadge :tone="resource.tracked ? 'ok' : 'neutral'">
                        {{ resource.tracked ? 'Tracked row' : 'No stored row' }}
                      </UiBadge>
                      <UiBadge :tone="resource.hasLimits ? 'ok' : 'neutral'">
                        {{ resource.hasLimits ? 'Limits set' : 'No limits' }}
                      </UiBadge>
                      <UiBadge :tone="resource.hasReservations ? 'ok' : 'neutral'">
                        {{ resource.hasReservations ? 'Reservations set' : 'No reservations' }}
                      </UiBadge>
                    </div>
                  </div>

                  <div class="grid gap-3 text-xs text-[color:var(--muted)] sm:grid-cols-2">
                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Limits</p>
                        <UiBadge :tone="resource.hasLimits ? 'ok' : 'neutral'">
                          {{ resource.hasLimits ? 'Configured' : 'Empty' }}
                        </UiBadge>
                      </div>
                      <div class="space-y-3">
                        <label class="space-y-2">
                          <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">CPU</span>
                          <UiInput
                            :model-value="workbenchResourceInputValue(resource, 'limitCpus')"
                            type="text"
                            placeholder="0.50 or ${LIMIT_CPUS}"
                            :disabled="!isAdmin || workbenchResourceActionDisabled(resource)"
                            @update:model-value="setWorkbenchResourceInputValue(resource.serviceName, 'limitCpus', $event)"
                          />
                        </label>
                        <div class="flex flex-wrap items-center justify-between gap-2">
                          <span>Current</span>
                          <span class="font-mono text-[color:var(--text)]">
                            {{ resource.limitCpus || 'Not declared' }}
                          </span>
                        </div>
                        <UiButton
                          v-if="isAdmin"
                          variant="ghost"
                          size="sm"
                          :disabled="!resource.limitCpus || workbenchResourceActionDisabled(resource)"
                          @click="clearWorkbenchResourceFields(resource, ['limitCpus'])"
                        >
                          Clear CPU
                        </UiButton>

                        <label class="space-y-2">
                          <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Memory</span>
                          <UiInput
                            :model-value="workbenchResourceInputValue(resource, 'limitMemory')"
                            type="text"
                            placeholder="512M or ${LIMIT_MEMORY}"
                            :disabled="!isAdmin || workbenchResourceActionDisabled(resource)"
                            @update:model-value="setWorkbenchResourceInputValue(resource.serviceName, 'limitMemory', $event)"
                          />
                        </label>
                        <div class="flex flex-wrap items-center justify-between gap-2">
                          <span>Current</span>
                          <span class="font-mono text-[color:var(--text)]">
                            {{ resource.limitMemory || 'Not declared' }}
                          </span>
                        </div>
                        <UiButton
                          v-if="isAdmin"
                          variant="ghost"
                          size="sm"
                          :disabled="!resource.limitMemory || workbenchResourceActionDisabled(resource)"
                          @click="clearWorkbenchResourceFields(resource, ['limitMemory'])"
                        >
                          Clear memory
                        </UiButton>
                      </div>
                    </UiPanel>

                    <UiPanel variant="soft" class="space-y-2 p-3">
                      <div class="flex flex-wrap items-start justify-between gap-2">
                        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                          Reservations
                        </p>
                        <UiBadge :tone="resource.hasReservations ? 'ok' : 'neutral'">
                          {{ resource.hasReservations ? 'Configured' : 'Empty' }}
                        </UiBadge>
                      </div>
                      <div class="space-y-3">
                        <label class="space-y-2">
                          <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">CPU</span>
                          <UiInput
                            :model-value="workbenchResourceInputValue(resource, 'reservationCpus')"
                            type="text"
                            placeholder="0.25 or ${RESERVE_CPUS}"
                            :disabled="!isAdmin || workbenchResourceActionDisabled(resource)"
                            @update:model-value="setWorkbenchResourceInputValue(resource.serviceName, 'reservationCpus', $event)"
                          />
                        </label>
                        <div class="flex flex-wrap items-center justify-between gap-2">
                          <span>Current</span>
                          <span class="font-mono text-[color:var(--text)]">
                            {{ resource.reservationCpus || 'Not declared' }}
                          </span>
                        </div>
                        <UiButton
                          v-if="isAdmin"
                          variant="ghost"
                          size="sm"
                          :disabled="!resource.reservationCpus || workbenchResourceActionDisabled(resource)"
                          @click="clearWorkbenchResourceFields(resource, ['reservationCpus'])"
                        >
                          Clear CPU
                        </UiButton>

                        <label class="space-y-2">
                          <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Memory</span>
                          <UiInput
                            :model-value="workbenchResourceInputValue(resource, 'reservationMemory')"
                            type="text"
                            placeholder="256M or ${RESERVE_MEMORY}"
                            :disabled="!isAdmin || workbenchResourceActionDisabled(resource)"
                            @update:model-value="setWorkbenchResourceInputValue(resource.serviceName, 'reservationMemory', $event)"
                          />
                        </label>
                        <div class="flex flex-wrap items-center justify-between gap-2">
                          <span>Current</span>
                          <span class="font-mono text-[color:var(--text)]">
                            {{ resource.reservationMemory || 'Not declared' }}
                          </span>
                        </div>
                        <UiButton
                          v-if="isAdmin"
                          variant="ghost"
                          size="sm"
                          :disabled="!resource.reservationMemory || workbenchResourceActionDisabled(resource)"
                          @click="clearWorkbenchResourceFields(resource, ['reservationMemory'])"
                        >
                          Clear memory
                        </UiButton>
                      </div>
                    </UiPanel>
                  </div>

                  <div
                    v-if="isAdmin"
                    class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/45 p-3"
                  >
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                          Edit stored values
                        </p>
                        <p class="mt-1 text-xs text-[color:var(--muted)]">
                          Save one or more changed CPU or memory values, or clear a stored field to remove it from the persisted Workbench snapshot.
                        </p>
                      </div>
                      <UiBadge :tone="resource.tracked ? 'ok' : 'neutral'">
                        {{ resource.tracked ? 'Mutable row ready' : 'Creates row on save' }}
                      </UiBadge>
                    </div>

                    <div class="flex flex-wrap gap-2">
                      <UiButton
                        variant="primary"
                        size="sm"
                        :disabled="workbenchResourceActionDisabled(resource)"
                        @click="saveWorkbenchResource(resource)"
                      >
                        <span class="inline-flex items-center gap-2">
                          <UiInlineSpinner v-if="workbenchResourceMutationBusy(resource)" />
                          Save values
                        </span>
                      </UiButton>
                      <UiButton
                        variant="ghost"
                        size="sm"
                        :disabled="workbenchResourceActionDisabled(resource)"
                        @click="resetWorkbenchResourceInputs(resource)"
                      >
                        Reset form
                      </UiButton>
                      <UiButton
                        variant="ghost"
                        size="sm"
                        :disabled="
                          workbenchResourceActionDisabled(resource) ||
                          (!resource.hasLimits && !resource.hasReservations)
                        "
                        @click="clearWorkbenchResourceFields(resource, ['limitCpus', 'limitMemory', 'reservationCpus', 'reservationMemory'])"
                      >
                        Clear stored values
                      </UiButton>
                    </div>

                    <UiInlineFeedback
                      v-if="workbenchResourceMutationFeedback(resource)"
                      :tone="workbenchResourceMutationFeedback(resource)?.tone || 'neutral'"
                    >
                      {{ workbenchResourceMutationFeedback(resource)?.message }}
                    </UiInlineFeedback>
                  </div>
                  <p v-else class="text-xs text-[color:var(--muted)]">
                    Read-only access: admin permissions are required to change stored CPU and memory values.
                  </p>
                </UiPanel>
              </div>
            </UiPanel>

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Resource posture</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Constraint summary</h3>
              </div>
              <p>
                Workbench stores imported CPU and memory values per service. Admins can now set or clear stored values directly from this section while non-admin users remain read-only.
              </p>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Services</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchResourceSummary.total }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Tracked rows</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchResourceSummary.tracked }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">With limits</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchResourceSummary.withLimits }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">With reservations</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchResourceSummary.withReservations }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Unconstrained</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchResourceSummary.unconstrained }}
                  </p>
                </UiPanel>
              </div>

              <p class="text-xs text-[color:var(--muted)]">
                Values round-trip against the stored snapshot immediately after each successful edit. Generate a compose preview below to inspect the resulting YAML before apply.
              </p>
            </UiPanel>
          </div>

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Compose preview</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Generate and apply</h3>
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
                </div>
              </div>

              <p class="text-sm text-[color:var(--muted)]">
                Preview is mandatory before apply. This is where catalog-managed service changes, port edits, and resource edits turn into generated compose output; stack restart and job handoff remain on the existing project controls.
              </p>

              <div v-if="isAdmin" class="flex flex-wrap gap-2">
                <UiButton
                  variant="ghost"
                  size="sm"
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
                  :disabled="!workbenchLastPreviewResult?.compose || workbenchComposeActionBusy"
                  @click="copyWorkbenchPreviewCompose"
                >
                  Copy preview
                </UiButton>
                <UiButton
                  variant="primary"
                  size="sm"
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
                class="space-y-3 border border-[color:var(--line)] p-3 text-sm text-[color:var(--muted)]"
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
                  class="grid gap-2 sm:grid-cols-2"
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

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Apply gate</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Revision and diagnostics</h3>
              </div>
              <p>
                Port, resource, and catalog-managed service edits all change the stored Workbench model first. Any model change, stale revision, or external compose drift invalidates the previous preview until a new preview is generated.
              </p>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
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
                <UiPanel variant="soft" class="space-y-2 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Current fingerprint</p>
                  <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
                    {{ workbenchFingerprintLabel }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-2 p-3">
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
                Non-admin users can inspect stored warnings and diagnostics, but preview/apply stays admin-only.
              </p>

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
                <div v-else class="max-h-[16rem] space-y-2 overflow-auto pr-1">
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
            </UiPanel>
          </div>

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
            <UiPanel variant="soft" class="space-y-4 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Topology</p>
                  <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Service relationships</h3>
                </div>
                <UiBadge :tone="workbenchTopologyBadgeTone">
                  {{ workbenchTopologyInventory.length }} visible
                </UiBadge>
              </div>

              <UiState v-if="workbenchTopologyInventory.length === 0">
                No Workbench topology rows are stored for this snapshot yet.
              </UiState>
              <div v-else class="space-y-3">
                <UiListRow
                  v-for="row in workbenchTopologyInventory"
                  :key="row.key"
                  as="article"
                  class="space-y-4"
                >
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service</p>
                      <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">{{ row.serviceName }}</h4>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <UiBadge :tone="row.dependsOn.length > 0 || row.dependedBy.length > 0 ? 'ok' : 'neutral'">
                        {{ row.dependsOn.length + row.dependedBy.length > 0 ? 'Connected' : 'Isolated' }}
                      </UiBadge>
                      <UiBadge :tone="row.networkNames.length > 0 ? 'ok' : 'neutral'">
                        {{ row.networkNames.length > 0 ? `${row.networkNames.length} networks` : 'No networks' }}
                      </UiBadge>
                    </div>
                  </div>

                  <div class="grid gap-3 text-xs text-[color:var(--muted)] lg:grid-cols-2">
                    <div class="space-y-2">
                      <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Depends on</p>
                      <div v-if="row.dependsOn.length > 0" class="flex flex-wrap gap-2">
                        <UiBadge
                          v-for="dependency in row.dependsOn"
                          :key="`${row.serviceName}-depends-on-${dependency}`"
                          tone="neutral"
                        >
                          {{ dependency }}
                        </UiBadge>
                      </div>
                      <p v-else>No upstream dependencies recorded.</p>
                    </div>

                    <div class="space-y-2">
                      <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Required by</p>
                      <div v-if="row.dependedBy.length > 0" class="flex flex-wrap gap-2">
                        <UiBadge
                          v-for="dependent in row.dependedBy"
                          :key="`${row.serviceName}-required-by-${dependent}`"
                          tone="neutral"
                        >
                          {{ dependent }}
                        </UiBadge>
                      </div>
                      <p v-else>No downstream dependents recorded.</p>
                    </div>

                    <div class="space-y-2">
                      <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Networks</p>
                      <div v-if="row.networkNames.length > 0" class="flex flex-wrap gap-2">
                        <UiBadge
                          v-for="networkName in row.networkNames"
                          :key="`${row.serviceName}-network-${networkName}`"
                          tone="neutral"
                        >
                          {{ networkName }}
                        </UiBadge>
                      </div>
                      <p v-else>No network attachments recorded.</p>
                    </div>

                    <div class="space-y-2">
                      <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Legacy metadata</p>
                      <div v-if="row.moduleTypes.length > 0" class="flex flex-wrap gap-2">
                        <UiBadge
                          v-for="moduleType in row.moduleTypes"
                          :key="`${row.serviceName}-module-${moduleType}`"
                          tone="ok"
                        >
                          {{ moduleType }}
                        </UiBadge>
                      </div>
                      <p v-else>No legacy transition metadata recorded.</p>
                    </div>
                  </div>
                </UiListRow>
              </div>
            </UiPanel>

            <UiPanel variant="raise" class="space-y-4 p-4 text-sm text-[color:var(--muted)]">
              <div>
                <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Topology summary</p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Dependency footprint</h3>
              </div>
              <p>
                This view mirrors the stored dependency graph only. It exposes imported service edges plus stored network attachments and legacy transition annotations without widening into edit controls.
              </p>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Services</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">{{ workbenchTopologySummary.services }}</p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Dependency edges</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchTopologySummary.dependencyEdges }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Connected</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchTopologySummary.connectedServices }}
                  </p>
                </UiPanel>
                <UiPanel variant="soft" class="space-y-1 p-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Isolated</p>
                  <p class="text-lg font-semibold text-[color:var(--text)]">
                    {{ workbenchTopologySummary.isolatedServices }}
                  </p>
                </UiPanel>
              </div>

              <div class="space-y-3">
                <div class="space-y-2">
                  <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Networks</p>
                  <div v-if="workbenchTopologySummary.networks.length > 0" class="flex flex-wrap gap-2">
                    <UiBadge
                      v-for="networkName in workbenchTopologySummary.networks"
                      :key="`topology-network-${networkName}`"
                      tone="neutral"
                    >
                      {{ networkName }}
                    </UiBadge>
                  </div>
                  <p v-else class="text-xs">No stored network summary is available in this snapshot.</p>
                </div>

                <div class="space-y-2">
                  <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Legacy metadata types</p>
                  <div v-if="workbenchTopologySummary.moduleTypes.length > 0" class="flex flex-wrap gap-2">
                    <UiBadge
                      v-for="moduleType in workbenchTopologySummary.moduleTypes"
                      :key="`topology-module-${moduleType}`"
                      tone="ok"
                    >
                      {{ moduleType }}
                    </UiBadge>
                  </div>
                  <p v-else class="text-xs">No legacy transition summary is available in this snapshot.</p>
                </div>
              </div>

              <p class="text-xs text-[color:var(--muted)]">
                Topology remains read-only. Use the preview/apply and restore surfaces above to move between generated compose output and retained backups without leaving Project Detail.
              </p>
            </UiPanel>
          </div>
          </template>
        </template>
      </UiPanel>

      <UiPanel class="space-y-5 p-6">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Runtime units ({{ detail.containers.length }})</h2>
        </div>
        <UiState v-if="detail.containers.length === 0">No containers currently match this compose project label.</UiState>
        <div v-else class="grid gap-4 xl:grid-cols-2">
          <UiListRow
            v-for="container in detail.containers"
            :key="container.id"
            as="article"
            class="space-y-4"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ container.service || 'Container' }}
                </p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">{{ container.name }}</h3>
                <p class="mt-1 font-mono text-[11px] text-[color:var(--muted-2)]">{{ container.id }}</p>
              </div>
              <UiBadge :tone="containerTone(container)">{{ container.status || 'unknown' }}</UiBadge>
            </div>
            <div class="space-y-2 text-xs text-[color:var(--muted)]">
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Image</span>
                <span class="text-[color:var(--text)] break-all">{{ container.image }}</span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Ports</span>
                <span class="text-[color:var(--text)]">{{ container.ports || '—' }}</span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Service</span>
                <span class="text-[color:var(--text)]">{{ container.service || '—' }}</span>
              </div>
            </div>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel class="space-y-5 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Archive cleanup</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Plan and execute</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Cleanup is asynchronous and always queued as a project job.
            </p>
          </div>
          <UiButton variant="ghost" size="sm" :disabled="archivePlanLoading" @click="loadArchivePlan">
            <span class="inline-flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="archivePlanLoading" />
              Refresh plan
            </span>
          </UiButton>
        </div>

        <UiState v-if="!isAdmin">
          Read-only access: admin permissions are required to preview and execute archive cleanup.
        </UiState>
        <UiState v-else-if="archivePlanLoading" loading>Building archive cleanup plan...</UiState>
        <UiState v-else-if="archivePlanError" tone="error">{{ archivePlanError }}</UiState>

        <template v-else-if="archivePlan">
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.containers.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.hostnames.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress rules</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.ingressRules.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS records</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">
                {{ archivePlan.dnsRecords.filter((record) => record.deleteEligible).length }}/{{ archivePlan.dnsRecords.length }}
              </p>
            </UiPanel>
          </div>

          <UiInlineFeedback v-if="archivePlan.warnings.length > 0" tone="warn">
            {{ archivePlan.warnings.length }} warning(s): {{ archivePlan.warnings.join(' | ') }}
          </UiInlineFeedback>
          <UiInlineFeedback v-if="archiveExecutedWithWarnings" tone="warn">
            Last archive request was queued with warnings in the plan preview. Review job logs after completion.
          </UiInlineFeedback>
          <UiInlineFeedback v-if="archiveExecuteError" tone="error">
            {{ archiveExecuteError }}
          </UiInlineFeedback>

          <div class="grid gap-4 xl:grid-cols-2">
            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <UiState v-if="archivePlan.hostnames.length === 0">No hostnames discovered.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="hostname in archivePlan.hostnames"
                  :key="hostname"
                  class="font-mono text-[color:var(--text)] break-all"
                >
                  {{ hostname }}
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Container targets</p>
              <UiState v-if="archivePlan.containers.length === 0">No project containers found.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="container in archivePlan.containers"
                  :key="container.id || container.name"
                  class="flex flex-wrap items-center justify-between gap-2"
                >
                  <span class="font-mono text-[color:var(--text)]">{{ container.name }}</span>
                  <UiBadge :tone="statusTone(container.status)">
                    {{ container.status || 'unknown' }}
                  </UiBadge>
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress targets</p>
              <UiState v-if="archivePlan.ingressRules.length === 0">No ingress rules matched.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="rule in archivePlan.ingressRules"
                  :key="`${rule.source}-${rule.hostname}-${rule.service}`"
                  class="space-y-1"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <span class="font-mono text-[color:var(--text)] break-all">{{ rule.hostname }}</span>
                    <UiBadge :tone="rule.source === 'remote' ? 'ok' : 'neutral'">
                      {{ rule.source }}
                    </UiBadge>
                  </div>
                  <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">{{ rule.service || 'service not set' }}</p>
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS targets</p>
              <UiState v-if="archivePlan.dnsRecords.length === 0">No DNS records matched.</UiState>
              <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
                <li
                  v-for="record in archivePlan.dnsRecords"
                  :key="`${record.zoneId}-${record.id}-${record.name}`"
                  class="space-y-1"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <span class="font-mono text-[color:var(--text)] break-all">{{ record.name }}</span>
                    <UiBadge :tone="record.deleteEligible ? 'ok' : 'warn'">
                      {{ record.deleteEligible ? 'deletable' : 'skip' }}
                    </UiBadge>
                  </div>
                  <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                    {{ record.type }} → {{ record.content }}
                  </p>
                  <p v-if="record.skipReason" class="text-[11px] text-[color:var(--muted)]">
                    {{ record.skipReason }}
                  </p>
                </li>
              </ul>
            </UiPanel>
          </div>

          <UiPanel variant="raise" class="space-y-4 p-5">
            <div>
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Execution</p>
              <p class="mt-2 text-sm text-[color:var(--muted)]">
                Confirmation phrase: <span class="font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
              </p>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <UiToggle v-model="archiveOptions.removeContainers" :disabled="!isAdmin">
                Remove project containers
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeVolumes" :disabled="!isAdmin || !archiveOptions.removeContainers">
                Remove container volumes
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeIngress" :disabled="!isAdmin">
                Remove ingress rules
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeDns" :disabled="!isAdmin">
                Remove DNS records
              </UiToggle>
            </div>

            <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Confirmation phrase
            </label>
            <UiInput
              v-model="archiveConfirmInput"
              :disabled="!isAdmin || archiveExecuting"
              autocomplete="off"
              spellcheck="false"
              placeholder="Type the phrase exactly"
            />

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                variant="danger"
                size="sm"
                :disabled="!canSubmitArchive"
                @click="queueArchive"
              >
                <span class="inline-flex items-center gap-2">
                  <UiInlineSpinner v-if="archiveExecuting" />
                  {{ archiveExecuting ? 'Queueing archive...' : 'Queue archive job' }}
                </span>
              </UiButton>
              <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
                Read-only access: admin permissions are required to queue archive cleanup.
              </p>
            </div>
          </UiPanel>
        </template>
      </UiPanel>

      <UiPanel class="space-y-5 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project jobs</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Activity timeline</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              {{ jobsTotal }} total jobs for {{ detail.project.normalizedName }}.
            </p>
          </div>
          <UiButton variant="ghost" size="sm" :disabled="jobsLoading" @click="loadProjectJobs(jobsPage)">
            <span class="inline-flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="jobsLoading" />
              Refresh jobs
            </span>
          </UiButton>
        </div>

        <UiState v-if="jobsError" tone="error">{{ jobsError }}</UiState>
        <UiState v-else-if="jobsLoading" loading>Loading project jobs...</UiState>
        <UiState v-else-if="projectJobs.length === 0">No jobs have been recorded for this project yet.</UiState>

        <div v-else class="space-y-3">
          <UiListRow
            v-for="job in projectJobs"
            :key="job.id"
            as="article"
            class="space-y-4"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Job #{{ job.id }}</p>
                <h3 class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ job.type }}</h3>
              </div>
              <UiBadge :tone="jobStatusTone(job.status)">
                {{ jobStatusLabel(job.status) }}
              </UiBadge>
            </div>

            <div class="mt-4 grid gap-2 text-xs text-[color:var(--muted)] sm:grid-cols-3">
              <p>Created: <span class="text-[color:var(--text)]">{{ fmtDate(job.createdAt) }}</span></p>
              <p>Started: <span class="text-[color:var(--text)]">{{ fmtDate(job.startedAt) }}</span></p>
              <p>Finished: <span class="text-[color:var(--text)]">{{ fmtDate(job.finishedAt) }}</span></p>
            </div>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <UiButton variant="ghost" size="sm" @click="openJobLogs(job.id)">
                View job logs
              </UiButton>
              <UiButton :as="RouterLink" :to="`/jobs/${job.id}`" variant="ghost" size="sm">
                Open job page
              </UiButton>
            </div>
          </UiListRow>
        </div>

        <div
          v-if="jobsTotalPages > 1 && !jobsLoading"
          class="flex flex-wrap items-center justify-between gap-3 bg-[color:var(--surface-2)] px-4 py-3 text-xs text-[color:var(--muted)]"
        >
          <span>Page {{ jobsPage }} of {{ jobsTotalPages }}</span>
          <div class="flex items-center gap-2">
            <UiButton variant="ghost" size="sm" :disabled="!canGoJobsBack" @click="goToJobsPage(jobsPage - 1)">
              Previous
            </UiButton>
            <UiButton variant="ghost" size="sm" :disabled="!canGoJobsForward" @click="goToJobsPage(jobsPage + 1)">
              Next
            </UiButton>
          </div>
        </div>
      </UiPanel>
    </template>

    <UiFormSidePanel
      v-model="jobLogsPanelOpen"
      eyebrow="Project jobs"
      :title="selectedJobId ? `Job #${selectedJobId} logs` : 'Job logs'"
    >
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Log viewer</p>
            <p class="mt-1 text-sm text-[color:var(--muted)]">
              {{ selectedJob ? selectedJob.type : 'Select a job entry to load logs.' }}
            </p>
          </div>
          <UiBadge v-if="selectedJob" :tone="jobStatusTone(selectedJob.status)">
            {{ jobStatusLabel(selectedJob.status) }}
          </UiBadge>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <UiButton variant="ghost" size="sm" :disabled="selectedJobLoading" @click="refreshSelectedJobLogs">
            <span class="inline-flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="selectedJobLoading" />
              Refresh
            </span>
          </UiButton>
          <UiButton variant="ghost" size="sm" :disabled="!selectedJobLogOutput" @click="copySelectedJobLogs">
            Copy to clipboard
          </UiButton>
          <UiButton variant="ghost" size="sm" @click="cycleProjectJobLogFontSize">
            Log size: {{ projectJobLogFontSize }}px
          </UiButton>
        </div>

        <UiState v-if="selectedJobError" tone="error">{{ selectedJobError }}</UiState>
        <UiState v-else-if="selectedJobLoading && !selectedJob" loading>Loading job logs...</UiState>

        <pre
          v-else
          class="max-h-[70vh] overflow-auto bg-[color:var(--surface-2)] p-4 text-[color:var(--text)]"
          :style="{ fontSize: `${projectJobLogFontSize}px`, lineHeight: '1.45' }"
        ><code>{{ selectedJobLogOutput || 'No logs yet. Try refresh if the job is still running.' }}</code></pre>
      </div>
    </UiFormSidePanel>
  </section>
</template>
