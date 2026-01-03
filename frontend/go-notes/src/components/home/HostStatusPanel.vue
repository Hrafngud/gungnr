<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { DockerHealth, TunnelHealth } from '@/types/health'
import type { Settings } from '@/types/settings'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

interface JobCounts {
  pending: number
  running: number
  completed: number
  failed: number
}

interface LastJob {
  type: string
  createdAt: string
}

interface LastProject {
  name: string
  updatedAt?: string
  createdAt: string
}

const props = defineProps<{
  machineName: string
  dockerHealth: DockerHealth | null
  tunnelHealth: TunnelHealth | null
  settings: Settings | null
  hostLoading: boolean
  settingsError: string | null
  jobsError: string | null
  projectsError: string | null
  jobCounts: JobCounts
  lastJob: LastJob | null
  lastProject: LastProject | null
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'start-onboarding'): void
}>()

const jobTone = computed<BadgeTone>(() => {
  if (props.jobCounts.failed > 0) return 'error'
  if (props.jobCounts.running > 0) return 'warn'
  if (props.jobCounts.pending > 0) return 'neutral'
  if (props.jobCounts.completed > 0) return 'ok'
  return 'neutral'
})

const lastServiceLabel = computed(() => props.lastProject?.name ?? 'n/a')

const lastServiceTime = computed(() => {
  if (!props.lastProject) return 'No deployments yet.'
  const stamp = props.lastProject.updatedAt || props.lastProject.createdAt
  return `Updated ${formatDate(stamp)}`
})

const domainLabel = computed(() => props.settings?.baseDomain || 'n/a')

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

const formatDate = (value?: string | null) => {
  if (!value) return 'n/a'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'n/a'
  return date.toLocaleString()
}
</script>

<template>
  <section class="space-y-12">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Home
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Host status
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Monitor the deploy surface and keep automation primed for your next stack.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <UiButton variant="ghost" @click="emit('refresh')">
          Refresh status
        </UiButton>
        <UiButton
          :as="RouterLink"
          to="/host-settings"
          variant="primary"
        >
          Open host settings
        </UiButton>
      </div>
    </div>
    <hr />
    <UiPanel variant="soft" class="space-y-4 p-4" data-onboard="home-status">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Host status
          </p>
          <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
            Live runtime signals
          </h2>
        </div>
        <UiBadge tone="neutral">
          {{ hostLoading ? 'Refreshing' : 'Live snapshot' }}
        </UiBadge>
      </div>
      <UiState v-if="settingsError" tone="error">
        {{ settingsError }}
      </UiState>
      <UiState v-if="jobsError" tone="error">
        {{ jobsError }}
      </UiState>
      <UiState v-if="projectsError" tone="error">
        {{ projectsError }}
      </UiState>
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Containers
              </p>
              <p class="mt-1 text-xl font-semibold text-[color:var(--text)]">
                {{ dockerHealth?.containers ?? 'n/a' }}
              </p>
            </div>
            <UiBadge :tone="healthTone(dockerHealth?.status)">
              {{ dockerHealth?.status || 'unknown' }}
            </UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ dockerHealth?.detail || 'No container data available yet.' }}
          </p>
        </UiListRow>
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Jobs
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                Automation queue
              </p>
            </div>
            <UiBadge :tone="jobTone">
              {{ jobCounts.pending + jobCounts.running + jobCounts.completed + jobCounts.failed }} total
            </UiBadge>
          </div>
          <div class="grid gap-1 text-xs text-[color:var(--muted)]">
            <div class="flex items-center justify-between">
              <span>Queued</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.pending }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span>Running</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.running }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span>Completed</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.completed }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span>Failed</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.failed }}</span>
            </div>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{
              lastJob
                ? `Latest: ${lastJob.type} - ${formatDate(lastJob.createdAt)}`
                : 'No job history yet.'
            }}
          </p>
        </UiListRow>
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Machine
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                {{ machineName || 'n/a' }}
              </p>
            </div>
            <UiBadge tone="neutral">Panel host</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            Hostname pulled from the active panel URL.
          </p>
        </UiListRow>
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Tunnel
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                {{ tunnelHealth?.tunnel || 'n/a' }}
              </p>
            </div>
            <UiBadge :tone="healthTone(tunnelHealth?.status)">
              {{ tunnelHealth?.status || 'unknown' }}
            </UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ tunnelHealth?.detail || 'Cloudflared status unavailable.' }}
          </p>
        </UiListRow>
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Domain
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                {{ domainLabel }}
              </p>
            </div>
            <UiBadge tone="neutral">Primary</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            Used for new subdomains and host tunnel ingress.
          </p>
        </UiListRow>
        <UiListRow as="article" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Last service
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                {{ lastServiceLabel }}
              </p>
            </div>
            <UiBadge tone="neutral">Deploy</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ lastServiceTime }}
          </p>
        </UiListRow>
      </div>
      <UiListRow class="flex flex-wrap items-center justify-between gap-3" data-onboard="home-onboarding">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Onboarding
          </p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            Finish host setup to unlock host tunnel automation and DNS updates.
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton :as="RouterLink" to="/host-settings" variant="primary" size="sm">
            Configure host
          </UiButton>
          <UiButton variant="ghost" size="sm" @click="emit('start-onboarding')">
            View guide
          </UiButton>
        </div>
      </UiListRow>
    </UiPanel>
  </section>
</template>
