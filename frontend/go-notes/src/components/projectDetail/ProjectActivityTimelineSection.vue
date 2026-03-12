<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { apiErrorMessage } from '@/services/api'
import { jobsApi } from '@/services/jobs'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { Job, JobDetail, JobListResponse } from '@/types/jobs'

const props = defineProps<{
  projectName: string
  projectDisplayName: string
}>()

const toastStore = useToastStore()
const projectJobs = ref<Job[]>([])
const jobsLoading = ref(false)
const jobsError = ref<string | null>(null)
const jobsPage = ref(1)
const jobsTotal = ref(0)
const jobsTotalPages = ref(0)
const jobsPageSize = 8

const jobLogsPanelOpen = ref(false)
const selectedJobId = ref<number | null>(null)
const selectedJob = ref<JobDetail | null>(null)
const selectedJobLoading = ref(false)
const selectedJobError = ref<string | null>(null)
const projectLogFontSizes = [11, 12, 13, 14] as const
const projectJobLogFontSize = ref<number>(12)

const canGoJobsBack = computed(() => jobsPage.value > 1)
const canGoJobsForward = computed(() => jobsTotalPages.value > 0 && jobsPage.value < jobsTotalPages.value)
const selectedJobLogOutput = computed(() => selectedJob.value?.logLines?.join('\n') ?? '')

