<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiState from '@/components/ui/UiState.vue'
import UiStatusToggleButton from '@/components/ui/UiStatusToggleButton.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetbirdStore } from '@/stores/netbird'
import { useProjectsStore } from '@/stores/projects'
import type { NetBirdMode, NetBirdServiceRebindingOperation } from '@/types/netbird'
import type { Project } from '@/types/projects'

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
  legacy: 'Docker default network isolation / expose',
  mode_a: 'Gungnr is isolated in a dedicated Netbird interface.',
  mode_b: 'Mode A + per-project isolation networks over NetBird.',
}

const isNetBirdMode = (value: unknown): value is NetBirdMode =>
  value === 'legacy' || value === 'mode_a' || value === 'mode_b'

function normalizeProjectIDs(values: number[]) {
  return Array.from(new Set((values || []).filter((value) => Number.isFinite(value) && value > 0)))
    .map((value) => Math.trunc(value))
    .sort((left, right) => left - right)
}

function arraysEqual(left: number[], right: number[]) {
  if (left.length !== right.length) return false
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) return false
  }
  return true
}

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()
const projectsStore = useProjectsStore()

const targetMode = ref<NetBirdMode>('legacy')
const targetModeTouched = ref(false)
const modeBProjectsTouched = ref(false)
const allowLocalhost = ref(false)
const modeBProjectIds = ref<number[]>([])
const confirmInput = ref('')
const applyFlowModalOpen = ref(false)
const applyFlowStep = ref<'review' | 'confirm'>('review')
const showConfigRequirementHint = ref(false)
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
const projects = computed(() => projectsStore.projects)
const projectsLoading = computed(() => projectsStore.loading)
const projectsError = computed(() => projectsStore.error)
const plan = computed(() => netbirdStore.modePlan.data)
const planLoading = computed(() => netbirdStore.modePlan.loading)
const planError = computed(() => netbirdStore.modePlan.error)
const applySubmitting = computed(() => netbirdStore.modeApply.submitting)
const applyError = computed(() => netbirdStore.modeApply.error)
const applyJob = computed(() => netbirdStore.modeApply.job)
const applyPolling = computed(() => netbirdStore.modeApplyPolling)
const applyPollingLifecycle = computed(() => applyPolling.value.lifecycle)
const applyPollingError = computed(() => applyPolling.value.error)

const modeConfig = computed(() => netbirdStore.modeConfig.data)
const modeConfigLoading = computed(() => netbirdStore.modeConfig.loading)
const modeConfigError = computed(() => netbirdStore.modeConfig.error)
const modeConfigSaving = computed(() => netbirdStore.modeConfig.saving)
const modeConfigSaveError = computed(() => netbirdStore.modeConfig.saveError)

const applyPollingJobId = computed(() => applyPolling.value.jobId ?? applyJob.value?.id ?? null)

const parsedConfigAdminPeerIds = computed(() =>
  configAdminPeerIdsInput.value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0),
)

