<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useProjectsStore } from '@/stores/projects'
import { useJobsStore } from '@/stores/jobs'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toasts'
import { projectsApi } from '@/services/projects'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import type { LocalProject } from '@/types/projects'
import type { DockerHealth, TunnelHealth } from '@/types/health'
import type { Settings } from '@/types/settings'
import type { OnboardingStep } from '@/types/onboarding'

type QueueState = {
  loading: boolean
  error: string | null
  success: string | null
  jobId: number | null
}

type ServicePreset = {
  name: string
  subdomain: string
  port: number
  description: string
}

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const projectsStore = useProjectsStore()
const jobsStore = useJobsStore()
const auth = useAuthStore()
const toastStore = useToastStore()

const machineName = ref('')
const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const settings = ref<Settings | null>(null)
const hostLoading = ref(false)
const settingsError = ref<string | null>(null)
const onboardingKey = 'warp-panel-onboarding-home'
const onboardingOpen = ref(false)
const onboardingStep = ref(0)

const localProjects = ref<LocalProject[]>([])
const localLoading = ref(false)
const localError = ref<string | null>(null)

const templateState = reactive<QueueState>({
  loading: false,
  error: null,
  success: null,
  jobId: null,
})

const existingState = reactive<QueueState>({
  loading: false,
  error: null,
  success: null,
  jobId: null,
})

const quickState = reactive<QueueState>({
  loading: false,
  error: null,
  success: null,
  jobId: null,
})

const templateForm = reactive({
  name: '',
  subdomain: '',
  proxyPort: '',
  dbPort: '',
})

const existingForm = reactive({
  name: '',
  subdomain: '',
  port: '80',
})

const quickForm = reactive({
  subdomain: '',
  port: '',
})

const servicePresets: ServicePreset[] = [
  {
    name: 'Excalidraw',
    subdomain: 'draw',
    port: 5000,
    description: 'Whiteboard collaboration',
  },
  {
    name: 'OpenWebUI',
    subdomain: 'openwebui',
    port: 3000,
    description: 'LLM web interface',
  },
  {
    name: 'Ollama',
    subdomain: 'ollama',
    port: 11434,
    description: 'Local model runtime',
  },
  {
    name: 'Redis',
    subdomain: 'redis',
    port: 6379,
    description: 'Cache + queue store',
  },
  {
    name: 'Postgres',
    subdomain: 'postgres',
    port: 5432,
    description: 'Database service',
  },
]

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'host-status',
    title: 'Check host readiness',
    description: 'Confirm Docker and the tunnel are healthy before queuing deploys.',
    target: "[data-onboard='home-status']",
  },
  {
    id: 'quick-deploy',
    title: 'Queue deploys quickly',
    description: 'Launch a template repo or forward a local port. Jobs and Activity update as soon as each run starts.',
    target: "[data-onboard='home-quick-deploy']",
  },
  {
    id: 'finish-setup',
    title: 'Finish host setup',
    description: 'Complete Host Settings to unlock DNS automation and full tunnel control.',
    target: "[data-onboard='home-onboarding']",
  },
]

const isAuthenticated = computed(() => Boolean(auth.user))

const jobCounts = computed(() => {
  const counts = {
    pending: 0,
    running: 0,
    completed: 0,
    failed: 0,
  }
  jobsStore.jobs.forEach((job) => {
    if (job.status === 'pending') counts.pending += 1
    else if (job.status === 'running') counts.running += 1
    else if (job.status === 'completed') counts.completed += 1
    else if (job.status === 'failed') counts.failed += 1
  })
  return counts
})

const jobTone = computed<BadgeTone>(() => {
  if (jobCounts.value.failed > 0) return 'error'
  if (jobCounts.value.running > 0) return 'warn'
  if (jobCounts.value.pending > 0) return 'neutral'
  if (jobCounts.value.completed > 0) return 'ok'
  return 'neutral'
})

const lastJob = computed(() => jobsStore.jobs[0] ?? null)

const lastProject = computed(() => {
  if (projectsStore.projects.length === 0) return null
  return [...projectsStore.projects].sort((a, b) => {
    const aTime = new Date(a.updatedAt || a.createdAt).getTime()
    const bTime = new Date(b.updatedAt || b.createdAt).getTime()
    return bTime - aTime
  })[0]
})

