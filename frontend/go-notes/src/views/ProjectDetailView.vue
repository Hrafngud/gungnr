<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import NavIcon from '@/components/NavIcon.vue'
import { jobsApi } from '@/services/jobs'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import { jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { Job, JobDetail, JobListResponse } from '@/types/jobs'
import type {
  ProjectArchiveOptions,
  ProjectArchivePlan,
  ProjectContainer,
  ProjectDetail,
} from '@/types/projects'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const route = useRoute()
const authStore = useAuthStore()
const toastStore = useToastStore()
const pageLoading = usePageLoadingStore()
const loading = ref(false)
const error = ref<string | null>(null)
const detail = ref<ProjectDetail | null>(null)
const stackRestarting = ref(false)
const stackRestartError = ref<string | null>(null)
const archivePlan = ref<ProjectArchivePlan | null>(null)
const archivePlanLoading = ref(false)
const archivePlanError = ref<string | null>(null)
const archiveExecuting = ref(false)
const archiveExecuteError = ref<string | null>(null)
const archiveExecutedWithWarnings = ref(false)
const archiveOptions = ref<ProjectArchiveOptions>({
  removeContainers: true,
  removeVolumes: false,
  removeIngress: true,
  removeDns: true,
})
const archiveConfirmInput = ref('')
const isAdmin = computed(() => authStore.isAdmin)

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

const projectName = computed(() => {
  const raw = route.params.name
  if (typeof raw !== 'string') return ''
  return decodeURIComponent(raw).trim()
})

const canGoJobsBack = computed(() => jobsPage.value > 1)
const canGoJobsForward = computed(() => jobsTotalPages.value > 0 && jobsPage.value < jobsTotalPages.value)
const selectedJobLogOutput = computed(() => selectedJob.value?.logLines?.join('\n') ?? '')
const archiveConfirmationPhrase = computed(() => {
  const normalized = (detail.value?.project.normalizedName || projectName.value || '').toLowerCase().trim()
  if (!normalized) return 'ARCHIVE PROJECT'
  return `ARCHIVE ${normalized}`
})
const canSubmitArchive = computed(() => {
  if (!isAdmin.value || archiveExecuting.value) return false
  if (archiveOptions.value.removeVolumes && !archiveOptions.value.removeContainers) return false
  return archiveConfirmInput.value.trim() === archiveConfirmationPhrase.value
})

const statusTone = (status: string): BadgeTone => {
  const normalized = status.trim().toLowerCase()
  if (!normalized) return 'neutral'
  if (normalized === 'running' || normalized === 'up' || normalized.includes('running')) return 'ok'
  if (normalized.includes('failed') || normalized.includes('error')) return 'error'
  if (normalized.includes('pending') || normalized.includes('building')) return 'warn'
  return 'neutral'
}

const containerTone = (container: ProjectContainer): BadgeTone => {
  const normalized = container.status.trim().toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  if (normalized.includes('paused') || normalized.includes('restarting')) return 'warn'
  return 'neutral'
}

const fmtDate = (value?: string | null) => {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

const applyProjectJobsResponse = (data: JobListResponse) => {
  projectJobs.value = data.jobs ?? []
  jobsPage.value = data.page ?? 1
  jobsTotal.value = data.total ?? 0
  jobsTotalPages.value = data.totalPages ?? 0
}

const loadProjectJobs = async (page = 1) => {
  const name = projectName.value
  if (!name) {
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
    const { data } = await projectsApi.listJobs(name, { page, limit: jobsPageSize })
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

const applyArchiveDefaults = (plan: ProjectArchivePlan) => {
  archiveOptions.value = {
    removeContainers: plan.defaults.removeContainers,
    removeVolumes: plan.defaults.removeVolumes,
    removeIngress: plan.defaults.removeIngress,
    removeDns: plan.defaults.removeDns,
  }
}

const loadArchivePlan = async () => {
  const name = projectName.value
  if (!name) {
    archivePlan.value = null
    archivePlanError.value = 'Invalid project name.'
    return
  }
  if (!isAdmin.value) {
    archivePlan.value = null
    archivePlanError.value = null
    return
  }

  archivePlanLoading.value = true
  archivePlanError.value = null
  try {
    const { data } = await projectsApi.getArchivePlan(name)
    archivePlan.value = data.plan
    applyArchiveDefaults(data.plan)
  } catch (err) {
    archivePlan.value = null
    archivePlanError.value = apiErrorMessage(err)
  } finally {
    archivePlanLoading.value = false
  }
}

const load = async () => {
  const name = projectName.value
  if (!name) {
    error.value = 'Invalid project name.'
    detail.value = null
    return
  }

  loading.value = true
  error.value = null
  pageLoading.start(`Loading project ${name}...`)
  try {
    const { data } = await projectsApi.getDetail(name)
    detail.value = data
    await loadProjectJobs(1)
    await loadArchivePlan()
  } catch (err) {
    detail.value = null
    error.value = apiErrorMessage(err)
    projectJobs.value = []
    jobsTotal.value = 0
    jobsTotalPages.value = 0
    jobsPage.value = 1
    archivePlan.value = null
    archivePlanError.value = null
  } finally {
    loading.value = false
    pageLoading.stop()
  }
}

const restartStack = async () => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Restart blocked')
    return
  }
  if (stackRestarting.value) return

  stackRestartError.value = null
  stackRestarting.value = true
  try {
    const { data } = await projectsApi.restartStack(name)
    toastStore.success(`Project "${name}" restart queued (job #${data.job.id}).`, 'Docker compose')
    await loadProjectJobs(1)
  } catch (err) {
    const message = apiErrorMessage(err)
    stackRestartError.value = message
    toastStore.error(message, 'Queue failed')
  } finally {
    stackRestarting.value = false
  }
}

const queueArchive = async () => {
  const name = projectName.value
  if (!name) return
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Archive blocked')
    return
  }
  if (!canSubmitArchive.value) return

  archiveExecuteError.value = null
  archiveExecuting.value = true
  try {
    const payload = {
      removeContainers: archiveOptions.value.removeContainers,
      removeVolumes: archiveOptions.value.removeVolumes,
      removeIngress: archiveOptions.value.removeIngress,
      removeDns: archiveOptions.value.removeDns,
    }
    const { data } = await projectsApi.archiveProject(name, payload)
    archivePlan.value = data.plan
    archiveExecutedWithWarnings.value = (data.plan.warnings?.length ?? 0) > 0
    archiveConfirmInput.value = ''

    if (archiveExecutedWithWarnings.value) {
      toastStore.warn(
        `Archive queued (job #${data.job.id}) with ${data.plan.warnings.length} warning(s) in plan preview.`,
        'Archive queued',
      )
    } else {
      toastStore.success(`Archive queued (job #${data.job.id}).`, 'Project cleanup')
    }
    await load()
  } catch (err) {
    const message = apiErrorMessage(err)
    archiveExecuteError.value = message
    toastStore.error(message, 'Archive queue failed')
  } finally {
    archiveExecuting.value = false
  }
}

const openJobLogs = async (jobId: number) => {
  selectedJobId.value = jobId
  selectedJob.value = null
  selectedJobError.value = null
  jobLogsPanelOpen.value = true
  await refreshSelectedJobLogs()
}

const refreshSelectedJobLogs = async () => {
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

const copySelectedJobLogs = async () => {
  const output = selectedJobLogOutput.value
  if (!output) {
    toastStore.warn('No logs to copy yet.', 'Nothing to copy')
    return
  }

  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(output)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = output
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.focus()
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    toastStore.success('Logs copied to clipboard.', 'Copied')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const cycleProjectJobLogFontSize = () => {
  const currentIndex = projectLogFontSizes.findIndex((size) => size === projectJobLogFontSize.value)
  const nextIndex = currentIndex === -1 ? 0 : (currentIndex + 1) % projectLogFontSizes.length
  projectJobLogFontSize.value = projectLogFontSizes[nextIndex] ?? projectLogFontSizes[0]
}

const goToJobsPage = async (nextPage: number) => {
  if (nextPage < 1) return
  if (jobsTotalPages.value > 0 && nextPage > jobsTotalPages.value) return
  await loadProjectJobs(nextPage)
}

onMounted(load)
watch(projectName, () => {
  stackRestartError.value = null
  archivePlan.value = null
  archivePlanError.value = null
  archiveExecuteError.value = null
  archiveExecuting.value = false
  archiveExecutedWithWarnings.value = false
  archiveConfirmInput.value = ''
  jobLogsPanelOpen.value = false
  void load()
})

watch(jobLogsPanelOpen, (open) => {
  if (open) return
  selectedJobId.value = null
  selectedJob.value = null
  selectedJobError.value = null
  selectedJobLoading.value = false
})

watch(
  () => archiveOptions.value.removeContainers,
  (enabled) => {
    if (enabled) return
    archiveOptions.value.removeVolumes = false
  },
)
</script>

<template>
  <section class="page space-y-8">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project workspace</p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">{{ projectName || 'Project detail' }}</h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Runtime metadata, containers, and job history for this deployment.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <RouterLink to="/projects" class="btn btn-ghost px-3 py-2 text-xs font-semibold">
          <span class="inline-flex items-center gap-2">
            <NavIcon name="arrow-left" class="h-3.5 w-3.5" />
            Back
          </span>
        </RouterLink>
        <UiButton variant="ghost" size="sm" @click="load">
          <span class="inline-flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            Refresh
          </span>
        </UiButton>
      </div>
    </header>

    <UiState v-if="loading" :loading="true">Loading project detail...</UiState>
    <UiState v-else-if="error" tone="error">{{ error }}</UiState>

    <template v-else-if="detail">
      <UiPanel
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Workspace guidance</p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            Read access is available to all authenticated users. Restart actions require admin permissions.
          </p>
        </div>
        <UiBadge :tone="statusTone(detail.project.record?.status || '')">
          {{ detail.project.record?.status || 'unknown' }}
        </UiBadge>
      </UiPanel>

      <hr />

      <div class="grid gap-5 xl:grid-cols-3">
        <UiPanel class="space-y-5 p-6 xl:col-span-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project profile</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">General</h2>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Project</p>
              <p class="text-base font-semibold text-[color:var(--text)]">{{ detail.project.name }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Normalized</p>
              <p class="font-mono text-sm text-[color:var(--text)]">{{ detail.project.normalizedName }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Last updated</p>
              <p class="text-sm text-[color:var(--muted)]">{{ fmtDate(detail.project.record?.updatedAt) }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Source</p>
              <p class="text-sm text-[color:var(--text)]">{{ detail.runtime.source || 'unknown' }}</p>
            </div>
            <div class="space-y-1 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Path</p>
              <p class="font-mono text-xs text-[color:var(--muted)] break-all">{{ detail.runtime.path }}</p>
            </div>
            <div class="space-y-1 sm:col-span-2">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Repository</p>
              <p class="text-sm text-[color:var(--muted)] break-all">
                {{ detail.project.record?.repoUrl || 'No repository URL recorded' }}
              </p>
            </div>
          </div>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-4 p-5">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Runtime</p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Compose and env</h2>
          </div>
          <div class="space-y-2 text-sm text-[color:var(--muted)]">
            <div class="flex items-center justify-between gap-2">
              <span>Compose files</span>
              <span class="font-semibold text-[color:var(--text)]">{{ detail.runtime.composeFiles.length }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>.env</span>
              <span class="font-semibold text-[color:var(--text)]">{{ detail.runtime.envExists ? 'present' : 'missing' }}</span>
            </div>
            <p class="break-all font-mono text-xs text-[color:var(--muted-2)]">{{ detail.runtime.envPath }}</p>
          </div>
          <UiPanel variant="raise" class="space-y-3 p-4">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Stack action</p>
            <UiButton
              variant="ghost"
              size="sm"
              :disabled="stackRestarting || !isAdmin"
              @click="restartStack"
            >
              <span class="inline-flex items-center gap-2">
                <NavIcon name="restart" class="h-3.5 w-3.5" />
                <UiInlineSpinner v-if="stackRestarting" />
                {{ stackRestarting ? 'Restarting stack...' : 'Restart stack' }}
              </span>
            </UiButton>
            <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
              Read-only access: admin permissions are required to restart this project stack.
            </p>
            <UiInlineFeedback v-if="stackRestartError" tone="error">
              {{ stackRestartError }}
            </UiInlineFeedback>
          </UiPanel>
        </UiPanel>
      </div>

      <div class="grid gap-5 xl:grid-cols-2">
        <UiPanel class="space-y-5 p-6">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Network</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Published ports</h2>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <UiPanel variant="soft" class="space-y-2 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Proxy</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ detail.network.proxyPort || '—' }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-2 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Database</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ detail.network.dbPort || '—' }}</p>
            </UiPanel>
          </div>
          <div v-if="detail.network.publishedPorts.length === 0" class="text-sm text-[color:var(--muted)]">
            No published container ports detected.
          </div>
          <div v-else class="space-y-2">
            <UiPanel
              v-for="binding in detail.network.publishedPorts"
              :key="`${binding.container}-${binding.hostPort}-${binding.containerPort}-${binding.proto}`"
              variant="soft"
              class="space-y-1 p-3 text-sm text-[color:var(--muted)]"
            >
              <p class="font-semibold text-[color:var(--text)]">{{ binding.container }}</p>
              <p class="font-mono text-xs text-[color:var(--muted-2)]">
                {{ binding.hostIp || '0.0.0.0' }}:{{ binding.hostPort }} -> {{ binding.containerPort }}/{{ binding.proto || 'tcp' }}
              </p>
            </UiPanel>
          </div>
        </UiPanel>

        <UiPanel class="space-y-5 p-6">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Compose</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Detected files</h2>
          </div>
          <div v-if="detail.runtime.composeFiles.length > 0" class="space-y-2">
            <UiPanel
              v-for="file in detail.runtime.composeFiles"
              :key="file"
              variant="soft"
              class="p-3 font-mono text-xs text-[color:var(--muted)] break-all"
            >
              {{ file }}
            </UiPanel>
          </div>
          <p v-else class="text-sm text-[color:var(--muted)]">No compose files discovered in the project directory.</p>
        </UiPanel>
      </div>

      <UiPanel class="space-y-5 p-6">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Runtime units ({{ detail.containers.length }})</h2>
        </div>
        <UiState v-if="detail.containers.length === 0">No containers currently match this compose project label.</UiState>
        <div v-else class="grid gap-4 xl:grid-cols-2">
          <UiListRow
            v-for="container in detail.containers"
            :key="container.id"
            as="article"
            class="space-y-4"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ container.service || 'Container' }}
                </p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">{{ container.name }}</h3>
                <p class="mt-1 font-mono text-[11px] text-[color:var(--muted-2)]">{{ container.id }}</p>
              </div>
              <UiBadge :tone="containerTone(container)">{{ container.status || 'unknown' }}</UiBadge>
            </div>
            <div class="space-y-2 text-xs text-[color:var(--muted)]">
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Image</span>
                <span class="text-[color:var(--text)] break-all">{{ container.image }}</span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Ports</span>
                <span class="text-[color:var(--text)]">{{ container.ports || '—' }}</span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Service</span>
                <span class="text-[color:var(--text)]">{{ container.service || '—' }}</span>
              </div>
            </div>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel class="space-y-5 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Archive cleanup</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Plan and execute</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Cleanup is asynchronous and always queued as a project job.
            </p>
          </div>
          <UiButton variant="ghost" size="sm" :disabled="archivePlanLoading" @click="loadArchivePlan">
            <span class="inline-flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="archivePlanLoading" />
              Refresh plan
            </span>
          </UiButton>
        </div>

        <UiState v-if="!isAdmin">
          Read-only access: admin permissions are required to preview and execute archive cleanup.
        </UiState>
        <UiState v-else-if="archivePlanLoading" loading>Building archive cleanup plan...</UiState>
        <UiState v-else-if="archivePlanError" tone="error">{{ archivePlanError }}</UiState>

        <template v-else-if="archivePlan">
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.containers.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.hostnames.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress rules</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.ingressRules.length }}</p>
            </UiPanel>
            <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS records</p>
              <p class="text-lg font-semibold text-[color:var(--text)]">
                {{ archivePlan.dnsRecords.filter((record) => record.deleteEligible).length }}/{{ archivePlan.dnsRecords.length }}
              </p>
            </UiPanel>
          </div>

          <UiInlineFeedback v-if="archivePlan.warnings.length > 0" tone="warn">
            {{ archivePlan.warnings.length }} warning(s): {{ archivePlan.warnings.join(' | ') }}
          </UiInlineFeedback>
          <UiInlineFeedback v-if="archiveExecutedWithWarnings" tone="warn">
            Last archive request was queued with warnings in the plan preview. Review job logs after completion.
          </UiInlineFeedback>
          <UiInlineFeedback v-if="archiveExecuteError" tone="error">
            {{ archiveExecuteError }}
          </UiInlineFeedback>

          <div class="grid gap-4 xl:grid-cols-2">
            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <UiState v-if="archivePlan.hostnames.length === 0">No hostnames discovered.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="hostname in archivePlan.hostnames"
                  :key="hostname"
                  class="font-mono text-[color:var(--text)] break-all"
                >
                  {{ hostname }}
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Container targets</p>
              <UiState v-if="archivePlan.containers.length === 0">No project containers found.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="container in archivePlan.containers"
                  :key="container.id || container.name"
                  class="flex flex-wrap items-center justify-between gap-2"
                >
                  <span class="font-mono text-[color:var(--text)]">{{ container.name }}</span>
                  <UiBadge :tone="statusTone(container.status)">
                    {{ container.status || 'unknown' }}
                  </UiBadge>
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress targets</p>
              <UiState v-if="archivePlan.ingressRules.length === 0">No ingress rules matched.</UiState>
              <ul v-else class="space-y-1 text-xs text-[color:var(--muted)]">
                <li
                  v-for="rule in archivePlan.ingressRules"
                  :key="`${rule.source}-${rule.hostname}-${rule.service}`"
                  class="space-y-1"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <span class="font-mono text-[color:var(--text)] break-all">{{ rule.hostname }}</span>
                    <UiBadge :tone="rule.source === 'remote' ? 'ok' : 'neutral'">
                      {{ rule.source }}
                    </UiBadge>
                  </div>
                  <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">{{ rule.service || 'service not set' }}</p>
                </li>
              </ul>
            </UiPanel>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS targets</p>
              <UiState v-if="archivePlan.dnsRecords.length === 0">No DNS records matched.</UiState>
              <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
                <li
                  v-for="record in archivePlan.dnsRecords"
                  :key="`${record.zoneId}-${record.id}-${record.name}`"
                  class="space-y-1"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <span class="font-mono text-[color:var(--text)] break-all">{{ record.name }}</span>
                    <UiBadge :tone="record.deleteEligible ? 'ok' : 'warn'">
                      {{ record.deleteEligible ? 'deletable' : 'skip' }}
                    </UiBadge>
                  </div>
                  <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                    {{ record.type }} → {{ record.content }}
                  </p>
                  <p v-if="record.skipReason" class="text-[11px] text-[color:var(--muted)]">
                    {{ record.skipReason }}
                  </p>
                </li>
              </ul>
            </UiPanel>
          </div>

          <UiPanel variant="raise" class="space-y-4 p-5">
            <div>
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Execution</p>
              <p class="mt-2 text-sm text-[color:var(--muted)]">
                Confirmation phrase: <span class="font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
              </p>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <UiToggle v-model="archiveOptions.removeContainers" :disabled="!isAdmin">
                Remove project containers
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeVolumes" :disabled="!isAdmin || !archiveOptions.removeContainers">
                Remove container volumes
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeIngress" :disabled="!isAdmin">
                Remove ingress rules
              </UiToggle>
              <UiToggle v-model="archiveOptions.removeDns" :disabled="!isAdmin">
                Remove DNS records
              </UiToggle>
            </div>

            <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Confirmation phrase
            </label>
            <UiInput
              v-model="archiveConfirmInput"
              :disabled="!isAdmin || archiveExecuting"
              autocomplete="off"
              spellcheck="false"
              placeholder="Type the phrase exactly"
            />

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                variant="danger"
                size="sm"
                :disabled="!canSubmitArchive"
                @click="queueArchive"
              >
                <span class="inline-flex items-center gap-2">
                  <UiInlineSpinner v-if="archiveExecuting" />
                  {{ archiveExecuting ? 'Queueing archive...' : 'Queue archive job' }}
                </span>
              </UiButton>
              <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
                Read-only access: admin permissions are required to queue archive cleanup.
              </p>
            </div>
          </UiPanel>
        </template>
      </UiPanel>

      <UiPanel class="space-y-5 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project jobs</p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Activity timeline</h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              {{ jobsTotal }} total jobs for {{ detail.project.normalizedName }}.
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
    </template>

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
  </section>
</template>
