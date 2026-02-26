<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
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

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()

const targetMode = ref<NetBirdMode>('legacy')
const targetModeTouched = ref(false)
const allowLocalhost = ref(false)
const apiToken = ref('')
const apiBaseUrl = ref('')
const hostPeerId = ref('')
const adminPeerIdsInput = ref('')
const confirmInput = ref('')
const modeInitialized = ref(false)

const isAdmin = computed(() => authStore.isAdmin)
const status = computed(() => netbirdStore.status.data)
const plan = computed(() => netbirdStore.modePlan.data)
const planLoading = computed(() => netbirdStore.modePlan.loading)
const planError = computed(() => netbirdStore.modePlan.error)
const applySubmitting = computed(() => netbirdStore.modeApply.submitting)
const applyError = computed(() => netbirdStore.modeApply.error)
const applyJob = computed(() => netbirdStore.modeApply.job)

const parsedAdminPeerIds = computed(() =>
  adminPeerIdsInput.value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0),
)

const requiresPeerInputs = computed(() => targetMode.value !== 'legacy')
const confirmationPhrase = computed(() => `apply ${targetMode.value}`)
const confirmationReady = computed(() => confirmInput.value.trim() === confirmationPhrase.value)
const hasApiToken = computed(() => apiToken.value.trim().length > 0)
const hasHostPeerId = computed(() => hostPeerId.value.trim().length > 0)
const hasAdminPeerIds = computed(() => parsedAdminPeerIds.value.length > 0)
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
  hasApiToken.value &&
  (!requiresPeerInputs.value || (hasHostPeerId.value && hasAdminPeerIds.value)) &&
  confirmationReady.value,
)

const isNetBirdMode = (value: string | number): value is NetBirdMode =>
  value === 'legacy' || value === 'mode_a' || value === 'mode_b'

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
    apiToken: apiToken.value.trim(),
    apiBaseUrl: apiBaseUrl.value.trim() || undefined,
    hostPeerId: hostPeerId.value.trim() || undefined,
    adminPeerIds: parsedAdminPeerIds.value.length > 0 ? parsedAdminPeerIds.value : undefined,
  })
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

onMounted(() => {
  if (!status.value) {
    void netbirdStore.loadStatus()
  }
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
      <UiBadge tone="neutral">
        Current: {{ status ? modeLabel(status.currentMode) : 'unknown' }}
      </UiBadge>
    </div>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required for mode planning and apply actions.
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

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          NetBird API token <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="apiToken"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || applySubmitting"
          placeholder="Paste API token"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          NetBird API base URL (optional)
        </span>
        <UiInput
          v-model="apiBaseUrl"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || applySubmitting"
          placeholder="https://api.netbird.io"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Host peer ID
          <span v-if="requiresPeerInputs" class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="hostPeerId"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || applySubmitting || !requiresPeerInputs"
          placeholder="Host peer ID used for panel/project groups"
        />
      </label>

      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Admin peer IDs (comma separated)
          <span v-if="requiresPeerInputs" class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="adminPeerIdsInput"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :disabled="!isAdmin || applySubmitting || !requiresPeerInputs"
          placeholder="peer-id-1,peer-id-2"
        />
      </label>

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

      <UiState v-if="isAdmin && !hasApiToken" tone="warn">
        API token is required to queue apply.
      </UiState>
      <UiState v-if="isAdmin && requiresPeerInputs && !hasHostPeerId" tone="warn">
        Host peer ID is required for Mode A and Mode B.
      </UiState>
      <UiState v-if="isAdmin && requiresPeerInputs && !hasAdminPeerIds" tone="warn">
        At least one admin peer ID is required for Mode A and Mode B.
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
      <UiState v-if="applyJob" tone="ok">
        Mode apply queued as job #{{ applyJob.id }}.
      </UiState>
      <UiButton
        v-if="applyJob"
        :as="RouterLink"
        :to="`/jobs/${applyJob.id}`"
        variant="ghost"
        size="sm"
      >
        Open job log
      </UiButton>
    </UiPanel>
  </UiPanel>
</template>
