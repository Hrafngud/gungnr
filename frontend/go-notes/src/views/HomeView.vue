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
import { useOnboardingStore } from '@/stores/onboarding'
import { projectsApi } from '@/services/projects'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { githubApi } from '@/services/github'
import { apiErrorMessage } from '@/services/api'
import { isPendingJob } from '@/utils/jobStatus'
import type { LocalProject } from '@/types/projects'
import type { DockerHealth, TunnelHealth } from '@/types/health'
import type { Settings } from '@/types/settings'
import type { GitHubCatalog } from '@/types/github'
import type { OnboardingStep } from '@/types/onboarding'

import { servicePresets } from '@/data/service-presets'

type QueueState = {
  loading: boolean
  error: string | null
  success: string | null
  jobId: number | null
}

type ServiceCard = {
  id: string
  name: string
  description: string
  subdomain?: string
  port?: number
  image?: string
  containerPort?: number
  repoLabel: string
  repoUrl: string
  kind: 'custom' | 'preset'
}

type TemplateCardId = 'create' | 'existing'
type TemplateCard = {
  id: TemplateCardId
  title: string
  description: string
  actionLabel: string
}

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const projectsStore = useProjectsStore()
const jobsStore = useJobsStore()
const auth = useAuthStore()
const toastStore = useToastStore()
const onboardingStore = useOnboardingStore()

const machineName = ref('')
const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const settings = ref<Settings | null>(null)
const hostLoading = ref(false)
const settingsError = ref<string | null>(null)
const onboardingOpen = ref(false)
const onboardingStep = ref(0)
const localProjects = ref<LocalProject[]>([])
const localLoading = ref(false)
const localError = ref<string | null>(null)
const catalog = ref<GitHubCatalog | null>(null)
const catalogError = ref<string | null>(null)

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
  image: '',
  containerPort: '',
})

const templateCards: TemplateCard[] = [
  {
    id: 'create',
    title: 'Create from template',
    description: 'Create a new repo from the approved template source.',
    actionLabel: 'Configure template',
  },
  {
    id: 'existing',
    title: 'Deploy existing',
    description: 'Launch a local template folder with new ingress.',
    actionLabel: 'Configure deploy',
  },
]

const customServiceCard: ServiceCard = {
  id: 'custom',
  name: 'Custom service',
  description: 'Forward any local port through the host tunnel.',
  repoLabel: 'cloudflare/cloudflared',
  repoUrl: 'https://github.com/cloudflare/cloudflared',
  kind: 'custom',
}

const serviceCards = computed<ServiceCard[]>(() => [
  customServiceCard,
  ...servicePresets.map((preset) => ({
    id: preset.id,
    name: preset.name,
    description: preset.description,
    subdomain: preset.subdomain,
    port: preset.port,
    image: preset.image,
    containerPort: preset.containerPort,
    repoLabel: preset.repoLabel,
    repoUrl: preset.repoUrl,
    kind: 'preset' as const,
  })),
])

const selectedTemplateCard = ref<TemplateCardId | null>(null)
const selectedServiceCard = ref<string | null>(null)

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'host-status',
    title: 'Check host readiness',
    description: 'Confirm Docker and the host cloudflared service are healthy before queuing deploys.',
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
    description: 'Complete Host Settings to connect the host tunnel service and DNS automation.',
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
    if (isPendingJob(job.status)) counts.pending += 1
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

const templateRepoLabel = computed(() => {
  if (catalogError.value) return 'Template source unavailable'
  if (!catalog.value?.template?.configured) return 'Template source not configured'
  const owner = catalog.value.template.owner
  const repo = catalog.value.template.repo
  if (!owner || !repo) return 'Template source not configured'
  return `${owner}/${repo}`
})

const templateRepoUrl = computed(() => {
  if (!catalog.value?.template?.configured) return ''
  const owner = catalog.value.template.owner
  const repo = catalog.value.template.repo
  if (!owner || !repo) return ''
  return `https://github.com/${owner}/${repo}`
})

const selectedService = computed(() =>
  serviceCards.value.find((card) => card.id === selectedServiceCard.value) ?? null,
)

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

const loadCatalog = async () => {
  if (!isAuthenticated.value) {
    catalog.value = null
    catalogError.value = null
    return
  }
  catalogError.value = null
  try {
    const { data } = await githubApi.catalog()
    catalog.value = data.catalog
  } catch (err) {
    catalog.value = null
    catalogError.value = apiErrorMessage(err)
  }
}

