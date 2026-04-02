<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { apiErrorMessage } from '@/services/api'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import type { Job } from '@/types/jobs'
import type {
  ProjectArchiveOptions,
  ProjectArchivePlan,
  ProjectArchivePlanServiceExposure,
} from '@/types/projects'

const props = defineProps<{
  projectName: string
  projectDisplayName: string
  isAdmin: boolean
  queuedJob: Job | null
  queuedWarningCount: number
}>()

const emit = defineEmits<{
  queued: [{ job: Job; warningCount: number }]
  showActivity: []
}>()

const toastStore = useToastStore()

const archivePlan = ref<ProjectArchivePlan | null>(null)
const archivePlanLoading = ref(false)
const archivePlanError = ref<string | null>(null)
const archiveExecuting = ref(false)
const archiveExecuteError = ref<string | null>(null)
const archiveConfirmInput = ref('')
const archiveReviewModalOpen = ref(false)
const archiveOptions = ref<ProjectArchiveOptions>({
  removeContainers: true,
  removeVolumes: false,
  removeIngress: true,
  removeDns: true,
})

const archiveConfirmationPhrase = computed(() => {
  const normalized = (props.projectDisplayName || props.projectName || '').toLowerCase().trim()
  if (!normalized) return 'ARCHIVE PROJECT'
  return `ARCHIVE ${normalized}`
})

const archivePlanWarnings = computed(() => archivePlan.value?.warnings ?? [])

const deletableDnsRecordsCount = computed(() =>
  archivePlan.value?.dnsRecords.filter((record) => record.deleteEligible).length ?? 0,
)

const queuedContainerCleanupCount = computed(() =>
  archiveOptions.value.removeContainers ? (archivePlan.value?.containers.length ?? 0) : 0,
)

const queuedIngressCleanupCount = computed(() =>
  archiveOptions.value.removeIngress ? (archivePlan.value?.ingressRules.length ?? 0) : 0,
)

const queuedDnsCleanupCount = computed(() =>
  archiveOptions.value.removeDns ? deletableDnsRecordsCount.value : 0,
)

const archiveQueuedJobRoute = computed(() =>
  props.queuedJob ? `/jobs/${props.queuedJob.id}` : '',
)

const archiveExecutionBlockedReason = computed(() => {
  if (!props.isAdmin) return 'Admin or superuser access is required to queue archive cleanup.'
  if (!archivePlan.value) return 'Refresh the archive preview before queueing cleanup.'
  if (archiveOptions.value.removeVolumes && !archiveOptions.value.removeContainers) {
    return 'Volume deletion requires container cleanup to remain enabled.'
  }
  return null
})

const canOpenArchiveReview = computed(() => !archiveExecuting.value && !archiveExecutionBlockedReason.value)

const canSubmitArchive = computed(() => {
  if (!archiveReviewModalOpen.value || !props.isAdmin || archiveExecuting.value) return false
  if (archiveExecutionBlockedReason.value) return false
  return archiveConfirmInput.value.trim() === archiveConfirmationPhrase.value
})