const lastServiceLabel = computed(() => lastProject.value?.name ?? 'n/a')
const lastServiceTime = computed(() => {
  if (!lastProject.value) return 'No deployments yet.'
  const stamp = lastProject.value.updatedAt || lastProject.value.createdAt
  return `Updated ${formatDate(stamp)}`
})

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

const resetState = (state: QueueState) => {
  state.error = null
  state.success = null
  state.jobId = null
}

const parsePort = (value: string, required: boolean) => {
  const trimmed = value.trim()
  if (!trimmed) return required ? null : undefined
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed)) return null
  return parsed
}

const formatDate = (value?: string | null) => {
  if (!value) return 'n/a'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'n/a'
  return date.toLocaleString()
}

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

const loadLocalProjects = async () => {
  localLoading.value = true
  localError.value = null
  try {
    const { data } = await projectsApi.listLocal()
    localProjects.value = data.projects
  } catch (err) {
    localError.value = apiErrorMessage(err)
  } finally {
    localLoading.value = false
  }
}

const refreshAll = async () => {
  await Promise.allSettled([
    loadHostStatus(),
    jobsStore.fetchJobs(),
    projectsStore.fetchProjects(),
  ])
}

const submitTemplate = async () => {
  if (templateState.loading || !isAuthenticated.value) return
  resetState(templateState)

  if (!templateForm.name.trim()) {
    templateState.error = 'Project name is required.'
    return
  }
  const proxyPort = parsePort(templateForm.proxyPort, false)
  const dbPort = parsePort(templateForm.dbPort, false)
  if (proxyPort === null || dbPort === null) {
    templateState.error = 'Ports must be numeric.'
    return
  }

  templateState.loading = true
  try {
    const { data } = await projectsApi.createFromTemplate({
      name: templateForm.name,
      subdomain: templateForm.subdomain || undefined,
      proxyPort,
      dbPort,
    })
    templateState.jobId = data.job.id
    templateState.success = 'Template job queued.'
    toastStore.success('Template job queued.', 'Template queued')
    await refreshAll()
  } catch (err) {
    const message = apiErrorMessage(err)
    templateState.error = message
    toastStore.error(message, 'Template job failed')
  } finally {
    templateState.loading = false
  }
}

const submitExisting = async () => {
  if (existingState.loading || !isAuthenticated.value) return
  resetState(existingState)

  if (!existingForm.name.trim() || !existingForm.subdomain.trim()) {
    existingState.error = 'Project name and subdomain are required.'
    return
  }
  const port = parsePort(existingForm.port, false)
  if (port === null) {
    existingState.error = 'Port must be numeric.'
    return
  }

  existingState.loading = true
  try {
    const { data } = await projectsApi.deployExisting({
      name: existingForm.name,
      subdomain: existingForm.subdomain,
      port,
    })
    existingState.jobId = data.job.id
    existingState.success = 'Deployment queued.'
    toastStore.success('Deployment queued.', 'Deploy job queued')
    await refreshAll()
  } catch (err) {
    const message = apiErrorMessage(err)
    existingState.error = message
    toastStore.error(message, 'Deploy job failed')
  } finally {
    existingState.loading = false
  }
}

const submitQuick = async () => {
  if (quickState.loading || !isAuthenticated.value) return
  resetState(quickState)

  if (!quickForm.subdomain.trim()) {
    quickState.error = 'Subdomain is required.'
    return
  }
  const port = parsePort(quickForm.port, true)
  if (port === null || port === undefined) {
    quickState.error = 'Port must be numeric.'
    return
  }

  quickState.loading = true
  try {
    const { data } = await projectsApi.quickService({
      subdomain: quickForm.subdomain,
      port,
    })
    quickState.jobId = data.job.id
    quickState.success = 'Service forward queued.'
    toastStore.success('Service forward queued.', 'Forward queued')
    await refreshAll()
  } catch (err) {
    const message = apiErrorMessage(err)
    quickState.error = message
    toastStore.error(message, 'Forward failed')
  } finally {
    quickState.loading = false
  }
}

const applyPreset = (preset: ServicePreset) => {
  quickForm.subdomain = preset.subdomain
  quickForm.port = preset.port.toString()
}

const startOnboarding = () => {
  onboardingStep.value = 0
  onboardingOpen.value = true
}

const markOnboardingComplete = () => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(onboardingKey, 'done')
  }
}

