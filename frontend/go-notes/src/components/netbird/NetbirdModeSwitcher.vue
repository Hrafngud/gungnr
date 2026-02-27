<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetbirdStore } from '@/stores/netbird'
import type { NetBirdMode, NetBirdServiceRebindingOperation } from '@/types/netbird'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'

type SelectOption = {
  value: string | number
  label: string
  disabled?: boolean
}

const modeOptions: SelectOption[] = [
  { value: 'legacy', label: 'Legacy' },
  { value: 'mode_a', label: 'Mode A' },
  { value: 'mode_b', label: 'Mode B' },
]

const modeDescriptions: Record<NetBirdMode, string> = {
  legacy: 'Existing listener behavior without NetBird policy isolation.',
  mode_a: 'Admins can reach only the panel listener over NetBird.',
  mode_b: 'Admins can reach panel plus per-project ingress over NetBird.',
}

const isNetBirdMode = (value: unknown): value is NetBirdMode =>
  value === 'legacy' || value === 'mode_a' || value === 'mode_b'

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()

const targetMode = ref<NetBirdMode>('legacy')
const targetModeTouched = ref(false)
const allowLocalhost = ref(false)
const confirmInput = ref('')
const modeInitialized = ref(false)
const terminalRefreshJobId = ref<number | null>(null)

const configPanelOpen = ref(false)
const configApiBaseUrl = ref('')
const configApiToken = ref('')
const configHostPeerId = ref('')
const configAdminPeerIdsInput = ref('')
const configSaveSuccess = ref<string | null>(null)

const isAdmin = computed(() => authStore.isAdmin)
const status = computed(() => netbirdStore.status.data)
const plan = computed(() => netbirdStore.modePlan.data)
const planLoading = computed(() => netbirdStore.modePlan.loading)
const planError = computed(() => netbirdStore.modePlan.error)
const applySubmitting = computed(() => netbirdStore.modeApply.submitting)
const applyError = computed(() => netbirdStore.modeApply.error)
const applyJob = computed(() => netbirdStore.modeApply.job)
const applyPolling = computed(() => netbirdStore.modeApplyPolling)
const applyPollingLifecycle = computed(() => applyPolling.value.lifecycle)
const applyPollingError = computed(() => applyPolling.value.error)
const applyPollingJob = computed(() => applyPolling.value.lastJob)
const applyPollingSummary = computed(() => applyPolling.value.summary)

const modeConfig = computed(() => netbirdStore.modeConfig.data)
const modeConfigLoading = computed(() => netbirdStore.modeConfig.loading)
const modeConfigError = computed(() => netbirdStore.modeConfig.error)
const modeConfigSaving = computed(() => netbirdStore.modeConfig.saving)
const modeConfigSaveError = computed(() => netbirdStore.modeConfig.saveError)

const applyPollingJobId = computed(() => applyPolling.value.jobId ?? applyJob.value?.id ?? null)
const applyPollingStatus = computed(
  () =>
    applyPollingJob.value?.status ??
    applyJob.value?.status ??
    (applyPollingLifecycle.value === 'running' ? 'pending' : ''),
)
const applyPollingStatusLabel = computed(() => jobStatusLabel(applyPollingStatus.value))
const applyPollingStatusTone = computed(() => jobStatusTone(applyPollingStatus.value))
const applyPollingWarnings = computed(() => applyPollingSummary.value?.warnings ?? [])
const applyPollingRebindingFailures = computed(
  () => applyPollingSummary.value?.rebindingExecution?.counts?.failed ?? 0,
)
const applyPollingRedeployFailures = computed(
  () => applyPollingSummary.value?.redeployExecution?.counts?.failed ?? 0,
)
const applyPollingFailureCount = computed(
  () => applyPollingRebindingFailures.value + applyPollingRedeployFailures.value,
)
const applyPollingHasWarnings = computed(
  () => applyPollingWarnings.value.length > 0 || applyPollingFailureCount.value > 0,
)
const applyPollingTerminalTone = computed<'ok' | 'warn' | 'error'>(() => {
  if (applyPollingStatus.value === 'failed') return 'error'
  return applyPollingHasWarnings.value ? 'warn' : 'ok'
})
const applyPollingRunningMessage = computed(() => {
  if (applyPollingStatus.value === 'running') {
    return 'Mode apply is running. NetBird reconcile and rebinding steps are in progress.'
  }
  if (applyPollingStatus.value === 'pending') {
    return 'Mode apply is queued. Polling will continue until the job reaches a terminal state.'
  }
  return 'Polling latest mode-apply job status...'
})
const applyPollingTerminalMessage = computed(() => {
  if (applyPollingStatus.value === 'failed') {
    return applyPollingJob.value?.error?.trim() || 'Mode apply failed. Open the job log for details.'
  }
  if (applyPollingHasWarnings.value) {
    return 'Mode apply completed with warnings. Review warning details below.'
  }
  return 'Mode apply completed successfully.'
})