const archiveScopeRows = computed(() => [
  {
    key: 'containers',
    label: 'Project containers',
    enabled: archiveOptions.value.removeContainers,
    queuedCount: queuedContainerCleanupCount.value,
    totalCount: archivePlan.value?.containers.length ?? 0,
    actionLabel: archiveOptions.value.removeContainers ? 'Remove queued' : 'Keep queued',
    detail: archiveOptions.value.removeContainers
      ? 'Queued archive will remove the managed project containers resolved by the preview.'
      : 'Container cleanup stays out of scope for the queued archive job.',
  },
  {
    key: 'volumes',
    label: 'Container volumes',
    enabled: archiveOptions.value.removeVolumes,
    queuedCount: archiveOptions.value.removeVolumes ? queuedContainerCleanupCount.value : 0,
    totalCount: archivePlan.value?.containers.length ?? 0,
    actionLabel: archiveOptions.value.removeVolumes ? 'Delete queued' : 'Keep queued',
    detail: archiveOptions.value.removeVolumes
      ? 'Attached volumes will be deleted together with the selected managed containers.'
      : 'Volume deletion stays disabled unless container cleanup is enabled.',
  },
  {
    key: 'ingress',
    label: 'Ingress rules',
    enabled: archiveOptions.value.removeIngress,
    queuedCount: queuedIngressCleanupCount.value,
    totalCount: archivePlan.value?.ingressRules.length ?? 0,
    actionLabel: archiveOptions.value.removeIngress ? 'Remove queued' : 'Keep queued',
    detail: archiveOptions.value.removeIngress
      ? 'Queued archive will remove only the exact ingress rule targets authored by the preview.'
      : 'Ingress cleanup stays out of scope for the queued archive job.',
  },
  {
    key: 'dns',
    label: 'DNS records',
    enabled: archiveOptions.value.removeDns,
    queuedCount: queuedDnsCleanupCount.value,
    totalCount: archivePlan.value?.dnsRecords.length ?? 0,
    actionLabel: archiveOptions.value.removeDns ? 'Delete queued' : 'Keep queued',
    detail: archiveOptions.value.removeDns
      ? 'Queued archive will delete only matching tunnel-backed CNAME records marked as eligible in the preview.'
      : 'DNS cleanup stays out of scope for the queued archive job.',
  },
])

const deletableDnsHostnames = computed(() => {
  const hostnames = new Set<string>()
  for (const record of archivePlan.value?.dnsRecords ?? []) {
    if (!record.deleteEligible) continue
    const hostname = record.name.trim().toLowerCase()
    if (!hostname) continue
    hostnames.add(hostname)
  }
  return hostnames
})

function exposureHasHostname(exposure: ProjectArchivePlanServiceExposure) {
  return exposure.hostname.trim().length > 0
}

function exposureHasContainer(exposure: ProjectArchivePlanServiceExposure) {
  return (exposure.container ?? '').trim().length > 0
}

function exposureHasDnsTarget(exposure: ProjectArchivePlanServiceExposure) {
  if (!exposureHasHostname(exposure)) return false
  return deletableDnsHostnames.value.has(exposure.hostname.trim().toLowerCase())
}

function joinCleanupPhrases(parts: string[]) {
  if (parts.length === 0) return ''
  if (parts.length === 1) return parts[0]
  if (parts.length === 2) return `${parts[0]} and ${parts[1]}`
  return `${parts.slice(0, -1).join(', ')}, and ${parts[parts.length - 1]}`
}

function plannedExposureCleanupBadges(exposure: ProjectArchivePlanServiceExposure) {
  const badges: Array<{ key: string; label: string; tone: 'ok' | 'neutral' }> = []

  if (exposureHasContainer(exposure)) {
    badges.push({
      key: 'container',
      label: archiveOptions.value.removeContainers ? 'remove container' : 'keep container',
      tone: archiveOptions.value.removeContainers ? 'ok' : 'neutral',
    })
  } else {
    badges.push({ key: 'container', label: 'no container', tone: 'neutral' })
  }

  if (exposureHasHostname(exposure)) {
    badges.push({
      key: 'ingress',
      label: archiveOptions.value.removeIngress ? 'remove ingress' : 'keep ingress',
      tone: archiveOptions.value.removeIngress ? 'ok' : 'neutral',
    })
  } else {
    badges.push({ key: 'ingress', label: 'no ingress', tone: 'neutral' })
  }

  if (exposureHasDnsTarget(exposure)) {
    badges.push({
      key: 'dns',
      label: archiveOptions.value.removeDns ? 'remove dns' : 'keep dns',
      tone: archiveOptions.value.removeDns ? 'ok' : 'neutral',
    })
  } else {
    badges.push({
      key: 'dns',
      label: exposureHasHostname(exposure) ? 'no dns match' : 'no dns',
      tone: 'neutral',
    })
  }

  return badges
}