const requiresPeerInputs = computed(() => targetMode.value !== 'legacy')
const requiresModeBSelection = computed(() => targetMode.value === 'mode_b')
const confirmationPhrase = computed(() => `apply ${targetMode.value}`)
const confirmationReady = computed(() => confirmInput.value.trim() === confirmationPhrase.value)
const modeConfigHasToken = computed(() => Boolean(modeConfig.value?.apiTokenSet))
const modeConfigHasHostPeer = computed(() => (modeConfig.value?.hostPeerId || '').trim().length > 0)
const modeConfigHasAdminPeers = computed(() => (modeConfig.value?.adminPeerIds?.length ?? 0) > 0)
const planMatchesSelection = computed(() => {
  if (!plan.value) return false
  const planModeBProjectIds = normalizeProjectIDs(plan.value.targetModeBProjectIds || [])
  const selectedModeBProjectIds = normalizeProjectIDs(modeBProjectIds.value)
  return (
    plan.value.targetMode === targetMode.value &&
    plan.value.allowLocalhost === allowLocalhost.value &&
    (!requiresModeBSelection.value || arraysEqual(planModeBProjectIds, selectedModeBProjectIds))
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

const applyConfigMissingItems = computed(() => {
  const items: string[] = []
  if (!modeConfigHasToken.value) {
    items.push('API token')
  }
  if (requiresPeerInputs.value && !modeConfigHasHostPeer.value) {
    items.push('host peer ID')
  }
  if (requiresPeerInputs.value && !modeConfigHasAdminPeers.value) {
    items.push('admin peer IDs')
  }
  return items
})
const applyConfigRequirementsMet = computed(() => applyConfigMissingItems.value.length === 0)
const applyFlowTitle = computed(() =>
  applyFlowStep.value === 'review' ? 'Review dry-run plan' : 'Confirm mode switch',
)
const applyFlowDescription = computed(() =>
  applyFlowStep.value === 'review'
    ? `Review plan feedback before queueing ${modeLabel(targetMode.value)}.`
    : `Type ${confirmationPhrase.value} to queue ${modeLabel(targetMode.value)}.`,
)

const modeLabel = (mode: NetBirdMode) => {
  if (mode === 'mode_a') return 'Mode A'
  if (mode === 'mode_b') return 'Mode B'
  return 'Legacy'
}

const projectStatusTone = (project: Project): 'ok' | 'warn' | 'neutral' => {
  const statusValue = (project.status || '').trim().toLowerCase()
  if (statusValue === 'running') return 'ok'
  if (statusValue === 'degraded') return 'warn'
  return 'neutral'
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
  if (!applyConfigRequirementsMet.value) {
    showConfigRequirementHint.value = true
    applyFlowModalOpen.value = false
    return
  }
  showConfigRequirementHint.value = false
  await netbirdStore.planModeSwitch({
    targetMode: targetMode.value,
    allowLocalhost: allowLocalhost.value,
    modeBProjectIds: requiresModeBSelection.value ? normalizeProjectIDs(modeBProjectIds.value) : [],
  })
  if (!planError.value && plan.value) {
    applyFlowStep.value = 'review'
    confirmInput.value = ''
    applyFlowModalOpen.value = true
  }
}

const goToConfirmStep = () => {
  if (!plan.value || !planMatchesSelection.value) return
  applyFlowStep.value = 'confirm'
}

const triggerApply = async () => {
  if (!canApply.value) return
  await netbirdStore.applyModeSwitch({
    targetMode: targetMode.value,
    allowLocalhost: allowLocalhost.value,
    modeBProjectIds: requiresModeBSelection.value ? normalizeProjectIDs(modeBProjectIds.value) : [],
  })
  if (!applyError.value) {
    applyFlowModalOpen.value = false
    applyFlowStep.value = 'review'
    confirmInput.value = ''
  }
}

const toggleModeBProject = (projectId: number) => {
  modeBProjectsTouched.value = true
  const next = new Set(modeBProjectIds.value)
  if (next.has(projectId)) {
    next.delete(projectId)
  } else {
    next.add(projectId)
  }
  modeBProjectIds.value = normalizeProjectIDs(Array.from(next))
}

const clearModeBProjects = () => {
  modeBProjectsTouched.value = true
  modeBProjectIds.value = []
}

const selectAllModeBProjects = () => {
  modeBProjectsTouched.value = true
  modeBProjectIds.value = normalizeProjectIDs(projects.value.map((project) => project.id))
}

const selectRunningModeBProjects = () => {
  modeBProjectsTouched.value = true
  modeBProjectIds.value = normalizeProjectIDs(
    projects.value
      .filter((project) => (project.status || '').trim().toLowerCase() === 'running')
      .map((project) => project.id),
  )
}

const resetModeBProjectsToEffective = () => {
  modeBProjectsTouched.value = false
  modeBProjectIds.value = normalizeProjectIDs(status.value?.effectiveModeBProjectIds || [])
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

watch(
  () => status.value?.effectiveModeBProjectIds,
  (ids) => {
    if (modeBProjectsTouched.value) return
    modeBProjectIds.value = normalizeProjectIDs(ids || [])
  },
  { immediate: true },
)

watch([targetMode, allowLocalhost, modeBProjectIds], () => {
  confirmInput.value = ''
  applyFlowModalOpen.value = false
  applyFlowStep.value = 'review'
})

watch(
  () => applyConfigRequirementsMet.value,
  (ready) => {
    if (ready) {
      showConfigRequirementHint.value = false
    }
  },
)

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
  if (!projectsStore.initialized && !projectsLoading.value) {
    loaders.push(projectsStore.fetchProjects())
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
        <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          Mode switch
        </h2>
      </div>
      <div class="flex flex-wrap items-center gap-2">
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
        :class="showConfigRequirementHint && !applyConfigRequirementsMet ? 'ring-2 ring-[color:var(--accent)] ring-offset-2 ring-offset-[color:var(--surface)]' : ''"
        :disabled="!isAdmin || modeConfigLoading"
        @click="openConfigPanel"
      >
        <span class="inline-flex items-center gap-2">
          <UiInlineSpinner v-if="modeConfigLoading" />
          {{ modeConfigLoading ? 'Loading config...' : 'Edit mode config' }}
        </span>
      </UiButton>
    </div>

    <UiState v-if="showConfigRequirementHint && !applyConfigRequirementsMet" tone="warn">
      Complete NetBird mode config first, then run dry-run plan again. Missing:
      <span class="font-semibold">{{ applyConfigMissingItems.join(', ') }}</span>.
    </UiState>

    <UiState v-if="modeConfigError" tone="error">
      {{ modeConfigError }}
    </UiState>

    <UiPanel variant="soft" class="space-y-4 p-4">
      <div class="grid grid-cols-2">
        <div class="flex flex-col self-start gap-2 col-span-1">
        <label class="text-xs h-10 uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Target mode
        </label>
        <div class="flex flex-wrap items-center gap-3">
        <UiToggle
          v-model="allowLocalhost"
          :disabled="!isAdmin || planLoading || applySubmitting"
        >
          Allow localhost listeners
        </UiToggle>
      </div>
        </div>
        <div class="flex flex-col gap-2 col-span-1">
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
      </div>

      <UiPanel
        v-if="requiresModeBSelection"
        variant="soft"
        class="space-y-3 p-4"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Mode B project assignments
          </p>
          <UiBadge tone="neutral">{{ modeBProjectIds.length }}</UiBadge>
        </div>

        <div class="mode-b-action-buttons">
          <UiButton
            variant="ghost"
            size="sm"
            class="mode-b-action-button"
            :disabled="!isAdmin || projectsLoading"
            @click="selectRunningModeBProjects"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="activity" class="h-3.5 w-3.5" />
              Select running
            </span>
          </UiButton>
          <UiButton
            variant="ghost"
            size="sm"
            class="mode-b-action-button"
            :disabled="!isAdmin || projectsLoading"
            @click="selectAllModeBProjects"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="projects" class="h-3.5 w-3.5" />
              Select all
            </span>
          </UiButton>
          <UiButton
            variant="ghost"
            size="sm"
            class="mode-b-action-button"
            :disabled="!isAdmin"
            @click="clearModeBProjects"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="stop" class="h-3.5 w-3.5" />
              Clear
            </span>
          </UiButton>
          <UiButton
            variant="ghost"
            size="sm"
            class="mode-b-action-button"
            :disabled="!isAdmin"
            @click="resetModeBProjectsToEffective"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              Reset to active
            </span>
          </UiButton>
        </div>

        <UiState v-if="projectsError" tone="error">
          {{ projectsError }}
        </UiState>
        <UiState v-else-if="projectsLoading" loading>
          Loading projects...
        </UiState>
        <UiState v-else-if="projects.length === 0">
          No projects found.
        </UiState>
        <div
          v-else
          class="mode-b-project-grid-shell"
          role="group"
          aria-label="Mode B project assignments"
        >
          <div class="mode-b-project-grid">
            <UiStatusToggleButton
              v-for="project in projects"
              :key="`mode-b-project-${project.id}`"
              :model-value="modeBProjectIds.includes(project.id)"
              :label="project.name"
              :status-tone="projectStatusTone(project)"
              :disabled="!isAdmin"
              @update:model-value="toggleModeBProject(project.id)"
            />
          </div>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Selected projects will run ingress listeners under NetBird in Mode B. Unselected projects remain on legacy listeners.
        </p>
      </UiPanel>

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
          <span class="font-mono">allowLocalhost</span>
          <span v-if="requiresModeBSelection"> + <span class="font-mono">modeBProjectIds</span></span>.
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

    <UiState v-else-if="!plan">
      Run a dry-run plan to preview rebinding and redeploy impact before apply.
    </UiState>

    <UiState v-if="applyError" tone="error">
      {{ applyError }}
    </UiState>
    <UiState v-if="applyPollingLifecycle === 'error'" tone="error">
      {{ applyPollingError || 'Mode apply polling failed.' }}
    </UiState>

  </UiPanel>

  <UiModal
    v-model="applyFlowModalOpen"
    :title="applyFlowTitle"
    :description="applyFlowDescription"
    class="!w-[min(1100px,96vw)]"
  >
    <div v-if="applyFlowStep === 'review'" class="space-y-4">
      <UiState v-if="!plan || !planMatchesSelection" tone="warn">
        Plan snapshot is stale or missing. Run dry-run plan again from the mode switch section.
      </UiState>

      <div v-else class="space-y-4">
        <div class="grid gap-4 lg:grid-cols-2">
          <UiPanel variant="soft" class="max-h-[420px] space-y-3 overflow-y-auto p-4">
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

          <UiPanel variant="soft" class="max-h-[420px] space-y-3 overflow-y-auto p-4">
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
        </div>

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
    </div>

    <div v-else class="space-y-4">
      <div class="rounded border border-[color:var(--accent-soft)] bg-[color:var(--surface-2)] px-4 py-3">
        <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Confirmation phrase
        </p>
        <p class="mt-2 break-all font-mono text-xl font-semibold text-[color:var(--accent-strong)]">
          {{ confirmationPhrase }}
        </p>
      </div>

      <UiState v-if="applyError" tone="error">
        {{ applyError }}
      </UiState>

      <label class="grid gap-2 text-sm">
        <UiInput
          v-model="confirmInput"
          type="text"
          autocomplete="off"
          spellcheck="false"
          class="!border-[color:var(--border-soft)] !bg-[color:var(--surface-inset)]"
          :disabled="!isAdmin || applySubmitting"
          placeholder="Type confirmation phrase"
        />
      </label>
      <p class="text-xs text-[color:var(--muted)]">
        Queue apply activates once this input matches the phrase exactly.
      </p>
    </div>
    <template #footer>
      <div class="flex flex-wrap justify-end gap-3">
        <template v-if="applyFlowStep === 'review'">
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="applySubmitting"
            @click="applyFlowModalOpen = false"
          >
            Cancel
          </UiButton>
          <UiButton
            variant="primary"
            size="sm"
            :disabled="!plan || !planMatchesSelection || applySubmitting"
            @click="goToConfirmStep"
          >
            Proceed
          </UiButton>
        </template>
        <template v-else>
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="applySubmitting"
            @click="applyFlowStep = 'review'"
          >
            Back
          </UiButton>
          <UiButton
            variant="danger"
            size="sm"
            :disabled="!canApply"
            @click="triggerApply"
          >
            <span class="inline-flex items-center gap-2">
              <UiInlineSpinner v-if="applySubmitting" />
              {{ applySubmitting ? 'Queueing...' : `Queue apply (${modeLabel(targetMode)})` }}
            </span>
          </UiButton>
        </template>
      </div>
    </template>
  </UiModal>

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

<style scoped>
.mode-b-action-buttons {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.25rem, 1fr));
  gap: 0.5rem;
}

.mode-b-action-button {
  justify-content: center;
}

.mode-b-project-grid-shell {
  --mode-b-row-height: 2.75rem;
  --mode-b-gap: 0.5rem;
  width: 100%;
  max-height: calc((var(--mode-b-row-height) * 2.5) + (var(--mode-b-gap) * 2));
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 0.25rem;
  padding-bottom: 0.25rem;
}

.mode-b-project-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-auto-rows: var(--mode-b-row-height);
  gap: var(--mode-b-gap);
  width: 100%;
}
</style>
