<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { apiErrorMessage } from '@/services/api'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import type {
  ProjectArchiveOptions,
  ProjectArchivePlan,
  ProjectArchivePlanServiceExposure,
} from '@/types/projects'

const props = defineProps<{
  projectName: string
  projectDisplayName: string
  isAdmin: boolean
}>()

const emit = defineEmits<{
  queued: []
}>()

const toastStore = useToastStore()

const archivePlan = ref<ProjectArchivePlan | null>(null)
const archivePlanLoading = ref(false)
const archivePlanError = ref<string | null>(null)
const archiveExecuting = ref(false)
const archiveExecuteError = ref<string | null>(null)
const archiveExecutedWithWarnings = ref(false)
const archiveConfirmInput = ref('')
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

const canSubmitArchive = computed(() => {
  if (!props.isAdmin || archiveExecuting.value) return false
  if (archiveOptions.value.removeVolumes && !archiveOptions.value.removeContainers) return false
  return archiveConfirmInput.value.trim() === archiveConfirmationPhrase.value
})

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

    emit('queued')
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
    archiveExecutedWithWarnings.value = false
    archiveConfirmInput.value = ''
    void loadArchivePlan()
  },
  { immediate: true },
)

</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project lifecycle</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Archive</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Review the dry-run preview first, then queue the archive execution as an asynchronous project job.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" :disabled="archivePlanLoading" @click="loadArchivePlan">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="archivePlanLoading" />
          Refresh preview
        </span>
      </UiButton>
    </div>

    <UiState v-if="!isAdmin">
      Read-only access: admin permissions are required to preview and execute archive cleanup.
    </UiState>
    <UiState v-else-if="archivePlanLoading" loading>Building archive cleanup plan...</UiState>
    <UiState v-else-if="archivePlanError" tone="error">{{ archivePlanError }}</UiState>

    <template v-else-if="archivePlan">
      <UiPanel variant="raise" class="space-y-4 p-5">
        <div>
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Preview Then Execute</p>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            This preview is read-only. The queued archive job reuses the toggles below and applies cleanup through the
            existing bridge-backed project workflow.
          </p>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Confirmation phrase: <span class="font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
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

        <div class="flex flex-wrap items-end gap-3">
          <div class="min-w-[280px] flex-1">
            <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Confirmation phrase
            </label>
            <UiInput
              v-model="archiveConfirmInput"
              :disabled="!isAdmin || archiveExecuting"
              autocomplete="off"
              spellcheck="false"
              placeholder="Type the phrase exactly"
              class="mt-2"
            />
          </div>
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

      <UiInlineFeedback v-if="archivePlan.warnings.length > 0" tone="warn">
        {{ archivePlan.warnings.length }} warning(s): {{ archivePlan.warnings.join(' | ') }}
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecutedWithWarnings" tone="warn">
        Last archive request was queued with warnings in the plan preview. Review job logs after completion.
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
              as="li"
              variant="raise"
              :key="`${record.zoneId}-${record.id}-${record.name}`"
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
</template>