const parsedConfigAdminPeerIds = computed(() =>
  configAdminPeerIdsInput.value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0),
)

const requiresPeerInputs = computed(() => targetMode.value !== 'legacy')
const confirmationPhrase = computed(() => `apply ${targetMode.value}`)
const confirmationReady = computed(() => confirmInput.value.trim() === confirmationPhrase.value)
const modeConfigHasToken = computed(() => Boolean(modeConfig.value?.apiTokenSet))
const modeConfigHasHostPeer = computed(() => (modeConfig.value?.hostPeerId || '').trim().length > 0)
const modeConfigHasAdminPeers = computed(() => (modeConfig.value?.adminPeerIds?.length ?? 0) > 0)
const planMatchesSelection = computed(() => {
  if (!plan.value) return false
  return (
    plan.value.targetMode === targetMode.value &&
    plan.value.allowLocalhost === allowLocalhost.value
  )
})

const canRunPlan = computed(() =>
  isAdmin.value && !planLoading.value && !applySubmitting.value,
)

const canApply = computed(() =>
  isAdmin.value &&
  !applySubmitting.value &&
  planMatchesSelection.value &&
  modeConfigHasToken.value &&
  (!requiresPeerInputs.value || (modeConfigHasHostPeer.value && modeConfigHasAdminPeers.value)) &&
  confirmationReady.value,
)

const modeLabel = (mode: NetBirdMode) => {
  if (mode === 'mode_a') return 'Mode A'
  if (mode === 'mode_b') return 'Mode B'
  return 'Legacy'
}

const listenerLabel = (listeners: string[]) =>
  listeners.length > 0 ? listeners.join(', ') : 'none'

const rebindingTitle = (operation: NetBirdServiceRebindingOperation) => {
  if (operation.projectName) {
    return `${operation.projectName} (${operation.service})`
  }
  return operation.service
}

const hydrateConfigForm = () => {
  configApiBaseUrl.value = modeConfig.value?.apiBaseUrl || ''
  configHostPeerId.value = modeConfig.value?.hostPeerId || status.value?.peerId || ''
  configAdminPeerIdsInput.value = (modeConfig.value?.adminPeerIds || []).join(',')
  configApiToken.value = ''
}

const openConfigPanel = () => {
  configSaveSuccess.value = null
  hydrateConfigForm()
  configPanelOpen.value = true
}

const triggerPlan = async () => {
  if (!isAdmin.value) return
  await netbirdStore.planModeSwitch({
    targetMode: targetMode.value,
    allowLocalhost: allowLocalhost.value,
  })
}

const triggerApply = async () => {
  if (!canApply.value) return
  await netbirdStore.applyModeSwitch({
    targetMode: targetMode.value,
    allowLocalhost: allowLocalhost.value,
  })
}

const saveModeConfig = async () => {
  if (!isAdmin.value || modeConfigSaving.value) return
  configSaveSuccess.value = null

  const payload: {
    apiBaseUrl?: string
    apiToken?: string
    hostPeerId?: string
    adminPeerIds: string[]
  } = {
    apiBaseUrl: configApiBaseUrl.value.trim() || undefined,
    hostPeerId: configHostPeerId.value.trim() || undefined,
    adminPeerIds: parsedConfigAdminPeerIds.value,
  }
  const token = configApiToken.value.trim()
  if (token !== '') {
    payload.apiToken = token
  }

  await netbirdStore.saveModeConfig(payload)
  if (!modeConfigSaveError.value) {
    configApiToken.value = ''
    configSaveSuccess.value = 'NetBird mode config saved.'
    await netbirdStore.loadModeConfig()
  }
}