function describeExposureCleanup(exposure: ProjectArchivePlanServiceExposure) {
  const summary =
    exposure.type === 'forward_local'
      ? 'Hostname-owned cleanup only.'
      : !exposureHasHostname(exposure)
        ? 'Internal-only quick_service.'
        : !exposureHasContainer(exposure)
          ? 'Host-published quick_service with unresolved container ownership.'
          : 'Host-published quick_service.'

  const plannedActions: string[] = []
  const keptActions: string[] = []
  const notes: string[] = []

  if (exposureHasContainer(exposure)) {
    if (archiveOptions.value.removeContainers) {
      plannedActions.push('remove the managed container')
    } else {
      keptActions.push('keep the managed container')
    }
  } else if (exposure.type === 'forward_local') {
    notes.push('No service-exposure container is tracked.')
  } else {
    notes.push('No managed container is tracked for this exposure.')
  }

  if (exposureHasHostname(exposure)) {
    if (archiveOptions.value.removeIngress) {
      plannedActions.push('remove ingress rules')
    } else {
      keptActions.push('keep ingress rules')
    }
  } else {
    notes.push('Ingress and DNS cleanup stay out of scope because no managed hostname was resolved.')
  }

  if (exposureHasDnsTarget(exposure)) {
    if (archiveOptions.value.removeDns) {
      plannedActions.push('remove matching tunnel CNAME records')
    } else {
      keptActions.push('keep matching tunnel CNAME records')
    }
  } else if (exposureHasHostname(exposure)) {
    notes.push('No matching tunnel CNAME records were resolved.')
  }

  const details = [summary]
  if (plannedActions.length > 0) {
    details.push(`Queued archive will ${joinCleanupPhrases(plannedActions)}.`)
  }
  if (keptActions.length > 0) {
    details.push(`Selected toggles will ${joinCleanupPhrases(keptActions)}.`)
  }
  details.push(...notes)
  return details.join(' ')
}

function describeDnsRecordAction(deleteEligible: boolean) {
  if (!deleteEligible) return 'skip'
  return archiveOptions.value.removeDns ? 'delete' : 'keep'
}

function dnsRecordTone(deleteEligible: boolean) {
  if (!deleteEligible) return 'warn'
  return archiveOptions.value.removeDns ? 'ok' : 'neutral'
}

function queuedDnsTargetsSummary() {
  return `${queuedDnsCleanupCount.value} queued / ${deletableDnsRecordsCount.value} deletable / ${archivePlan.value?.dnsRecords.length ?? 0} total`
}

function planSummaryValue(queuedCount: number, totalCount: number) {
  return `${queuedCount}/${totalCount}`
}

function archiveScopeTone(enabled: boolean) {
  return enabled ? 'warn' : 'neutral'
}

function openArchiveReview() {
  if (archiveExecutionBlockedReason.value) {
    archiveExecuteError.value = archiveExecutionBlockedReason.value
    return
  }
  archiveExecuteError.value = null
  archiveConfirmInput.value = ''
  archiveReviewModalOpen.value = true
}

function applyArchiveDefaults(plan: ProjectArchivePlan) {
  archiveOptions.value = {
    removeContainers: plan.defaults.removeContainers,
    removeVolumes: plan.defaults.removeVolumes,
    removeIngress: plan.defaults.removeIngress,
    removeDns: plan.defaults.removeDns,
  }
}

async function loadArchivePlan() {
  if (!props.projectName) {
    archivePlan.value = null
    archivePlanError.value = 'Invalid project name.'
    return
  }
  if (!props.isAdmin) {
    archivePlan.value = null
    archivePlanError.value = null
    return
  }

  archivePlanLoading.value = true
  archivePlanError.value = null
  try {
    const { data } = await projectsApi.getArchivePlan(props.projectName)
    archivePlan.value = data.plan
    applyArchiveDefaults(data.plan)
  } catch (err) {
    archivePlan.value = null
    archivePlanError.value = apiErrorMessage(err)
  } finally {
    archivePlanLoading.value = false
  }
}