function fmtDate(value?: string | null): string {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

function applyProjectJobsResponse(data: JobListResponse) {
  projectJobs.value = data.jobs ?? []
  jobsPage.value = data.page ?? 1
  jobsTotal.value = data.total ?? 0
  jobsTotalPages.value = data.totalPages ?? 0
}

async function loadProjectJobs(page = 1) {
  if (!props.projectName) {
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = 1
    jobsError.value = 'Invalid project name.'
    return
  }

  jobsLoading.value = true
  jobsError.value = null
  try {
    const { data } = await projectsApi.listJobs(props.projectName, { page, limit: jobsPageSize })
    applyProjectJobsResponse(data)
  } catch (err) {
    jobsError.value = apiErrorMessage(err)
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = page
  } finally {
    jobsLoading.value = false
  }
}

async function openJobLogs(jobId: number) {
  selectedJobId.value = jobId
  selectedJob.value = null
  selectedJobError.value = null
  jobLogsPanelOpen.value = true
  await refreshSelectedJobLogs()
}

async function refreshSelectedJobLogs() {
  if (!selectedJobId.value) return

  selectedJobLoading.value = true
  selectedJobError.value = null
  try {
    const { data } = await jobsApi.get(selectedJobId.value)
    selectedJob.value = data
  } catch (err) {
    selectedJobError.value = apiErrorMessage(err)
    selectedJob.value = null
  } finally {
    selectedJobLoading.value = false
  }
}

async function copyTextToClipboard(payload: string) {
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(payload)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = payload
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

async function copySelectedJobLogs() {
  const output = selectedJobLogOutput.value
  if (!output) {
    toastStore.warn('No logs to copy yet.', 'Nothing to copy')
    return
  }

  try {
    await copyTextToClipboard(output)
    toastStore.success('Logs copied to clipboard.', 'Copied')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

function cycleProjectJobLogFontSize() {
  const currentIndex = projectLogFontSizes.findIndex((size) => size === projectJobLogFontSize.value)
  const nextIndex = currentIndex === -1 ? 0 : (currentIndex + 1) % projectLogFontSizes.length
  projectJobLogFontSize.value = projectLogFontSizes[nextIndex] ?? projectLogFontSizes[0]
}

async function goToJobsPage(nextPage: number) {
  if (nextPage < 1) return
  if (jobsTotalPages.value > 0 && nextPage > jobsTotalPages.value) return
  await loadProjectJobs(nextPage)
}

watch(
  () => props.projectName,
  () => {
    projectJobs.value = []
    jobsLoading.value = false
    jobsError.value = null
    jobsPage.value = 1
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobLogsPanelOpen.value = false
    selectedJobId.value = null
    selectedJob.value = null
    selectedJobError.value = null
    selectedJobLoading.value = false
    void loadProjectJobs(1)
  },
  { immediate: true },
)

watch(jobLogsPanelOpen, (open) => {
  if (open) return
  selectedJobId.value = null
  selectedJob.value = null
  selectedJobError.value = null
  selectedJobLoading.value = false
})

</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project jobs</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Activity timeline</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          {{ jobsTotal }} total jobs for {{ projectDisplayName || projectName }}.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" :disabled="jobsLoading" @click="loadProjectJobs(jobsPage)">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="jobsLoading" />
          Refresh jobs
        </span>
      </UiButton>
    </div>

    <UiState v-if="jobsError" tone="error">{{ jobsError }}</UiState>
    <UiState v-else-if="jobsLoading" loading>Loading project jobs...</UiState>
    <UiState v-else-if="projectJobs.length === 0">No jobs have been recorded for this project yet.</UiState>

    <div v-else class="space-y-3">
      <UiListRow
        v-for="job in projectJobs"
        :key="job.id"
        as="article"
        class="space-y-4"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Job #{{ job.id }}</p>
            <h3 class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ job.type }}</h3>
          </div>
          <UiBadge :tone="jobStatusTone(job.status)">
            {{ jobStatusLabel(job.status) }}
          </UiBadge>
        </div>

        <div class="mt-4 grid gap-2 text-xs text-[color:var(--muted)] sm:grid-cols-3">
          <p>Created: <span class="text-[color:var(--text)]">{{ fmtDate(job.createdAt) }}</span></p>
          <p>Started: <span class="text-[color:var(--text)]">{{ fmtDate(job.startedAt) }}</span></p>
          <p>Finished: <span class="text-[color:var(--text)]">{{ fmtDate(job.finishedAt) }}</span></p>
        </div>

        <div class="mt-4 flex flex-wrap items-center gap-2">
          <UiButton variant="ghost" size="sm" @click="openJobLogs(job.id)">
            View job logs
          </UiButton>
          <UiButton :as="RouterLink" :to="`/jobs/${job.id}`" variant="ghost" size="sm">
            Open job page
          </UiButton>
        </div>
      </UiListRow>
    </div>

    <div
      v-if="jobsTotalPages > 1 && !jobsLoading"
      class="flex flex-wrap items-center justify-between gap-3 bg-[color:var(--surface-2)] px-4 py-3 text-xs text-[color:var(--muted)]"
    >
      <span>Page {{ jobsPage }} of {{ jobsTotalPages }}</span>
      <div class="flex items-center gap-2">
        <UiButton variant="ghost" size="sm" :disabled="!canGoJobsBack" @click="goToJobsPage(jobsPage - 1)">
          Previous
        </UiButton>
        <UiButton variant="ghost" size="sm" :disabled="!canGoJobsForward" @click="goToJobsPage(jobsPage + 1)">
          Next
        </UiButton>
      </div>
    </div>
  </UiPanel>

  <UiFormSidePanel
    v-model="jobLogsPanelOpen"
    eyebrow="Project jobs"
    :title="selectedJobId ? `Job #${selectedJobId} logs` : 'Job logs'"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Log viewer</p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            {{ selectedJob ? selectedJob.type : 'Select a job entry to load logs.' }}
          </p>
        </div>
        <UiBadge v-if="selectedJob" :tone="jobStatusTone(selectedJob.status)">
          {{ jobStatusLabel(selectedJob.status) }}
        </UiBadge>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <UiButton variant="ghost" size="sm" :disabled="selectedJobLoading" @click="refreshSelectedJobLogs">
          <span class="inline-flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="selectedJobLoading" />
            Refresh
          </span>
        </UiButton>
        <UiButton variant="ghost" size="sm" :disabled="!selectedJobLogOutput" @click="copySelectedJobLogs">
          Copy to clipboard
        </UiButton>
        <UiButton variant="ghost" size="sm" @click="cycleProjectJobLogFontSize">
          Log size: {{ projectJobLogFontSize }}px
        </UiButton>
      </div>

      <UiState v-if="selectedJobError" tone="error">{{ selectedJobError }}</UiState>
      <UiState v-else-if="selectedJobLoading && !selectedJob" loading>Loading job logs...</UiState>

      <pre
        v-else
        class="max-h-[70vh] overflow-auto bg-[color:var(--surface-2)] p-4 text-[color:var(--text)]"
        :style="{ fontSize: `${projectJobLogFontSize}px`, lineHeight: '1.45' }"
      ><code>{{ selectedJobLogOutput || 'No logs yet. Try refresh if the job is still running.' }}</code></pre>
    </div>
  </UiFormSidePanel>
</template>
