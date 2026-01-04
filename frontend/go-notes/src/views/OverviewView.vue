<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useJobsStore } from '@/stores/jobs'
import { useAuditStore } from '@/stores/audit'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { hostApi } from '@/services/host'
import { apiErrorMessage } from '@/services/api'
import { isPendingJob, jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { DockerContainer } from '@/types/host'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const jobsStore = useJobsStore()
const auditStore = useAuditStore()
const pageLoading = usePageLoadingStore()

const containers = ref<DockerContainer[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)

const isRunningStatus = (status: string) => {
  const normalized = status.toLowerCase()
  return normalized.startsWith('up') || normalized.includes('running')
}

const runningContainers = computed(() =>
  containers.value.filter((container) => isRunningStatus(container.status)),
)
const containerHighlights = computed(() => runningContainers.value.slice(0, 4))
const recentJobs = computed(() => jobsStore.jobs.slice(0, 4))
const recentActivity = computed(() => auditStore.logs.slice(0, 5))

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

const latestJob = computed(() => jobsStore.jobs[0] ?? null)

const containerTone = (status: string): BadgeTone => {
  const normalized = status.toLowerCase()
  if (isRunningStatus(normalized)) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  return 'neutral'
}

const formatDate = (value?: string | null) => {
  if (!value) return 'n/a'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'n/a'
  return date.toLocaleString()
}

const summarizeMetadata = (raw: string) => {
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw)
    const stringified = JSON.stringify(parsed)
    return stringified.length > 140 ? `${stringified.slice(0, 137)}...` : stringified
  } catch {
    return raw.length > 140 ? `${raw.slice(0, 137)}...` : raw
  }
}

const loadContainers = async () => {
  if (containersLoading.value) return
  containersLoading.value = true
  containersError.value = null
  try {
    const { data } = await hostApi.listDocker()
    containers.value = data.containers
  } catch (err) {
    containersError.value = apiErrorMessage(err)
    containers.value = []
  } finally {
    containersLoading.value = false
  }
}

const refreshAll = async () => {
  await Promise.allSettled([
    loadContainers(),
    jobsStore.fetchJobs(),
    auditStore.fetchLogs(),
  ])
}

onMounted(async () => {
  pageLoading.start('Loading overview data...')
  await Promise.allSettled([
    !jobsStore.initialized ? jobsStore.fetchJobs() : Promise.resolve(),
    !auditStore.initialized ? auditStore.fetchLogs() : Promise.resolve(),
    loadContainers(),
  ])
  pageLoading.stop()
})
</script>