async function queueArchive() {
  if (!props.projectName) return
  if (!props.isAdmin) {
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
    const { data } = await projectsApi.archiveProject(props.projectName, payload)
    archivePlan.value = data.plan
    const warningCount = data.plan.warnings.length
    archiveConfirmInput.value = ''
    archiveReviewModalOpen.value = false

    if (warningCount > 0) {
      toastStore.warn(
        `Archive queued (job #${data.job.id}) with ${data.plan.warnings.length} warning(s) in plan preview.`,
        'Archive queued',
      )
    } else {
      toastStore.success(`Archive queued (job #${data.job.id}).`, 'Project cleanup')
    }

    emit('queued', { job: data.job, warningCount })
  } catch (err) {
    const message = apiErrorMessage(err)
    archiveExecuteError.value = message
    toastStore.error(message, 'Archive queue failed')
  } finally {
    archiveExecuting.value = false
  }
}

watch(
  () => archiveOptions.value.removeContainers,
  (enabled) => {
    if (enabled) return
    archiveOptions.value.removeVolumes = false
  },
)

watch(
  () => [props.projectName, props.isAdmin],
  () => {
    archivePlan.value = null
    archivePlanError.value = null
    archiveExecuteError.value = null
    archiveExecuting.value = false
    archiveConfirmInput.value = ''
    archiveReviewModalOpen.value = false
    void loadArchivePlan()
  },
  { immediate: true },
)