onMounted(() => {
  if (!projectsStore.initialized) {
    projectsStore.fetchProjects()
  }
  if (!jobsStore.initialized) {
    jobsStore.fetchJobs()
  }
  loadHostStatus()
  if (typeof window !== 'undefined') {
    machineName.value = window.location.hostname || 'localhost'
    const seen = window.localStorage.getItem(onboardingKey)
    if (seen !== 'done') {
      onboardingOpen.value = true
    }
  }
})

watch(
  () => auth.user,
  (value) => {
    if (value && localProjects.value.length === 0 && !localLoading.value) {
      loadLocalProjects()
    }
  },
  { immediate: true },
)
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
        <UiButton variant="ghost" @click="refreshAll">
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

      <UiState v-if="jobsStore.error" tone="error">
        {{ jobsStore.error }}
      </UiState>

      <UiState v-if="projectsStore.error" tone="error">
        {{ projectsStore.error }}
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
              {{ jobsStore.jobs.length }} total
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
            Used for new subdomains and tunnel ingress.
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
            Finish host setup to unlock tunnel automation and DNS updates.
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton :as="RouterLink" to="/host-settings" variant="primary" size="sm">
            Configure host
          </UiButton>
          <UiButton variant="ghost" size="sm" @click="startOnboarding">
            View guide
          </UiButton>
        </div>
      </UiListRow>
    </UiPanel>

    <section class="space-y-6" data-onboard="home-quick-deploy">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Quick deploy
          </p>
          <h2 class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
            Launch templates and services
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Queue new stacks or forward local services through the active tunnel.
          </p>
        </div>
      </div>

      <UiPanel
        v-if="!isAuthenticated"
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-4 p-4 text-sm text-[color:var(--muted)]"
      >
        <span>Sign in to queue deploy jobs and tunnel actions.</span>
        <UiButton :as="RouterLink" to="/login" variant="primary">
          Sign in
        </UiButton>
      </UiPanel>

      <UiPanel
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Day-to-day flow
          </p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            Queue a template or service forward, then confirm progress in Jobs and Activity.
          </p>
        </div>
        <UiButton :as="RouterLink" to="/overview" variant="ghost" size="sm">
          Open overview
        </UiButton>
      </UiPanel>

      <div class="grid gap-6 lg:grid-cols-2">
        <UiPanel as="article" variant="raise" class="space-y-6 p-6">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Templates
              </p>
              <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
                GitHub-backed stacks
              </h3>
              <p class="mt-2 text-sm text-[color:var(--muted)]">
                Create new repos from templates or deploy existing folders.
              </p>
            </div>
            <UiButton :as="RouterLink" to="/github" variant="ghost" size="sm">
              GitHub settings
            </UiButton>
          </div>

          <UiPanel as="form" variant="soft" class="space-y-4 p-4" @submit.prevent="submitTemplate">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Create from template
              </p>
            </div>
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Project name
              </span>
              <UiInput
                v-model="templateForm.name"
                type="text"
                placeholder="warp-ops"
                :disabled="templateState.loading"
              />
            </label>
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Subdomain (optional)
              </span>
              <UiInput
                v-model="templateForm.subdomain"
                type="text"
                placeholder="warp-ops"
                :disabled="templateState.loading"
              />
            </label>
            <div class="grid gap-3 sm:grid-cols-2">
              <label class="grid gap-2 text-sm">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Proxy port
                </span>
                <UiInput
                  v-model="templateForm.proxyPort"
                  type="text"
                  placeholder="80"
                  :disabled="templateState.loading"
                />
              </label>
              <label class="grid gap-2 text-sm">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Database port
                </span>
                <UiInput
                  v-model="templateForm.dbPort"
                  type="text"
                  placeholder="5432"
                  :disabled="templateState.loading"
                />
              </label>
            </div>

            <UiInlineFeedback v-if="templateState.error" tone="error">
              {{ templateState.error }}
            </UiInlineFeedback>
            <UiInlineFeedback v-if="templateState.success" tone="ok">
              {{ templateState.success }}
            </UiInlineFeedback>

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                type="submit"
                variant="primary"
                :disabled="templateState.loading || !isAuthenticated"
              >
                {{ templateState.loading ? 'Queueing...' : 'Queue template job' }}
              </UiButton>
              <UiButton
                v-if="templateState.jobId"
                :as="RouterLink"
                :to="`/jobs/${templateState.jobId}`"
                variant="ghost"
              >
                View job log
              </UiButton>
            </div>
          </UiPanel>

          <UiPanel as="form" variant="soft" class="space-y-4 p-4" @submit.prevent="submitExisting">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Deploy existing
              </p>
              <UiButton
                type="button"
                variant="ghost"
                size="xs"
                @click="loadLocalProjects"
              >
                Refresh list
              </UiButton>
            </div>

            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Project folder
              </span>
              <UiInput
                v-model="existingForm.name"
                list="local-projects"
                type="text"
                placeholder="template-folder"
                :disabled="existingState.loading"
              />
              <datalist id="local-projects">
                <option v-for="project in localProjects" :key="project.name" :value="project.name" />
              </datalist>
              <p v-if="localLoading" class="text-xs text-[color:var(--muted)]">
                Loading local templates...
              </p>
              <p v-else-if="localError" class="text-xs text-[color:var(--danger)]">
                {{ localError }}
              </p>
            </label>

            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Subdomain
              </span>
              <UiInput
                v-model="existingForm.subdomain"
                type="text"
                placeholder="warp-ops"
                :disabled="existingState.loading"
              />
            </label>

            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Host port
              </span>
              <UiInput
                v-model="existingForm.port"
                type="text"
                placeholder="80"
                :disabled="existingState.loading"
              />
            </label>

            <UiInlineFeedback v-if="existingState.error" tone="error">
              {{ existingState.error }}
            </UiInlineFeedback>
            <UiInlineFeedback v-if="existingState.success" tone="ok">
              {{ existingState.success }}
            </UiInlineFeedback>

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                type="submit"
                variant="primary"
                :disabled="existingState.loading || !isAuthenticated"
              >
                {{ existingState.loading ? 'Queueing...' : 'Queue deploy job' }}
              </UiButton>
              <UiButton
                v-if="existingState.jobId"
                :as="RouterLink"
                :to="`/jobs/${existingState.jobId}`"
                variant="ghost"
              >
                View job log
              </UiButton>
            </div>
          </UiPanel>
        </UiPanel>

        <UiPanel as="article" variant="raise" class="space-y-6 p-6">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Services
            </p>
            <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Quick tunnel forwards
            </h3>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Expose a running port through the Cloudflare tunnel instantly.
            </p>
          </div>

          <UiPanel as="form" variant="soft" class="space-y-4 p-4" @submit.prevent="submitQuick">
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Subdomain
              </span>
              <UiInput
                v-model="quickForm.subdomain"
                type="text"
                placeholder="preview"
                :disabled="quickState.loading"
              />
            </label>
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Local port
              </span>
              <UiInput
                v-model="quickForm.port"
                type="text"
                placeholder="5173"
                :disabled="quickState.loading"
              />
            </label>

            <UiInlineFeedback v-if="quickState.error" tone="error">
              {{ quickState.error }}
            </UiInlineFeedback>
            <UiInlineFeedback v-if="quickState.success" tone="ok">
              {{ quickState.success }}
            </UiInlineFeedback>

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                type="submit"
                variant="primary"
                :disabled="quickState.loading || !isAuthenticated"
              >
                {{ quickState.loading ? 'Queueing...' : 'Forward service' }}
              </UiButton>
              <UiButton
                v-if="quickState.jobId"
                :as="RouterLink"
                :to="`/jobs/${quickState.jobId}`"
                variant="ghost"
              >
                View job log
              </UiButton>
            </div>
          </UiPanel>

          <UiPanel variant="soft" class="space-y-3 p-4">
            <div class="flex items-center justify-between gap-2">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Presets
              </p>
              <UiBadge tone="neutral">Ports</UiBadge>
            </div>
            <div class="flex flex-wrap gap-2">
              <UiButton
                v-for="preset in servicePresets"
                :key="preset.name"
                type="button"
                variant="chip"
                size="chip"
                class="text-[color:var(--text)]"
                @click="applyPreset(preset)"
              >
                {{ preset.name }} - {{ preset.port }}
              </UiButton>
            </div>
            <p class="text-xs text-[color:var(--muted)]">
              Click a preset to prefill the subdomain and port.
            </p>
          </UiPanel>
        </UiPanel>
      </div>
    </section>
  </section>

  <UiOnboardingOverlay
    v-model="onboardingOpen"
    v-model:stepIndex="onboardingStep"
    :steps="onboardingSteps"
    @finish="markOnboardingComplete"
    @skip="markOnboardingComplete"
  />
</template>
