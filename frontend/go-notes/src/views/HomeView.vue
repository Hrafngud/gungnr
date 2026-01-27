<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiFieldGuidance from '@/components/ui/UiFieldGuidance.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import NavIcon from '@/components/NavIcon.vue'
import HostStatusPanel from '@/components/home/HostStatusPanel.vue'
import TemplateCardsSection from '@/components/home/TemplateCardsSection.vue'
import ServiceCardsSection from '@/components/home/ServiceCardsSection.vue'
import QuickDeployTemplateForm from '@/components/home/QuickDeployTemplateForm.vue'
import QuickDeployExistingForm from '@/components/home/QuickDeployExistingForm.vue'
import QuickDeployQuickForm from '@/components/home/QuickDeployQuickForm.vue'
import { useProjectsStore } from '@/stores/projects'
import { useJobsStore } from '@/stores/jobs'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toasts'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { projectsApi } from '@/services/projects'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { githubApi } from '@/services/github'
import { cloudflareApi } from '@/services/cloudflare'
import { apiErrorMessage } from '@/services/api'
import { isPendingJob } from '@/utils/jobStatus'
import type { DockerHealth, TunnelHealth } from '@/types/health'
import type { Settings } from '@/types/settings'
import type { GitHubCatalog } from '@/types/github'
import type { CloudflareZone } from '@/types/cloudflare'
import { useFieldGuidance } from '@/composables/useFieldGuidance'

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
const fieldGuidance = useFieldGuidance()
const pageLoading = usePageLoadingStore()

const machineName = ref('')
const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const settings = ref<Settings | null>(null)
const hostLoading = ref(false)
const settingsError = ref<string | null>(null)
const catalog = ref<GitHubCatalog | null>(null)
const catalogError = ref<string | null>(null)
const zones = ref<CloudflareZone[]>([])
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
  templateRef: '',
  name: '',
  subdomain: '',
  domain: '',
})

const existingForm = reactive({
  name: '',
  subdomain: '',
  domain: '',
  port: '80',
})

const quickForm = reactive({
  subdomain: '',
  domain: '',
  port: '',
  image: '',
  containerPort: '',
})

const selectedTemplateCard = ref<TemplateCardId | null>(null)
const selectedServiceCard = ref<string | null>(null)

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

const templateOptions = computed(() => {
  const templates = catalog.value?.templates?.filter(
    (template) => template.owner && template.repo,
  )
  if (templates && templates.length > 0) {
    return templates.map((template) => ({
      value: `${template.owner}/${template.repo}`,
      label: template.private
        ? `${template.owner}/${template.repo} (private)`
        : `${template.owner}/${template.repo}`,
    }))
  }
  if (catalog.value?.template?.configured) {
    const { owner, repo, private: isPrivate } = catalog.value.template
    if (owner && repo) {
      return [
        {
          value: `${owner}/${repo}`,
          label: isPrivate ? `${owner}/${repo} (private)` : `${owner}/${repo}`,
        },
      ]
    }
  }
  return []
})

const domainOptions = computed(() => {
  const base = settings.value?.baseDomain?.trim().toLowerCase()
  const zonesList = zones.value
    .map((zone) => zone.name?.trim().toLowerCase())
    .filter(Boolean) as string[]
  const seen = new Set<string>()
  const options: { value: string; label: string }[] = []
  if (base) {
    seen.add(base)
    options.push({ value: base, label: `${base} (base)` })
  }
  zonesList.forEach((domain) => {
    if (!seen.has(domain)) {
      seen.add(domain)
      options.push({ value: domain, label: domain })
    }
  })
  return options
})

const templateRepoLabel = computed(() => {
  if (catalogError.value) return 'Template source unavailable'
  if (templateOptions.value.length === 0) {
    return isAuthenticated.value ? 'Template source unavailable' : 'Sign in to load templates'
  }
  const selected = templateOptions.value.find(
    (option) => option.value === templateForm.templateRef,
  )
  return selected?.label ?? templateOptions.value[0]?.label ?? 'Template source unavailable'
})