<template>
  <section class="page space-y-10">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Overview
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          In-depth host snapshot
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Track container health, automation status, and recent operator activity.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <UiButton variant="ghost" size="sm" @click="refreshAll">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh overview
          </span>
        </UiButton>
        <UiButton :as="RouterLink" to="/host-settings" variant="primary" size="sm">
          Open host settings
        </UiButton>
      </div>
    </div>

    <hr />

    <section class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Containers
          </p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
            Container highlights
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Monitor live services and their latest runtime state.
          </p>
        </div>
        <UiButton variant="ghost" size="sm" @click="loadContainers">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh list
          </span>
        </UiButton>
      </div>

      <UiState v-if="containersError" tone="error">
        {{ containersError }}
      </UiState>

      <UiState v-else-if="containersLoading" loading>
        Loading container inventory...
      </UiState>

      <UiState v-else-if="containerHighlights.length === 0">
        No running containers detected yet.
      </UiState>

      <div v-else class="grid gap-4 md:grid-cols-2">
        <UiPanel
          v-for="container in containerHighlights"
          :key="container.id"
          as="article"
          variant="soft"
          class="space-y-3 p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                {{ container.service || 'Container' }}
              </p>
              <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                {{ container.name }}
              </h3>
            </div>
            <UiBadge :tone="containerTone(container.status)">
              {{ container.status }}
            </UiBadge>
          </div>
          <div class="space-y-2 text-xs text-[color:var(--muted)]">
            <div class="flex items-center justify-between gap-2">
              <span>Ports</span>
              <span class="text-[color:var(--text)]">
                {{ container.ports || 'n/a' }}
              </span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>Project</span>
              <span class="text-[color:var(--text)]">
                {{ container.project || 'n/a' }}
              </span>
            </div>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ container.image }}
          </p>
        </UiPanel>
      </div>

      <UiPanel variant="soft" class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]">
        <span>Need the full container list?</span>
        <UiButton :as="RouterLink" to="/host-settings" variant="ghost" size="sm">
          View host inventory
        </UiButton>
      </UiPanel>
    </section>

    <hr />

    <section class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Jobs
          </p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
            Automation timeline
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Keep tabs on queued deploys, updates, and tunnel actions.
          </p>
        </div>
        <UiButton variant="ghost" size="sm" @click="jobsStore.fetchJobs">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh jobs
          </span>
        </UiButton>
      </div>

      <UiState v-if="jobsStore.error" tone="error">
        {{ jobsStore.error }}
      </UiState>

      <UiState v-else-if="jobsStore.loading" loading>
        Loading job timeline...
      </UiState>

      <UiState v-else-if="jobsStore.jobs.length === 0">
        No automation jobs yet. Queue a template deploy to populate the timeline.
      </UiState>

      <div v-else class="space-y-4">
        <div class="grid gap-4 md:grid-cols-2">
          <UiPanel variant="soft" class="space-y-2 p-4 text-xs text-[color:var(--muted)]">
            <div class="flex items-center justify-between">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Queue health
              </p>
              <UiBadge tone="neutral">{{ jobsStore.jobs.length }} total</UiBadge>
            </div>
            <div class="grid gap-2">
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
          </UiPanel>

          <UiPanel variant="soft" class="space-y-2 p-4 text-xs text-[color:var(--muted)]">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Latest job
            </p>
            <p class="text-sm font-semibold text-[color:var(--text)]">
              {{ latestJob ? latestJob.type : 'No job history yet' }}
            </p>
            <p>
              {{ latestJob ? `Created ${formatDate(latestJob.createdAt)}` : 'Run a deploy to generate logs.' }}
            </p>
            <UiButton
              v-if="latestJob"
              :as="RouterLink"
              :to="`/jobs/${latestJob.id}`"
              variant="ghost"
              size="sm"
              class="mt-2"
            >
              View latest log
            </UiButton>
          </UiPanel>
        </div>

        <div class="space-y-3">
          <UiListRow
            v-for="job in recentJobs"
            :key="job.id"
            as="article"
            class="space-y-3"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ job.type }}
                </p>
                <p class="mt-2 text-sm font-semibold text-[color:var(--text)]">
                  {{ job.error ? 'Attention required' : 'Automation update' }}
                </p>
              </div>
              <UiBadge :tone="jobStatusTone(job.status)">
                {{ jobStatusLabel(job.status) }}
              </UiBadge>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-[color:var(--muted)]">
              <span>Created {{ formatDate(job.createdAt) }}</span>
              <UiButton :as="RouterLink" :to="`/jobs/${job.id}`" variant="ghost" size="sm">
                View log
              </UiButton>
            </div>
            <UiState v-if="job.error" tone="error">
              {{ job.error }}
            </UiState>
          </UiListRow>
        </div>
      </div>
    </section>

    <hr />

    <section class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Activity
          </p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
            Recent operator activity
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Audit trail for deploys, settings, and tunnel changes.
          </p>
        </div>
        <UiButton variant="ghost" size="sm" @click="auditStore.fetchLogs">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh activity
          </span>
        </UiButton>
      </div>

      <UiPanel variant="soft" class="space-y-2 p-4 text-xs text-[color:var(--muted)]">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Day-to-day guidance
        </p>
        <p class="text-sm text-[color:var(--muted)]">
          Check this feed after deploys, DNS updates, or settings changes to confirm the audit trail updated.
        </p>
      </UiPanel>

      <UiState v-if="auditStore.error" tone="error">
        {{ auditStore.error }}
      </UiState>

      <UiState v-else-if="auditStore.loading" loading>
        Loading audit trail...
      </UiState>

      <UiState v-else-if="auditStore.logs.length === 0">
        No audit entries yet. Actions will appear once deploy workflows run.
      </UiState>

      <div v-else class="space-y-3">
        <UiListRow
          v-for="entry in recentActivity"
          :key="entry.id"
          as="article"
          class="space-y-3"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                {{ entry.action }}
              </p>
              <p class="mt-2 text-sm font-semibold text-[color:var(--text)]">
                {{ entry.target || 'System action' }}
              </p>
            </div>
            <UiBadge tone="neutral">Audit</UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ entry.userLogin || 'System' }} - {{ formatDate(entry.createdAt) }}
          </p>
          <p v-if="entry.metadata" class="text-xs text-[color:var(--muted)]">
            {{ summarizeMetadata(entry.metadata) }}
            <UiTooltip :text="entry.metadata">
              <span class="tooltip-trigger ml-2">i</span>
            </UiTooltip>
          </p>
        </UiListRow>
      </div>
    </section>
  </section>
</template>
