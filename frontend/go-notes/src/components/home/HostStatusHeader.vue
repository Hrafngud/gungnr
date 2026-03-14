<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiStatusDot from '@/components/ui/UiStatusDot.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useJobsStore } from '@/stores/jobs'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import { isPendingJob } from '@/utils/jobStatus'
import type { DockerHealth, TunnelHealth } from '@/types/health'
import type { Settings } from '@/types/settings'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const jobsStore = useJobsStore()

const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const settings = ref<Settings | null>(null)
const hostLoading = ref(false)
const settingsError = ref<string | null>(null)

const loadHostStatus = async () => {
  hostLoading.value = true
  settingsError.value = null
  const [dockerResult, tunnelResult, settingsResult] = await Promise.allSettled([
    healthApi.docker(),
    healthApi.tunnel(),
    settingsApi.get(),
  ])
  if (dockerResult.status === 'fulfilled') {
    dockerHealth.value = dockerResult.value.data
  } else {
    dockerHealth.value = { status: 'error', detail: apiErrorMessage(dockerResult.reason) }
  }
  if (tunnelResult.status === 'fulfilled') {
    tunnelHealth.value = tunnelResult.value.data
  } else {
    tunnelHealth.value = { status: 'error', detail: apiErrorMessage(tunnelResult.reason) }
  }
  if (settingsResult.status === 'fulfilled') {
    settings.value = settingsResult.value.data.settings
  } else {
    settingsError.value = apiErrorMessage(settingsResult.reason)
  }
  hostLoading.value = false
}

onMounted(async () => {
  await loadHostStatus()
  if (!jobsStore.initialized) {
    await jobsStore.fetchJobs()
  }
})

const jobCounts = computed(() => {
  const counts = {
    pending: 0,
    running: 0,
    completed: 0,
    failed: 0,
  }
  jobsStore.jobs.forEach((job) => {
    if (isPendingJob(job.status)) counts.pending += 1
    else if (job.status === 'running') counts.running += 1
    else if (job.status === 'completed') counts.completed += 1
    else if (job.status === 'failed') counts.failed += 1
  })
  return counts
})

const activeJobs = computed(() => jobCounts.value.pending + jobCounts.value.running)

const domainLabel = computed(() => settings.value?.baseDomain || 'n/a')

const healthTone = (status?: string): BadgeTone => {
  switch (status) {
    case 'ok':
      return 'ok'
    case 'warning':
      return 'warn'
    case 'error':
      return 'error'
    case 'missing':
      return 'neutral'
    default:
      return 'neutral'
  }
}

const statusLabel = (status?: string) => {
  switch (status) {
    case 'ok':
      return 'ok'
    case 'warning':
      return 'warn'
    case 'error':
      return 'down'
    case 'missing':
      return 'missing'
    default:
      return 'unknown'
  }
}

const dockerSummary = computed(() => {
  const status = statusLabel(dockerHealth.value?.status)
  const count = dockerHealth.value?.containers
  const rawStatus = dockerHealth.value?.status
  if (typeof count === 'number' && (rawStatus === 'ok' || rawStatus === 'warning')) {
    return `${status} · ${count} containers`
  }
  return status
})

const tunnelSummary = computed(() => {
  const status = statusLabel(tunnelHealth.value?.status)
  const name = tunnelHealth.value?.tunnel
  return name ? `${status} · ${name}` : status
})

const jobSummary = computed(() => {
  if (activeJobs.value === 0) return '0 active'
  return `${activeJobs.value} active`
})
</script>

<template>
  <div class="min-w-0 text-[10px] text-[color:var(--muted)] sm:text-[11px]">
    <div class="flex items-center gap-2">
      <div
        class="host-status-track flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto whitespace-nowrap pr-1"
        role="status"
        aria-live="polite"
      >
        <div class="inline-flex items-center gap-1.5 rounded-full border border-[color:var(--border)] bg-[color:var(--surface)] px-2.5 py-1">
          <UiStatusDot size="xs" palette="semantic" :tone="healthTone(dockerHealth?.status)" />
          <span class="text-[color:var(--muted-2)]">Docker</span>
          <span class="font-semibold text-[color:var(--text)]">{{ dockerSummary }}</span>
        </div>
        <div class="inline-flex items-center gap-1.5 rounded-full border border-[color:var(--border)] bg-[color:var(--surface)] px-2.5 py-1">
          <UiStatusDot size="xs" palette="semantic" :tone="healthTone(tunnelHealth?.status)" />
          <span class="text-[color:var(--muted-2)]">Tunnel</span>
          <span class="font-semibold text-[color:var(--text)]">{{ tunnelSummary }}</span>
        </div>
        <div
          class="hidden items-center gap-1.5 rounded-full border border-[color:var(--border)] bg-[color:var(--surface)] px-2.5 py-1 lg:inline-flex"
        >
          <span class="text-[color:var(--muted-2)]">Domain</span>
          <span class="font-semibold text-[color:var(--text)]">{{ domainLabel }}</span>
        </div>
        <div
          class="hidden items-center gap-1.5 rounded-full border border-[color:var(--border)] bg-[color:var(--surface)] px-2.5 py-1 md:inline-flex"
        >
          <span class="text-[color:var(--muted-2)]">Jobs</span>
          <span class="font-semibold text-[color:var(--text)]">{{ jobSummary }}</span>
        </div>
        <span v-if="settingsError" class="pl-1 text-[color:var(--danger)]">
          Host status unavailable
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-1.5">
        <UiButton
          variant="ghost"
          size="chip"
          :disabled="hostLoading"
          :aria-label="hostLoading ? 'Refreshing host status' : 'Refresh host status'"
          @click="loadHostStatus"
        >
          <span class="flex items-center gap-1.5">
            <NavIcon name="refresh" class="h-3 w-3" />
            <span class="hidden sm:inline">{{ hostLoading ? 'Refreshing' : 'Refresh' }}</span>
          </span>
        </UiButton>
        <UiButton
          :as="RouterLink"
          to="/host-settings"
          variant="ghost"
          size="chip"
          aria-label="Host settings"
        >
          <span class="flex items-center gap-1.5">
            <NavIcon name="host" class="h-3 w-3" />
            <span class="hidden sm:inline">Host settings</span>
          </span>
        </UiButton>
        <slot name="right" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.host-status-track {
  scrollbar-width: none;
}

.host-status-track::-webkit-scrollbar {
  display: none;
}
</style>