const reloadModeConfig = async () => {
  await netbirdStore.loadModeConfig()
  hydrateConfigForm()
}

const retryApplyPolling = () => {
  if (!applyPollingJobId.value) return
  void netbirdStore.startModeApplyJobPolling(applyPollingJobId.value)
}

const isTerminalJobStatus = (value?: string) => {
  const normalized = (value || '').trim().toLowerCase()
  return normalized === 'completed' || normalized === 'failed'
}

const syncStatusToJobStatus = (value?: string): string => {
  const normalized = (value || '').trim().toLowerCase()
  if (normalized === 'pending') return 'pending'
  if (normalized === 'succeeded') return 'completed'
  if (normalized === 'failed') return 'failed'
  return ''
}

const onTargetModeUpdate = (value: string | number) => {
  if (!isNetBirdMode(value)) return
  targetModeTouched.value = true
  targetMode.value = value
}

watch(
  () => status.value?.currentMode,
  (mode) => {
    if (mode && !modeInitialized.value && !targetModeTouched.value) {
      targetMode.value = mode
      modeInitialized.value = true
    }
  },
  { immediate: true },
)

watch([targetMode, allowLocalhost], () => {
  confirmInput.value = ''
})

watch(
  () => status.value?.peerId,
  (peerId) => {
    if (!peerId) return
    if (configHostPeerId.value.trim().length > 0) return
    configHostPeerId.value = peerId
  },
)

watch(
  () => [applyPollingLifecycle.value, applyPollingJobId.value] as const,
  ([lifecycle, jobId]) => {
    if (!jobId || lifecycle !== 'terminal') return
    if (terminalRefreshJobId.value === jobId) return
    terminalRefreshJobId.value = jobId
    void Promise.all([netbirdStore.loadStatus(), netbirdStore.loadAclGraph()])
  },
)

watch(
  () => [status.value?.lastPolicySyncJobId ?? null, status.value?.lastPolicySyncStatus ?? ''] as const,
  ([jobId, syncStatus]) => {
    if (!jobId) return

    const localSnapshot = applyPolling.value.lastJob
    const hasSameSnapshot = localSnapshot?.id === jobId
    const localSnapshotStatus = (localSnapshot?.status || '').trim().toLowerCase()
    const expectedStatus = syncStatusToJobStatus(syncStatus)
    const snapshotMatchesExpected =
      hasSameSnapshot && expectedStatus !== '' && localSnapshotStatus === expectedStatus

    if (applyPolling.value.jobId === jobId && applyPolling.value.lifecycle === 'running') return
    if (snapshotMatchesExpected && isTerminalJobStatus(localSnapshotStatus)) return

    void netbirdStore.startModeApplyJobPolling(jobId)
  },
  { immediate: true },
)

onMounted(() => {
  const loaders: Promise<unknown>[] = []
  if (!status.value) {
    loaders.push(netbirdStore.loadStatus())
  }
  if (isAdmin.value && !modeConfig.value && !modeConfigLoading.value) {
    loaders.push(netbirdStore.loadModeConfig())
  }
  if (loaders.length > 0) {
    void Promise.all(loaders).then(() => {
      if (isAdmin.value) {
        hydrateConfigForm()
      }
    })
  }
})

onBeforeUnmount(() => {
  netbirdStore.stopModeApplyJobPolling()
})
</script>