const refreshAll = async () => {
  await Promise.allSettled([
    loadHostStatus(),
    loadCatalog(),
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
    templateState.success = 'Template job queued. Automation started.'
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
    existingState.success = 'Deployment queued. Automation started.'
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
  const containerPort = parsePort(quickForm.containerPort, false)
  if (containerPort === null) {
    quickState.error = 'Container port must be numeric.'
    return
  }
  const image = quickForm.image.trim()
  quickState.loading = true
  try {
    const { data } = await projectsApi.quickService({
      subdomain: quickForm.subdomain,
      port,
      image: image ? image : undefined,
      containerPort: containerPort ?? undefined,
    })
    quickState.jobId = data.job.id
    quickState.success = 'Service forward queued. Automation started.'
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

const selectTemplateCard = async (id: TemplateCardId) => {
  if (selectedTemplateCard.value === id) {
    selectedTemplateCard.value = null
    return
  }
  selectedTemplateCard.value = id
  resetState(templateState)
  resetState(existingState)
  if (id === 'existing' && localProjects.value.length === 0 && !localLoading.value) {
    await loadLocalProjects()
  }
}

const selectServiceCard = (card: ServiceCard) => {
  if (selectedServiceCard.value === card.id) {
    selectedServiceCard.value = null
    return
  }
  selectedServiceCard.value = card.id
  resetState(quickState)
  if (card.kind === 'custom') {
    quickForm.subdomain = ''
    quickForm.port = ''
    quickForm.image = ''
    quickForm.containerPort = ''
    return
  }
  quickForm.subdomain = card.subdomain ?? ''
  quickForm.port = typeof card.port === 'number' ? card.port.toString() : ''
  quickForm.image = card.image ?? ''
  quickForm.containerPort =
    typeof card.containerPort === 'number' ? card.containerPort.toString() : ''
}

const startOnboarding = () => {
  onboardingStep.value = 0
  onboardingOpen.value = true
}

const markOnboardingComplete = () => {
  onboardingStore.updateState({ home: true })
}

onMounted(async () => {
  if (!projectsStore.initialized) {
    projectsStore.fetchProjects()
  }
  if (!jobsStore.initialized) {
    jobsStore.fetchJobs()
  }
  loadHostStatus()
  if (typeof window !== 'undefined') {
    machineName.value = window.location.hostname || 'localhost'
  }
  await onboardingStore.fetchState()
  if (!onboardingStore.state.home) {
    onboardingOpen.value = true
  }
})

watch(
  () => auth.user,
  (value) => {
    if (value) {
      if (localProjects.value.length === 0 && !localLoading.value) {
        loadLocalProjects()
      }
      loadCatalog()
    } else {
      catalog.value = null
      catalogError.value = null
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
            Queue new stacks or forward local services through the host cloudflared service. Jobs start automatically after queueing.
          </p>
        </div>
      </div>
      <UiPanel
        v-if="!isAuthenticated"
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-4 p-4 text-sm text-[color:var(--muted)]"
      >
        <span>Sign in to queue deploy jobs and host tunnel actions.</span>
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
          <div class="grid gap-4 sm:grid-cols-2">
            <UiPanel
              v-for="card in templateCards"
              :key="card.id"
              :variant="selectedTemplateCard === card.id ? 'raise' : 'soft'"
              class="flex h-full flex-col gap-4 p-4 text-left transition"
              :class="selectedTemplateCard === card.id ? 'border-[color:var(--accent)]' : ''"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    Template
                  </p>
                  <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                    {{ card.title }}
                  </h4>
                  <p class="mt-2 text-xs text-[color:var(--muted)]">
                    {{ card.description }}
                  </p>
                </div>
                <UiBadge :tone="selectedTemplateCard === card.id ? 'ok' : 'neutral'">
                  {{ selectedTemplateCard === card.id ? 'Selected' : 'Ready' }}
                </UiBadge>
              </div>
              <div class="flex items-center gap-2 text-xs text-[color:var(--muted)]">
                <svg
                  class="h-3.5 w-3.5 text-[color:var(--muted-2)]"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M10 13a5 5 0 0 1 0-7l2-2a5 5 0 0 1 7 7l-2 2" />
                  <path d="M14 11a5 5 0 0 1 0 7l-2 2a5 5 0 0 1-7-7l2-2" />
                </svg>
                <a
                  v-if="templateRepoUrl"
                  :href="templateRepoUrl"
                  target="_blank"
                  rel="noreferrer"
                  class="text-[color:var(--text)] transition hover:text-[color:var(--accent-ink)]"
                >
                  {{ templateRepoLabel }}
                </a>
                <span v-else>{{ templateRepoLabel }}</span>
              </div>
              <UiButton
                type="button"
                variant="ghost"
                size="sm"
                @click="selectTemplateCard(card.id)"
              >
                {{ selectedTemplateCard === card.id ? 'Hide form' : card.actionLabel }}
              </UiButton>
            </UiPanel>
          </div>
          <UiPanel
            v-if="selectedTemplateCard === 'create'"
            as="form"
            variant="soft"
            class="space-y-4 p-4"
            @submit.prevent="submitTemplate"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Create from template
                </p>
                <p class="mt-1 text-xs text-[color:var(--muted)]">
                  Template source: {{ templateRepoLabel }}
                </p>
              </div>
              <UiButton type="button" variant="ghost" size="xs" @click="selectedTemplateCard = null">
                Back to cards
              </UiButton>
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
          <UiPanel
            v-if="selectedTemplateCard === 'existing'"
            as="form"
            variant="soft"
            class="space-y-4 p-4"
            @submit.prevent="submitExisting"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Deploy existing
                </p>
                <p class="mt-1 text-xs text-[color:var(--muted)]">
                  Use a local template folder and assign a new subdomain.
                </p>
              </div>
              <div class="flex items-center gap-2">
                <UiButton type="button" variant="ghost" size="xs" @click="selectedTemplateCard = null">
                  Back to cards
                </UiButton>
                <UiButton type="button" variant="ghost" size="xs" @click="loadLocalProjects">
                  Refresh list
                </UiButton>
              </div>
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
              Expose a running port through the host tunnel service instantly.
            </p>
          </div>
          <div class="grid gap-4 sm:grid-cols-2 overflow-y-auto" style="max-height: calc(2 * (theme('spacing.40') + theme('spacing.4')))">
            <UiPanel
              v-for="card in serviceCards"
              :key="card.id"
              :variant="selectedServiceCard === card.id ? 'raise' : 'soft'"
              class="flex h-full flex-col gap-4 p-4 text-left transition"
              :class="selectedServiceCard === card.id ? 'border-[color:var(--accent)]' : ''"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    Service
                  </p>
                  <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                    {{ card.name }}
                  </h4>
                  <p class="mt-2 text-xs text-[color:var(--muted)]">
                    {{ card.description }}
                  </p>
                </div>
                <UiBadge :tone="selectedServiceCard === card.id ? 'ok' : 'neutral'">
                  {{ selectedServiceCard === card.id ? 'Selected' : card.kind === 'custom' ? 'Custom' : 'Preset' }}
                </UiBadge>
              </div>
              <div class="flex flex-wrap items-center gap-2 text-xs text-[color:var(--muted)]">
                <span v-if="card.subdomain">subdomain: {{ card.subdomain }}</span>
                <span v-if="card.port">port: {{ card.port }}</span>
              </div>
              <div class="flex items-center gap-2 text-xs text-[color:var(--muted)]">
                <svg
                  class="h-3.5 w-3.5 text-[color:var(--muted-2)]"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M10 13a5 5 0 0 1 0-7l2-2a5 5 0 0 1 7 7l-2 2" />
                  <path d="M14 11a5 5 0 0 1 0 7l-2 2a5 5 0 0 1-7-7l2-2" />
                </svg>
                <a
                  :href="card.repoUrl"
                  target="_blank"
                  rel="noreferrer"
                  class="text-[color:var(--text)] transition hover:text-[color:var(--accent-ink)]"
                >
                  {{ card.repoLabel }}
                </a>
              </div>
              <UiButton
                type="button"
                variant="ghost"
                size="sm"
                @click="selectServiceCard(card)"
              >
                {{ selectedServiceCard === card.id ? 'Hide form' : 'Select service' }}
              </UiButton>
            </UiPanel>
          </div>
          <UiPanel
            v-if="selectedService"
            as="form"
            variant="soft"
            class="space-y-4 p-4"
            @submit.prevent="submitQuick"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Forward {{ selectedService.name }}
                </p>
                <p class="mt-1 text-xs text-[color:var(--muted)]">
                  {{ selectedService.description }}
                </p>
              </div>
              <UiButton type="button" variant="ghost" size="xs" @click="selectedServiceCard = null">
                Back to cards
              </UiButton>
            </div>
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
              <p class="text-xs text-[color:var(--muted)]">
                Host port exposed by Docker on this machine.
              </p>
            </label>
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Container image (optional)
              </span>
              <UiInput
                v-model="quickForm.image"
                type="text"
                placeholder="excalidraw/excalidraw:latest"
                :disabled="quickState.loading"
              />
              <p class="text-xs text-[color:var(--muted)]">
                Leave blank to use the default image.
              </p>
            </label>
            <label class="grid gap-2 text-sm">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Container port (optional)
              </span>
              <UiInput
                v-model="quickForm.containerPort"
                type="text"
                placeholder="80"
                :disabled="quickState.loading"
              />
              <p class="text-xs text-[color:var(--muted)]">
                Port inside the container (default 80). Host port maps to this.
              </p>
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