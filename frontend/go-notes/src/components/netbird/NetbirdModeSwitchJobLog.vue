<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useNetbirdStore } from '@/stores/netbird'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'

const netbirdStore = useNetbirdStore()

const status = computed(() => netbirdStore.status.data)
const applyJob = computed(() => netbirdStore.modeApply.job)
const applyPolling = computed(() => netbirdStore.modeApplyPolling)

const syncStatusToJobStatus = (value?: string): string => {
  const normalized = (value || '').trim().toLowerCase()
  if (normalized === 'pending') return 'pending'
  if (normalized === 'succeeded') return 'completed'
  if (normalized === 'failed') return 'failed'
  return ''
}

const hasModeSwitchAssignedJob = computed(() => {
  const statusJobId = status.value?.lastPolicySyncJobId ?? 0
  if (statusJobId > 0) return true

  if (applyJob.value?.type === 'netbird_mode_apply' && applyJob.value.id > 0) {
    return true
  }

  if (applyPolling.value.lastJob?.type === 'netbird_mode_apply' && applyPolling.value.lastJob.id > 0) {
    return true
  }

  return false
})

const applyPollingJobId = computed(() => {
  if (!hasModeSwitchAssignedJob.value) return null
  return applyPolling.value.jobId ?? applyJob.value?.id ?? status.value?.lastPolicySyncJobId ?? null
})

const applyPollingJob = computed(() => {
  const jobID = applyPollingJobId.value
  const job = applyPolling.value.lastJob
  if (!jobID || !job) return null
  return job.id === jobID ? job : null
})

const fallbackStatus = computed(() => syncStatusToJobStatus(status.value?.lastPolicySyncStatus))
const applyPollingStatus = computed(
  () =>
    applyPollingJob.value?.status ??
    applyJob.value?.status ??
    fallbackStatus.value ??
    (applyPolling.value.lifecycle === 'running' ? 'pending' : ''),
)
const applyPollingStatusLabel = computed(() => jobStatusLabel(applyPollingStatus.value))
const applyPollingStatusTone = computed(() => jobStatusTone(applyPollingStatus.value))

const applyPollingSummary = computed(() => {
  if (!applyPolling.value.summary) return null
  if (applyPolling.value.jobId && applyPolling.value.jobId === applyPollingJobId.value) {
    return applyPolling.value.summary
  }
  if (applyPollingJob.value) {
    return applyPolling.value.summary
  }
  return null
})

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

const applyPollingLifecycle = computed<'idle' | 'running' | 'terminal' | 'error'>(() => {
  const lifecycle = applyPolling.value.lifecycle
  if (lifecycle === 'running' || lifecycle === 'error' || lifecycle === 'terminal') {
    return lifecycle
  }

  const normalizedStatus = applyPollingStatus.value.trim().toLowerCase()
  if (normalizedStatus === 'pending' || normalizedStatus === 'running') return 'running'
  if (normalizedStatus === 'completed' || normalizedStatus === 'failed') return 'terminal'
  return 'idle'
})

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

const retryApplyPolling = () => {
  if (!applyPollingJobId.value) return
  void netbirdStore.startModeApplyJobPolling(applyPollingJobId.value)
}
</script>

<template>
  <UiPanel
    v-if="hasModeSwitchAssignedJob && applyPollingJobId"
    as="article"
    variant="soft"
    class="space-y-3 p-5"
  >
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Job #{{ applyPollingJobId }}
        </p>
        <UiBadge :tone="applyPollingStatusTone">
          {{ applyPollingStatusLabel }}
        </UiBadge>
      </div>
      <UiButton
        :as="RouterLink"
        :to="`/jobs/${applyPollingJobId}`"
        variant="ghost"
        size="sm"
      >
        View log
      </UiButton>
    </div>

    <UiState v-if="applyPolling.lifecycle === 'error'" tone="error">
      {{ applyPolling.error || 'Mode apply polling failed.' }}
    </UiState>
    <UiState v-else-if="applyPollingLifecycle === 'running'" loading>
      {{ applyPollingRunningMessage }}
    </UiState>
    <UiState
      v-else-if="applyPollingLifecycle === 'terminal'"
      :tone="applyPollingTerminalTone"
    >
      {{ applyPollingTerminalMessage }}
      <span v-if="applyPollingHasWarnings">
        ({{ applyPollingWarnings.length }} warnings, {{ applyPollingFailureCount }} failures)
      </span>
    </UiState>

    <UiButton
      v-if="applyPolling.lifecycle === 'error'"
      variant="ghost"
      size="sm"
      @click="retryApplyPolling"
    >
      Retry polling
    </UiButton>
  </UiPanel>
</template>
