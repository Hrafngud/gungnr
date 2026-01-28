<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
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

const dotClass = (status?: string) => {
  switch (healthTone(status)) {
    case 'ok':
      return 'bg-[color:var(--success)]'
    case 'warn':
      return 'bg-[color:var(--warning)]'
    case 'error':
      return 'bg-[color:var(--danger)]'
    default:
      return 'bg-[color:var(--muted-2)]'
  }
}

const dockerSummary = computed(() => {
  const status = statusLabel(dockerHealth.value?.status)
  const count = dockerHealth.value?.containers
  if (typeof count === 'number') {
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
  <div
    class="mt-3 flex flex-col gap-3 text-[11px] text-[color:var(--muted)] sm:text-xs md:flex-row md:flex-wrap md:items-center md:justify-between"
  >
    <div class="flex flex-wrap items-center gap-x-6 gap-y-2">
      <div class="flex items-center gap-2">
      <span class="h-2 w-2 rounded-full" :class="dotClass(dockerHealth?.status)"></span>
      <span class="text-[color:var(--muted-2)]">Docker</span>
      <span class="font-semibold text-[color:var(--text)]">{{ dockerSummary }}</span>
      </div>
      <div class="flex items-center gap-2">
      <span class="h-2 w-2 rounded-full" :class="dotClass(tunnelHealth?.status)"></span>
      <span class="text-[color:var(--muted-2)]">Tunnel</span>
      <span class="font-semibold text-[color:var(--text)]">{{ tunnelSummary }}</span>
      </div>
      <div class="flex items-center gap-2">
      <span class="text-[color:var(--muted-2)]">Domain</span>
      <span class="font-semibold text-[color:var(--text)]">{{ domainLabel }}</span>
      </div>
      <div class="flex items-center gap-2">
      <span class="text-[color:var(--muted-2)]">Jobs</span>
      <span class="font-semibold text-[color:var(--text)]">{{ jobSummary }}</span>
      </div>
      <span v-if="settingsError" class="text-[color:var(--danger)]">
      Host status unavailable
      </span>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <UiButton
        variant="ghost"
        size="chip"
        :disabled="hostLoading"
        @click="loadHostStatus"
      >
        <span class="flex items-center gap-2">
          <NavIcon name="refresh" class="h-3 w-3" />
          {{ hostLoading ? 'Refreshing' : 'Refresh' }}
        </span>
      </UiButton>
      <UiButton
        :as="RouterLink"
        to="/host-settings"
        variant="ghost"
        size="chip"
      >
        <span class="flex items-center gap-2">
          <NavIcon name="host" class="h-3 w-3" />
          Host settings
        </span>
      </UiButton>
    </div>
  </div>
</template>