<template>
  <UiPanel as="article" class="space-y-5 p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          NetBird
        </p>
        <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          Mode switch
        </h2>
        <p class="mt-1 text-xs text-[color:var(--muted)]">
          Plan first, then apply mode changes through an explicit confirmation gate.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <UiBadge tone="neutral">
          Current: {{ status ? modeLabel(status.currentMode) : 'unknown' }}
        </UiBadge>
        <UiBadge :tone="modeConfigHasToken ? 'ok' : 'warn'">
          Config token: {{ modeConfigHasToken ? 'set' : 'missing' }}
        </UiBadge>
      </div>
    </div>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required for mode planning and apply actions.
    </UiState>

    <div class="flex flex-wrap items-center gap-3">
      <UiButton
        variant="ghost"
        size="sm"
        :disabled="!isAdmin || modeConfigLoading"
        @click="openConfigPanel"
      >
        <span class="inline-flex items-center gap-2">
          <UiInlineSpinner v-if="modeConfigLoading" />
          {{ modeConfigLoading ? 'Loading config...' : 'Edit mode config' }}
        </span>
      </UiButton>
      <p class="text-xs text-[color:var(--muted)]">
        Credentials are saved once and reused across mode switches.
      </p>
    </div>

    <UiState v-if="modeConfigError" tone="error">
      {{ modeConfigError }}
    </UiState>

    <UiPanel variant="soft" class="space-y-4 p-4">
      <div class="grid gap-2 text-sm">
        <label class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Target mode
        </label>
        <UiSelect
          :model-value="targetMode"
          :options="modeOptions"
          :disabled="!isAdmin || planLoading || applySubmitting"
          placeholder="Select target mode"
          @update:model-value="onTargetModeUpdate"
        />
        <p class="text-xs text-[color:var(--muted)]">
          {{ modeDescriptions[targetMode] }}
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UiToggle
          v-model="allowLocalhost"
          :disabled="!isAdmin || planLoading || applySubmitting"
        >
          Allow localhost listeners
        </UiToggle>
        <UiBadge :tone="allowLocalhost ? 'ok' : 'neutral'">
          {{ allowLocalhost ? 'Enabled' : 'Disabled' }}
        </UiBadge>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UiButton
          variant="primary"
          size="sm"
          :disabled="!canRunPlan"
          @click="triggerPlan"
        >
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner v-if="planLoading" />
            {{ planLoading ? 'Planning...' : 'Run dry-run plan' }}
          </span>
        </UiButton>
        <p class="text-xs text-[color:var(--muted)]">
          Dry-run uses <span class="font-mono">targetMode</span> and
          <span class="font-mono">allowLocalhost</span> only.
        </p>
      </div>
    </UiPanel>

    <UiState v-if="planError" tone="error">
      {{ planError }}
    </UiState>

    <UiState v-if="plan && !planMatchesSelection" tone="warn">
      Planned values no longer match current controls. Run dry-run again before apply.
    </UiState>

    <UiState v-if="planLoading && !plan" loading>
      Building NetBird mode plan...
    </UiState>

    <div v-else-if="plan" class="space-y-4">
      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Service rebinding operations
          </p>
          <UiBadge tone="neutral">{{ plan.serviceRebindingOperations.length }}</UiBadge>
        </div>
        <UiState v-if="plan.serviceRebindingOperations.length === 0">
          No listener rebinding changes are required.
        </UiState>
        <ul v-else class="space-y-2">
          <UiListRow
            v-for="operation in plan.serviceRebindingOperations"
            :key="`${operation.service}-${operation.projectId ?? 0}-${operation.port}`"
            as="li"
            class="space-y-2"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs font-semibold text-[color:var(--text)]">
                {{ rebindingTitle(operation) }}
              </p>
              <UiBadge tone="neutral">Port {{ operation.port }}</UiBadge>
            </div>
            <div class="grid gap-1 text-xs text-[color:var(--muted)]">
              <p>From: <span class="font-mono text-[color:var(--text)]">{{ listenerLabel(operation.fromListeners) }}</span></p>
              <p>To: <span class="font-mono text-[color:var(--text)]">{{ listenerLabel(operation.toListeners) }}</span></p>
              <p>{{ operation.reason }}</p>
            </div>
          </UiListRow>
        </ul>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Redeploy targets
          </p>
          <UiBadge tone="neutral">
            {{ (plan.redeployTargets.panel ? 1 : 0) + plan.redeployTargets.projects.length }}
          </UiBadge>
        </div>
        <div class="grid gap-1 text-xs text-[color:var(--muted)]">
          <p>
            Panel:
            <span class="text-[color:var(--text)]">
              {{ plan.redeployTargets.panel ? 'required' : 'not required' }}
            </span>
          </p>
        </div>
        <UiState v-if="plan.redeployTargets.projects.length === 0">
          No project stack redeploys are required.
        </UiState>
        <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
          <UiListRow
            v-for="project in plan.redeployTargets.projects"
            :key="`${project.projectId}-${project.port}`"
            as="li"
            class="space-y-1"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="font-semibold text-[color:var(--text)]">{{ project.projectName }}</p>
              <UiBadge tone="neutral">Port {{ project.port }}</UiBadge>
            </div>
            <p>{{ project.reason }}</p>
          </UiListRow>
        </ul>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Planner warnings
          </p>
          <UiBadge :tone="plan.warnings.length > 0 ? 'warn' : 'ok'">
            {{ plan.warnings.length }}
          </UiBadge>
        </div>
        <UiState v-if="plan.warnings.length === 0" tone="ok">
          No warnings reported for this dry-run.
        </UiState>
        <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
          <li
            v-for="(warning, index) in plan.warnings"
            :key="`warning-${index}`"
            class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2"
          >
            {{ warning }}
          </li>
        </ul>
      </UiPanel>
    </div>

    <UiState v-else>
      Run a dry-run plan to preview rebinding and redeploy impact before apply.
    </UiState>

    <UiPanel variant="raise" class="space-y-4 p-5">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Apply mode switch
        </p>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Confirmation phrase:
          <span class="font-mono text-[color:var(--text)]">{{ confirmationPhrase }}</span>
        </p>
      </div>

      <UiPanel variant="soft" class="space-y-2 p-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Saved mode config
          </p>
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="!isAdmin"
            @click="openConfigPanel"
          >
            Edit config
          </UiButton>
        </div>
        <div class="grid gap-1 text-xs text-[color:var(--muted)]">
          <p>
            API token:
            <span class="text-[color:var(--text)]">{{ modeConfigHasToken ? 'configured' : 'missing' }}</span>
          </p>
          <p>
            API base URL:
            <span class="text-[color:var(--text)]">{{ modeConfig?.apiBaseUrl || 'default (api.netbird.io)' }}</span>
          </p>
          <p>
            Host peer ID:
            <span class="font-mono text-[color:var(--text)]">{{ modeConfig?.hostPeerId || 'n/a' }}</span>
          </p>
          <p>
            Admin peer IDs:
            <span class="text-[color:var(--text)]">{{ modeConfig?.adminPeerIds?.length ?? 0 }}</span>
          </p>
        </div>
      </UiPanel>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Confirmation phrase
        </span>
        <UiInput
          v-model="confirmInput"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || applySubmitting"
          placeholder="Type the phrase exactly"
        />
      </label>

      <UiState v-if="isAdmin && !modeConfigHasToken" tone="warn">
        Save NetBird API credentials before queueing apply.
      </UiState>
      <UiState v-if="isAdmin && requiresPeerInputs && !modeConfigHasHostPeer" tone="warn">
        Saved host peer ID is required for Mode A and Mode B.
      </UiState>
      <UiState v-if="isAdmin && requiresPeerInputs && !modeConfigHasAdminPeers" tone="warn">
        Save at least one admin peer ID for Mode A and Mode B.
      </UiState>
      <UiState v-if="isAdmin && !planMatchesSelection" tone="warn">
        Apply requires a dry-run plan for the current target mode and localhost toggle.
      </UiState>
      <UiState v-if="isAdmin && !confirmationReady" tone="warn">
        Type the exact confirmation phrase to enable apply.
      </UiState>

      <div class="flex flex-wrap items-center gap-3">
        <UiButton
          variant="danger"
          size="sm"
          :disabled="!canApply"
          @click="triggerApply"
        >
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner v-if="applySubmitting" />
            {{ applySubmitting ? 'Queueing mode apply...' : `Queue apply (${modeLabel(targetMode)})` }}
          </span>
        </UiButton>
        <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
          Read-only access: apply actions are admin-only.
        </p>
      </div>

      <UiState v-if="applyError" tone="error">
        {{ applyError }}
      </UiState>
      <UiState v-if="applyPollingLifecycle === 'error'" tone="error">
        {{ applyPollingError || 'Mode apply polling failed.' }}
      </UiState>

      <UiPanel v-if="applyPollingJobId" variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Mode apply job
          </p>
          <UiBadge :tone="applyPollingStatusTone">
            {{ applyPollingStatusLabel }}
          </UiBadge>
        </div>
        <p class="text-xs text-[color:var(--muted)]">
          Job #{{ applyPollingJobId }}
        </p>

        <UiState v-if="applyPollingLifecycle === 'running'" loading>
          {{ applyPollingRunningMessage }}
        </UiState>
        <UiState
          v-else-if="applyPollingLifecycle === 'terminal'"
          :tone="applyPollingTerminalTone"
        >
          {{ applyPollingTerminalMessage }}
        </UiState>

        <UiState
          v-if="applyPollingLifecycle === 'terminal' && applyPollingHasWarnings"
          tone="warn"
        >
          Warnings: {{ applyPollingWarnings.length }} |
          Rebinding failures: {{ applyPollingRebindingFailures }} |
          Redeploy failures: {{ applyPollingRedeployFailures }}
        </UiState>

        <ul
          v-if="applyPollingLifecycle === 'terminal' && applyPollingWarnings.length > 0"
          class="space-y-2 text-xs text-[color:var(--muted)]"
        >
          <li
            v-for="(warning, index) in applyPollingWarnings.slice(0, 5)"
            :key="`mode-apply-warning-${index}`"
            class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2"
          >
            {{ warning }}
          </li>
        </ul>

        <div class="flex flex-wrap items-center gap-3">
          <UiButton
            v-if="applyPollingLifecycle === 'error'"
            variant="ghost"
            size="sm"
            @click="retryApplyPolling"
          >
            Retry polling
          </UiButton>
          <UiButton
            :as="RouterLink"
            :to="`/jobs/${applyPollingJobId}`"
            variant="ghost"
            size="sm"
          >
            Open job log
          </UiButton>
        </div>
      </UiPanel>
    </UiPanel>
  </UiPanel>

  <UiFormSidePanel
    v-model="configPanelOpen"
    title="NetBird mode config"
    eyebrow="NetBird"
  >
    <form class="space-y-5" @submit.prevent="saveModeConfig">
      <div>
        <p class="text-xs text-[color:var(--muted)]">
          Saved here once and reused for future mode switches.
        </p>
      </div>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          NetBird API token
        </span>
        <UiInput
          v-model="configApiToken"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || modeConfigSaving"
          placeholder="Leave empty to keep current token"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          NetBird API base URL (optional)
        </span>
        <UiInput
          v-model="configApiBaseUrl"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || modeConfigSaving"
          placeholder="https://api.netbird.io"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Host peer ID
        </span>
        <UiInput
          v-model="configHostPeerId"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || modeConfigSaving"
          placeholder="Host peer ID used for panel/project groups"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Admin peer IDs (comma separated)
        </span>
        <UiInput
          v-model="configAdminPeerIdsInput"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || modeConfigSaving"
          placeholder="peer-id-1,peer-id-2"
        />
      </label>

      <UiState v-if="modeConfigSaveError" tone="error">
        {{ modeConfigSaveError }}
      </UiState>
      <UiState v-if="configSaveSuccess" tone="ok">
        {{ configSaveSuccess }}
      </UiState>

      <div class="flex flex-wrap items-center gap-3">
        <UiButton
          type="submit"
          variant="primary"
          size="sm"
          :disabled="!isAdmin || modeConfigSaving"
        >
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner v-if="modeConfigSaving" />
            {{ modeConfigSaving ? 'Saving...' : 'Save mode config' }}
          </span>
        </UiButton>
        <UiButton
          type="button"
          variant="ghost"
          size="sm"
          :disabled="modeConfigLoading || modeConfigSaving"
          @click="reloadModeConfig"
        >
          Reload
        </UiButton>
      </div>
    </form>
  </UiFormSidePanel>
</template>
