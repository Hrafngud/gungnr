<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
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
}>()

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
  <section class="space-y-2">
    <div class="flex flex-wrap items-center justify-between gap-1">
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Host status
        </h1>
      <div class="flex flex-wrap gap-2">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <UiBadge tone="neutral">
            {{ hostLoading ? 'Refreshing' : 'Live snapshot' }}
          </UiBadge>
         </div>
        <UiButton variant="ghost" @click="emit('refresh')">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh status
          </span>
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
      <UiState v-if="settingsError" tone="error">
        {{ settingsError }}
      </UiState>
      <UiState v-if="jobsError" tone="error">
        {{ jobsError }}
      </UiState>
      <UiState v-if="projectsError" tone="error">
        {{ projectsError }}
      </UiState>
      <div class="flex flex-col gap-3">
        <div class="flex flex-wrap gap-3">
          <UiListRow as="article" class="flex min-w-[210px] flex-1 flex-col gap-2">
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Containers
                </p>
                <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">
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
          <UiListRow as="article" class="flex min-w-[210px] flex-1 flex-col gap-2">
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
          </UiListRow>
          <UiListRow as="article" class="flex min-w-[210px] flex-1 flex-col gap-2">
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
          <UiListRow as="article" class="flex min-w-[210px] flex-1 flex-col gap-2">
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Base Domain
                </p>
                <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                  {{ domainLabel }}
                </p>
              </div>
              <UiBadge tone="neutral">Primary</UiBadge>
            </div>
          </UiListRow>
          <UiListRow as="article" class="flex min-w-[210px] flex-1 flex-col gap-2">
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
        <UiListRow as="article" class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex items-center gap-4">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Jobs
              </p>
              <p class="mt-1 text-base font-semibold text-[color:var(--text)]">
                Automation queue
              </p>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-4 text-xs text-[color:var(--muted)]">
            <span class="flex items-center gap-2">
              <span>Total</span>
              <span class="text-[color:var(--text)]">
                {{ jobCounts.pending + jobCounts.running + jobCounts.completed + jobCounts.failed }}
              </span>
            </span>
            <span class="flex items-center gap-2">
              <span>Queued</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.pending }}</span>
            </span>
            
            <span class="flex items-center gap-2">
              <span>Running</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.running }}</span>
            </span>
            <span class="flex items-center gap-2">
              <span>Completed</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.completed }}</span>
            </span>
            <span class="flex items-center gap-2">
              <span>Failed</span>
              <span class="text-[color:var(--text)]">{{ jobCounts.failed }}</span>
            </span>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{
              lastJob
                ? `Latest: ${lastJob.type} - ${formatDate(lastJob.createdAt)}`
                : 'No job history yet.'
            }}
          </p>
        </UiListRow>
      </div>
  </section>
</template>