watch(archiveReviewModalOpen, (open) => {
  if (open) return
  archiveConfirmInput.value = ''
})
</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project lifecycle</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Archive</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Review the backend-authored dry-run preview first, then queue the existing asynchronous archive job only
          after an explicit privileged confirmation step.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" :disabled="archivePlanLoading || !isAdmin" @click="loadArchivePlan">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="archivePlanLoading" />
          Refresh preview
        </span>
      </UiButton>
    </div>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access is active. Admin and superuser accounts can inspect the live archive preview and queue the
      destructive cleanup job; this role cannot reach that execution path from Project Detail.
    </UiState>
    <UiState v-else-if="archivePlanLoading" loading>Building archive cleanup plan...</UiState>
    <UiState v-else-if="archivePlanError" tone="error">{{ archivePlanError }}</UiState>

    <template v-else-if="archivePlan">
      <UiPanel variant="raise" class="space-y-4 p-5">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="max-w-3xl">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Privileged cleanup panel</p>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              The preview below is authoritative cleanup scope. Toggle the existing archive options here, then use the
              confirmation step to queue the shipped async archive workflow without creating a second execution path.
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <UiBadge :tone="archivePlanWarnings.length > 0 ? 'warn' : 'ok'">
              {{ archivePlanWarnings.length }} warning{{ archivePlanWarnings.length === 1 ? '' : 's' }}
            </UiBadge>
            <UiBadge :tone="queuedJob ? 'ok' : 'neutral'">
              {{ queuedJob ? `Last queued: job #${queuedJob.id}` : 'No queued archive yet' }}
            </UiBadge>
          </div>
        </div>

        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <UiToggle v-model="archiveOptions.removeContainers" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove project containers
          </UiToggle>
          <UiToggle
            v-model="archiveOptions.removeVolumes"
            :disabled="!isAdmin || !archiveOptions.removeContainers"
            class="min-w-[240px] flex-1"
          >
            Remove container volumes
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeIngress" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove ingress rules
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeDns" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove DNS records
          </UiToggle>
        </div>

        <div class="grid gap-3 xl:grid-cols-2">
          <UiPanel
            v-for="scope in archiveScopeRows"
            :key="scope.key"
            variant="soft"
            class="space-y-2 p-3"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">{{ scope.label }}</p>
              <UiBadge :tone="archiveScopeTone(scope.enabled)">{{ scope.actionLabel }}</UiBadge>
            </div>
            <p class="text-lg font-semibold text-[color:var(--text)]">
              {{ planSummaryValue(scope.queuedCount, scope.totalCount) }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">{{ scope.detail }}</p>
          </UiPanel>
        </div>

        <UiInlineFeedback v-if="archiveExecutionBlockedReason" tone="warn">
          {{ archiveExecutionBlockedReason }}
        </UiInlineFeedback>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-[color:var(--border-soft)] pt-4">
          <div class="space-y-1 text-sm text-[color:var(--muted)]">
            <p>
              Confirmation phrase:
              <span class="ml-1 font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
            </p>
            <p>
              Queue feedback stays in the existing Jobs flow. Success returns the job ID here; follow-up belongs on the
              job page or in the project activity timeline.
            </p>
          </div>
          <UiButton variant="danger" size="sm" :disabled="!canOpenArchiveReview" @click="openArchiveReview">
            Review and confirm archive cleanup
          </UiButton>
        </div>
      </UiPanel>

      <UiInlineFeedback v-if="archivePlanWarnings.length > 0" tone="warn">
        {{ archivePlanWarnings.length }} warning(s): {{ archivePlanWarnings.join(' | ') }}
      </UiInlineFeedback>
      <UiPanel v-if="queuedJob" variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Jobs handoff</p>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Archive cleanup was queued as job #{{ queuedJob.id }}. Track execution in the existing Jobs flow;
              the archive tab does not run cleanup inline.
            </p>
          </div>
          <UiBadge :tone="queuedWarningCount > 0 ? 'warn' : 'ok'">
            {{ queuedWarningCount > 0 ? `${queuedWarningCount} warning(s) in preview` : 'Queued cleanly' }}
          </UiBadge>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton
            v-if="archiveQueuedJobRoute"
            :as="RouterLink"
            :to="archiveQueuedJobRoute"
            variant="ghost"
            size="sm"
          >
            Open job page
          </UiButton>
          <UiButton variant="ghost" size="sm" @click="emit('showActivity')">
            Open activity timeline
          </UiButton>
          <UiButton :as="RouterLink" to="/jobs" variant="ghost" size="sm">
            Open Jobs
          </UiButton>
        </div>
      </UiPanel>
      <UiInlineFeedback v-if="queuedWarningCount > 0" tone="warn">
        Last archive request was queued with warnings in the plan preview. Review the resulting job logs after
        completion.
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecuteError" tone="error">
        {{ archiveExecuteError }}
      </UiInlineFeedback>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <UiPanel variant="soft" class="space-y-3 p-4">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Plan summary</p>
          <div class="grid gap-3 sm:grid-cols-2">
            <UiPanel variant="raise" class="min-w-0 space-y-1 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers queued</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">
                {{ planSummaryValue(queuedContainerCleanupCount, archivePlan.containers.length) }}
              </p>
            </UiPanel>
            <UiPanel variant="raise" class="min-w-0 space-y-1 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.hostnames.length }}</p>
            </UiPanel>
            <UiPanel variant="raise" class="min-w-0 space-y-1 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress queued</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">
                {{ planSummaryValue(queuedIngressCleanupCount, archivePlan.ingressRules.length) }}
              </p>
            </UiPanel>
            <UiPanel variant="raise" class="min-w-0 space-y-1 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS queued</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">
                {{ planSummaryValue(queuedDnsCleanupCount, archivePlan.dnsRecords.length) }}
              </p>
            </UiPanel>
            <UiPanel variant="raise" class="min-w-0 space-y-1 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service exposures</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.serviceExposures.length }}</p>
            </UiPanel>
          </div>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS targets</p>
            <p class="text-xs text-[color:var(--muted)]">{{ queuedDnsTargetsSummary() }}</p>
          </div>
          <UiState v-if="archivePlan.dnsRecords.length === 0">No DNS records matched.</UiState>
          <ul
            v-else
            class="grid gap-3 text-xs text-[color:var(--muted)] [grid-template-columns:repeat(auto-fit,minmax(240px,1fr))]"
          >
            <UiPanel
              v-for="record in archivePlan.dnsRecords"
              :key="`${record.zoneId}-${record.id}-${record.name}`"
              as="li"
              variant="raise"
              class="min-w-0 space-y-1 p-3"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="font-mono text-[color:var(--text)] break-all">{{ record.name }}</span>
                <UiBadge :tone="dnsRecordTone(record.deleteEligible)">
                  {{ describeDnsRecordAction(record.deleteEligible) }}
                </UiBadge>
              </div>
              <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                {{ record.type }} → {{ record.content }}
              </p>
              <p v-if="record.skipReason" class="text-[11px] text-[color:var(--muted)]">
                {{ record.skipReason }}
              </p>
            </UiPanel>
          </ul>
        </UiPanel>
      </div>

      <div class="grid gap-4 xl:grid-cols-2">
        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Container cleanup scope</p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ planSummaryValue(queuedContainerCleanupCount, archivePlan.containers.length) }} queued
            </p>
          </div>
          <UiState v-if="archivePlan.containers.length === 0">No managed project containers matched.</UiState>
          <ul v-else class="grid gap-3 text-xs text-[color:var(--muted)]">
            <UiPanel
              v-for="container in archivePlan.containers"
              :key="container.id"
              as="li"
              variant="raise"
              class="min-w-0 space-y-2 p-3"
            >
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-mono text-[color:var(--text)] break-all">{{ container.name }}</p>
                  <p class="text-[11px] text-[color:var(--muted-2)]">{{ container.service }} · {{ container.status }}</p>
                </div>
                <UiBadge :tone="archiveOptions.removeContainers ? 'warn' : 'neutral'">
                  {{ archiveOptions.removeContainers ? 'remove' : 'keep' }}
                </UiBadge>
              </div>
              <p class="break-all text-[11px] text-[color:var(--muted)]">{{ container.image }}</p>
            </UiPanel>
          </ul>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress cleanup scope</p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ planSummaryValue(queuedIngressCleanupCount, archivePlan.ingressRules.length) }} queued
            </p>
          </div>
          <UiState v-if="archivePlan.ingressRules.length === 0">No managed ingress rules matched.</UiState>
          <ul v-else class="grid gap-3 text-xs text-[color:var(--muted)]">
            <UiPanel
              v-for="rule in archivePlan.ingressRules"
              :key="`${rule.source}-${rule.hostname}-${rule.service}`"
              as="li"
              variant="raise"
              class="min-w-0 space-y-2 p-3"
            >
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-mono text-[color:var(--text)] break-all">{{ rule.hostname }}</p>
                  <p class="text-[11px] text-[color:var(--muted-2)]">{{ rule.service }}</p>
                </div>
                <div class="flex flex-wrap gap-2">
                  <UiBadge tone="neutral">{{ rule.source }}</UiBadge>
                  <UiBadge :tone="archiveOptions.removeIngress ? 'warn' : 'neutral'">
                    {{ archiveOptions.removeIngress ? 'remove' : 'keep' }}
                  </UiBadge>
                </div>
              </div>
            </UiPanel>
          </ul>
        </UiPanel>
      </div>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service exposure cleanup</p>
          <p class="text-xs text-[color:var(--muted)]">Deterministic ownership for `forward_local` and `quick_service`</p>
        </div>
        <UiState v-if="archivePlan.serviceExposures.length === 0">
          No managed `forward_local` or `quick_service` artifacts were resolved for this project.
        </UiState>
        <ul
          v-else
          class="grid gap-3 text-xs text-[color:var(--muted)] [grid-template-columns:repeat(auto-fit,minmax(280px,1fr))]"
        >
          <UiPanel
            v-for="exposure in archivePlan.serviceExposures"
            :key="`${exposure.type}-${exposure.jobId}-${exposure.hostname || exposure.container || exposure.resolution}`"
            as="li"
            variant="raise"
            class="min-w-0 space-y-2 p-3"
          >
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-mono text-[color:var(--text)]">{{ exposure.type }}</p>
                <p class="text-[11px] text-[color:var(--muted-2)]">Job #{{ exposure.jobId }} · {{ exposure.resolution }}</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <UiBadge
                  v-for="badge in plannedExposureCleanupBadges(exposure)"
                  :key="badge.key"
                  :tone="badge.tone"
                >
                  {{ badge.label }}
                </UiBadge>
              </div>
            </div>
            <p class="break-all">
              <span class="text-[color:var(--muted-2)]">Hostname:</span>
              <span class="ml-1 font-mono text-[color:var(--text)]">
                {{ exposureHasHostname(exposure) ? exposure.hostname : 'not managed for cleanup' }}
              </span>
            </p>
            <p class="break-all">
              <span class="text-[color:var(--muted-2)]">Container:</span>
              <span class="ml-1 font-mono text-[color:var(--text)]">
                {{ exposureHasContainer(exposure) ? exposure.container : 'not tracked' }}
              </span>
            </p>
            <p class="text-[11px] text-[color:var(--muted)]">
              {{ describeExposureCleanup(exposure) }}
            </p>
          </UiPanel>
        </ul>
      </UiPanel>
    </template>
  </UiPanel>

  <UiModal
    v-model="archiveReviewModalOpen"
    title="Confirm archive cleanup"
    description="Review the current preview scope and type the confirmation phrase to queue the existing async archive job."
    class="!w-[min(960px,96vw)]"
  >
    <div v-if="!archivePlan" class="space-y-4">
      <UiState tone="warn">
        Archive preview is missing. Refresh the preview from Project Detail before queueing cleanup.
      </UiState>
    </div>
    <div v-else class="space-y-4">
      <div class="grid gap-3 md:grid-cols-2">
        <UiPanel
          v-for="scope in archiveScopeRows"
          :key="`modal-${scope.key}`"
          variant="soft"
          class="space-y-2 p-3"
        >
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">{{ scope.label }}</p>
            <UiBadge :tone="archiveScopeTone(scope.enabled)">{{ scope.actionLabel }}</UiBadge>
          </div>
          <p class="text-lg font-semibold text-[color:var(--text)]">
            {{ planSummaryValue(scope.queuedCount, scope.totalCount) }}
          </p>
          <p class="text-xs text-[color:var(--muted)]">{{ scope.detail }}</p>
        </UiPanel>
      </div>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Destructive copy</p>
        <p class="text-sm text-[color:var(--muted)]">
          Queueing archive cleanup marks the project for archive and hands off destructive container, ingress, and DNS
          mutation work to the existing Jobs workflow. No cleanup runs synchronously from this panel.
        </p>
        <p v-if="archiveOptions.removeVolumes" class="text-sm text-[color:var(--danger)]">
          Volume deletion is enabled. Attached data for matched managed containers cannot be recovered after the job
          removes those volumes.
        </p>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Planner warnings</p>
          <UiBadge :tone="archivePlanWarnings.length > 0 ? 'warn' : 'ok'">
            {{ archivePlanWarnings.length }}
          </UiBadge>
        </div>
        <UiState v-if="archivePlanWarnings.length === 0" tone="ok">
          No warnings were reported for this archive preview.
        </UiState>
        <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
          <li
            v-for="(warning, index) in archivePlanWarnings"
            :key="`archive-warning-${index}`"
            class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2"
          >
            {{ warning }}
          </li>
        </ul>
      </UiPanel>

      <div class="rounded border border-[color:var(--danger)]/40 bg-[color:var(--surface-inset)]/60 px-4 py-3">
        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Confirmation phrase</p>
        <p class="mt-2 break-all font-mono text-lg font-semibold text-[color:var(--text)]">
          {{ archiveConfirmationPhrase }}
        </p>
      </div>

      <UiInput
        v-model="archiveConfirmInput"
        :disabled="archiveExecuting"
        autocomplete="off"
        spellcheck="false"
        placeholder="Type the confirmation phrase exactly"
      />

      <UiInlineFeedback v-if="archiveExecuteError" tone="error">
        {{ archiveExecuteError }}
      </UiInlineFeedback>
    </div>

    <template #footer>
      <div class="flex flex-wrap justify-end gap-3">
        <UiButton variant="ghost" size="sm" :disabled="archiveExecuting" @click="archiveReviewModalOpen = false">
          Cancel
        </UiButton>
        <UiButton variant="danger" size="sm" :disabled="!canSubmitArchive" @click="queueArchive">
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner v-if="archiveExecuting" />
            {{ archiveExecuting ? 'Queueing archive...' : 'Queue archive job' }}
          </span>
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
