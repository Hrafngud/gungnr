<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import HostStatusPanel from '@/components/home/HostStatusPanel.vue'
import TemplateCardsSection from '@/components/home/TemplateCardsSection.vue'
import ServiceCardsSection from '@/components/home/ServiceCardsSection.vue'
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

type QueueState = {
  loading: boolean
  error: string | null
  success: string | null
  jobId: number | null
}

type TemplateCardId = 'create' | 'existing'

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
const templateFormOpen = ref(false)
const existingFormOpen = ref(false)
const quickFormOpen = ref(false)

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

const lastJob = computed(() => jobsStore.jobs[0] ?? null)

const lastProject = computed(() => {
  if (projectsStore.projects.length === 0) return null
  const sorted = [...projectsStore.projects].sort((a, b) => {
    const aTime = new Date(a.updatedAt || a.createdAt).getTime()
    const bTime = new Date(b.updatedAt || b.createdAt).getTime()
    return bTime - aTime
  })
  return sorted[0] ?? null
})

const templateRepoLabel = computed(() => {
  if (catalogError.value) return 'Template source unavailable'
  if (!catalog.value?.template?.configured) return 'Template source not configured'
  const owner = catalog.value.template.owner
  const repo = catalog.value.template.repo
  if (!owner || !repo) return 'Template source not configured'
  return `${owner}/${repo}`
})

const selectedServiceName = ref<string>('')

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
    if (id === 'create') {
      templateFormOpen.value = false
    } else if (id === 'existing') {
      existingFormOpen.value = false
    }
    return
  }
  selectedTemplateCard.value = id
  resetState(templateState)
  resetState(existingState)
  if (id === 'create') {
    templateFormOpen.value = true
  } else if (id === 'existing') {
    existingFormOpen.value = true
    if (localProjects.value.length === 0 && !localLoading.value) {
      await loadLocalProjects()
    }
  }
}

const selectServiceCard = (card: ServiceCard) => {
  if (selectedServiceCard.value === card.id) {
    selectedServiceCard.value = null
    selectedServiceName.value = ''
    quickFormOpen.value = false
    return
  }
  selectedServiceCard.value = card.id
  selectedServiceName.value = card.name
  resetState(quickState)
  quickFormOpen.value = true
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
  <HostStatusPanel
    :machine-name="machineName"
    :docker-health="dockerHealth"
    :tunnel-health="tunnelHealth"
    :settings="settings"
    :host-loading="hostLoading"
    :settings-error="settingsError"
    :jobs-error="jobsStore.error"
    :projects-error="projectsStore.error"
    :job-counts="jobCounts"
    :last-job="lastJob"
    :last-project="lastProject"
    @refresh="refreshAll"
    @start-onboarding="startOnboarding"
  />
  <hr />
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
        <TemplateCardsSection
          :catalog="catalog"
          :catalog-error="catalogError"
          :selected-card="selectedTemplateCard"
          @select-card="selectTemplateCard"
        />
        <ServiceCardsSection
          :selected-card-id="selectedServiceCard"
          @select-card="selectServiceCard"
        />
      </div>
    </section>
  <UiOnboardingOverlay
    v-model="onboardingOpen"
    v-model:stepIndex="onboardingStep"
    :steps="onboardingSteps"
    @finish="markOnboardingComplete"
    @skip="markOnboardingComplete"
  />
  <UiFormSidePanel
    v-model="templateFormOpen"
    title="Create from template"
  >
    <form class="space-y-5" @submit.prevent="submitTemplate">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Create a new GitHub repo from your template and deploy it locally with automatic port configuration.
        </p>
        <p class="text-xs text-[color:var(--muted)]">
          Template source: {{ templateRepoLabel }}
        </p>
      </div>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Project name <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="templateForm.name"
          type="text"
          placeholder="my-project"
          required
          :disabled="templateState.loading"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Name for the GitHub repo and local folder.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Subdomain <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="templateForm.subdomain"
          type="text"
          placeholder="my-project"
          required
          :disabled="templateState.loading"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Subdomain for web access via your Cloudflare tunnel.
        </p>
      </label>
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
    </form>
  </UiFormSidePanel>
  <UiFormSidePanel
    v-model="existingFormOpen"
    title="Forward localhost service"
  >
    <form class="space-y-5" @submit.prevent="submitExisting">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Forward any running localhost service (Docker or not) through your Cloudflare tunnel for web access.
        </p>
      </div>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Service name <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="existingForm.name"
          type="text"
          placeholder="my-service"
          required
          :disabled="existingState.loading"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Internal identifier for tracking this forwarded service.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Subdomain <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="existingForm.subdomain"
          type="text"
          placeholder="my-service"
          required
          :disabled="existingState.loading"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Subdomain for web access via your Cloudflare tunnel.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Running at (localhost port) <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          v-model="existingForm.port"
          type="text"
          placeholder="3000"
          required
          :disabled="existingState.loading"
        />
        <p class="text-xs text-[color:var(--muted)]">
          The localhost port where your service is currently running.
        </p>
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
          {{ existingState.loading ? 'Queueing...' : 'Queue forward job' }}
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
    </form>
  </UiFormSidePanel>
  <UiFormSidePanel
    v-model="quickFormOpen"
    :title="selectedServiceName ? `Forward ${selectedServiceName}` : 'Quick service'"
  >
    <form class="space-y-5" @submit.prevent="submitQuick">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Expose a running port through the host tunnel service instantly.
        </p>
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
    </form>
  </UiFormSidePanel>
</template>