const templateEmptyStateMessage = computed(() => {
  if (catalogError.value) return 'Template catalog failed to load.'
  if (!isAuthenticated.value) return 'Sign in to load template repositories.'
  return 'No template repositories available yet.'
})

const templateCreateBlocked = computed(
  () => Boolean(catalog.value) && !catalog.value?.app?.configured,
)

const templateSelectionDisabled = computed(
  () =>
    templateState.loading ||
    !isAuthenticated.value ||
    templateOptions.value.length === 0 ||
    templateCreateBlocked.value,
)

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

const loadZones = async () => {
  if (!isAuthenticated.value) {
    zones.value = []
    return
  }
  try {
    const { data } = await cloudflareApi.zones()
    zones.value = data.zones ?? []
  } catch (err) {
    zones.value = []
  }
}

const refreshAll = async () => {
  await Promise.allSettled([
    loadHostStatus(),
    loadZones(),
    loadCatalog(),
    jobsStore.fetchJobs(),
    projectsStore.fetchProjects(),
  ])
}

const submitTemplate = async () => {
  if (templateState.loading || !isAuthenticated.value) return
  resetState(templateState)
  if (templateCreateBlocked.value) {
    templateState.error = 'GitHub App credentials are required to create templates.'
    return
  }
  if (templateOptions.value.length === 0) {
    templateState.error = isAuthenticated.value
      ? 'Template source is unavailable.'
      : 'Sign in to load template sources.'
    return
  }
  if (!templateForm.templateRef.trim()) {
    templateState.error = 'Select a template source.'
    return
  }
  if (!templateForm.name.trim()) {
    templateState.error = 'Project name is required.'
    return
  }
  if (!templateForm.domain.trim()) {
    templateState.error = 'Select a domain.'
    return
  }
  templateState.loading = true
  try {
    const { data } = await projectsApi.createFromTemplate({
      template: templateForm.templateRef,
      name: templateForm.name,
      subdomain: templateForm.subdomain || undefined,
      domain: templateForm.domain,
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
    existingState.error = 'Service name and subdomain are required.'
    return
  }
  if (!existingForm.domain.trim()) {
    existingState.error = 'Select a domain.'
    return
  }
  const port = parsePort(existingForm.port, true)
  if (port === null || port === undefined) {
    existingState.error = 'Port must be numeric.'
    return
  }
  existingState.loading = true
  try {
    const { data } = await projectsApi.forwardLocal({
      name: existingForm.name,
      subdomain: existingForm.subdomain,
      domain: existingForm.domain,
      port,
    })
    existingState.jobId = data.job.id
    existingState.success = 'Forward queued. Automation started.'
    toastStore.success('Forward queued.', 'Forward job queued')
    await refreshAll()
  } catch (err) {
    const message = apiErrorMessage(err)
    existingState.error = message
    toastStore.error(message, 'Forward failed')
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
  if (!quickForm.domain.trim()) {
    quickState.error = 'Select a domain.'
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
      domain: quickForm.domain,
      port,
      image: image ? image : undefined,
      containerPort: containerPort ?? undefined,
    })
    quickState.jobId = data.job.id
    const hostPort = data.hostPort ?? port
    const portNote =
      hostPort === port
        ? `on port ${hostPort}`
        : `requested port ${port} was busy; using port ${hostPort}`
    quickState.success = `Service forward queued ${portNote}. Automation started.`
    toastStore.success(`Service forward queued ${portNote}.`, 'Forward queued')
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
  if (id === 'create' && templateCreateBlocked.value) return
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

onMounted(async () => {
  pageLoading.start('Loading host snapshot...')
  await Promise.allSettled([
    !projectsStore.initialized ? projectsStore.fetchProjects() : Promise.resolve(),
    !jobsStore.initialized ? jobsStore.fetchJobs() : Promise.resolve(),
    loadHostStatus(),
    loadZones(),
  ])
  pageLoading.stop()
  if (typeof window !== 'undefined') {
    machineName.value = window.location.hostname || 'localhost'
  }
})

watch(
  () => domainOptions.value,
  (options) => {
    const fallback = options[0]?.value ?? ''
    if (!options.find((option) => option.value === templateForm.domain)) {
      templateForm.domain = fallback
    }
    if (!options.find((option) => option.value === existingForm.domain)) {
      existingForm.domain = fallback
    }
    if (!options.find((option) => option.value === quickForm.domain)) {
      quickForm.domain = fallback
    }
  },
  { immediate: true },
)

watch(
  () => templateOptions.value,
  (options) => {
    if (options.length === 0) {
      templateForm.templateRef = ''
      return
    }
    if (!options.find((option) => option.value === templateForm.templateRef)) {
      const fallback = options[0]
      if (fallback) {
        templateForm.templateRef = fallback.value
      }
    }
  },
  { immediate: true },
)

watch(
  () => auth.user,
  (value) => {
    if (value) {
      loadCatalog()
      loadZones()
    } else {
      catalog.value = null
      catalogError.value = null
      zones.value = []
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
  />
  <hr />
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h2 class="text-2xl font-semibold text-[color:var(--text)]">
          Quick deploy
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Run a well known 
            <b>Quick Service</b>,
            forward a 
            <b>Local Port</b> 
            to the cloud, or a new fresh project from 
             <b>a Template</b>, 
            in seconds!
          </p>
        </div>
      </div>
      <UiPanel
        v-if="!isAuthenticated"
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-2 p-4 text-sm text-[color:var(--muted)]"
      >
        <span>Sign in to queue deploy jobs and host tunnel actions.</span>
        <UiButton :as="RouterLink" to="/login" variant="primary">
          <span class="flex items-center gap-2">
            <NavIcon name="login" class="h-4 w-4" />
            Sign in
          </span>
        </UiButton>
      </UiPanel>
      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
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
  <UiFieldGuidance
    :model-value="fieldGuidance.open.value"
    :content="fieldGuidance.content.value"
  />
  <QuickDeployTemplateForm
    v-model:open="templateFormOpen"
    v-model:template-ref="templateForm.templateRef"
    v-model:name="templateForm.name"
    v-model:subdomain="templateForm.subdomain"
    v-model:domain="templateForm.domain"
    :is-authenticated="isAuthenticated"
    :template-options="templateOptions"
    :template-repo-label="templateRepoLabel"
    :template-empty-state-message="templateEmptyStateMessage"
    :template-create-blocked="templateCreateBlocked"
    :template-selection-disabled="templateSelectionDisabled"
    :domain-options="domainOptions"
    :state="templateState"
    :show-guidance="fieldGuidance.show"
    :clear-guidance="fieldGuidance.clear"
    @submit="submitTemplate"
  />
  <QuickDeployExistingForm
    v-model:open="existingFormOpen"
    v-model:name="existingForm.name"
    v-model:subdomain="existingForm.subdomain"
    v-model:domain="existingForm.domain"
    v-model:port="existingForm.port"
    :is-authenticated="isAuthenticated"
    :domain-options="domainOptions"
    :state="existingState"
    :show-guidance="fieldGuidance.show"
    :clear-guidance="fieldGuidance.clear"
    @submit="submitExisting"
  />
  <QuickDeployQuickForm
    v-model:open="quickFormOpen"
    v-model:subdomain="quickForm.subdomain"
    v-model:domain="quickForm.domain"
    v-model:port="quickForm.port"
    v-model:image="quickForm.image"
    v-model:container-port="quickForm.containerPort"
    :title="selectedServiceName ? `Forward ${selectedServiceName}` : 'Quick service'"
    :is-authenticated="isAuthenticated"
    :domain-options="domainOptions"
    :state="quickState"
    :show-guidance="fieldGuidance.show"
    :clear-guidance="fieldGuidance.clear"
    @submit="submitQuick"
  />
</template>
