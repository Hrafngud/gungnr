<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFieldGuidance from '@/components/ui/UiFieldGuidance.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import NavIcon from '@/components/NavIcon.vue'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { hostApi } from '@/services/host'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import { useToastStore } from '@/stores/toasts'
import { useAuthStore } from '@/stores/auth'
import { useFieldGuidance } from '@/composables/useFieldGuidance'
import { usePageLoadingStore } from '@/stores/pageLoading'
import type { CloudflaredPreview, Settings, SettingsSources } from '@/types/settings'
import type { DockerContainer, DockerUsageSummary } from '@/types/host'
import type { LocalProject } from '@/types/projects'
import type { DockerHealth, TunnelHealth } from '@/types/health'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const settingsForm = reactive<Settings>({
  baseDomain: '',
  githubAppId: '',
  githubAppClientId: '',
  githubAppClientSecret: '',
  githubAppInstallationId: '',
  githubAppPrivateKey: '',
  cloudflareToken: '',
  cloudflareAccountId: '',
  cloudflareZoneId: '',
  cloudflaredTunnel: '',
  cloudflaredConfigPath: '',
})

const settingsKeys = [
  'baseDomain',
  'githubAppId',
  'githubAppClientId',
  'githubAppClientSecret',
  'githubAppInstallationId',
  'githubAppPrivateKey',
  'cloudflareToken',
  'cloudflareAccountId',
  'cloudflareZoneId',
  'cloudflaredTunnel',
  'cloudflaredConfigPath',
] as const

type SettingsKey = typeof settingsKeys[number]

const settingsSources = ref<SettingsSources | null>(null)
const cloudflaredTunnelName = ref<string | null>(null)
const templatesDir = ref<string | null>(null)

const toastStore = useToastStore()
const authStore = useAuthStore()
const fieldGuidance = useFieldGuidance()
const pageLoading = usePageLoadingStore()

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)

const revealGitHubSecret = ref(false)
const revealCloudflareToken = ref(false)
const importInput = ref<HTMLInputElement | null>(null)

const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)

const containers = ref<DockerContainer[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)
const usageSummary = ref<DockerUsageSummary | null>(null)
const usageLoading = ref(false)
const usageError = ref<string | null>(null)
const localProjects = ref<LocalProject[]>([])
const localProjectsLoading = ref(false)
const localProjectsError = ref<string | null>(null)

const settingsFormOpen = ref(false)
const ingressPreviewOpen = ref(false)
const statusFilter = ref<'all' | 'running' | 'stopped'>('all')
const projectFilter = ref('all')

type ContainerActionState = {
  stopping: boolean
  restarting: boolean
  removing: boolean
  error: string | null
}

const actionStates = reactive<Record<string, ContainerActionState>>({})
const removeModalOpen = ref(false)
const removeTarget = ref<DockerContainer | null>(null)
const removeTargetName = computed(() => removeTarget.value?.name ?? '')
const removeVolumes = ref(false)
const removeVolumesConfirm = ref(false)
const removeDescription = computed(() =>
  removeVolumes.value
    ? 'The container and its attached volumes will be permanently removed.'
    : 'The container will be stopped and permanently removed. Attached volumes will be preserved.',
)
const canConfirmRemove = computed(() => {
  const target = removeTarget.value
  if (!target) return false
  const state = actionStateFor(target)
  if (state.removing) return false
  if (removeVolumes.value && !removeVolumesConfirm.value) return false
  return true
})
const isAdmin = computed(() => authStore.isAdmin)

watch(removeModalOpen, (open) => {
  if (!open) {
    removeTarget.value = null
    removeVolumes.value = false
    removeVolumesConfirm.value = false
  }
})
watch(removeVolumes, (enabled) => {
  if (!enabled) {
    removeVolumesConfirm.value = false
  }
})

const hasPreview = computed(() => Boolean(preview.value?.contents))
const localProjectNames = computed(
  () => new Set(localProjects.value.map((project) => project.name.toLowerCase())),
)

const isRunningStatus = (status: string) => {
  const normalized = status.toLowerCase()
  return normalized.startsWith('up') || normalized.includes('running')
}

const isStoppedStatus = (status: string) => {
  const normalized = status.toLowerCase()
  return normalized.startsWith('exited') || normalized.includes('dead') || normalized.includes('created')
}
const projectOptions = computed(() => {
  const names = new Set<string>()
  containers.value.forEach((container) => {
    const project = container.project?.trim()
    if (project) {
      names.add(project)
    }
  })
  const options = Array.from(names)
    .sort((a, b) => a.localeCompare(b))
    .map((project) => ({ label: project, value: project }))
  return [{ label: 'All projects', value: 'all' }, ...options]
})

const runningCount = computed(() =>
  containers.value.filter((container) => isRunningStatus(container.status)).length,
)
const stoppedCount = computed(() =>
  containers.value.filter((container) => isStoppedStatus(container.status)).length,
)

const filteredContainers = computed(() => {
  const project = projectFilter.value
  return containers.value.filter((container) => {
    if (statusFilter.value === 'running' && !isRunningStatus(container.status)) {
      return false
    }
    if (statusFilter.value === 'stopped' && !isStoppedStatus(container.status)) {
      return false
    }
    if (project !== 'all' && container.project !== project) {
      return false
    }
    return true
  })
})

const usageCounts = computed(() => {
  const summary = usageSummary.value
  if (!summary) {
    return { containers: 0, images: 0, volumes: 0 }
  }
  if (projectFilter.value !== 'all' && summary.projectCounts) {
    return summary.projectCounts
  }
  return {
    containers: summary.containers?.count ?? 0,
    images: summary.images?.count ?? 0,
    volumes: summary.volumes?.count ?? 0,
  }
})

const statusTone = (status: string): BadgeTone => {
  const normalized = status.toLowerCase()
  if (isRunningStatus(normalized)) return 'ok'
  if (isStoppedStatus(normalized)) return 'error'
  if (normalized.includes('restarting') || normalized.includes('paused')) return 'warn'
  return 'neutral'
}

const healthTone = (status?: string): BadgeTone => {
  switch (status) {
    case 'ok':
      return 'ok'
    case 'warning':
      return 'warn'
    case 'missing':
      return 'neutral'
    case 'error':
      return 'error'
    default:
      return 'neutral'
  }
}

const exportSettings = () => {
  const payload = settingsKeys.reduce((acc, key) => {
    acc[key] = settingsForm[key]
    return acc
  }, {} as Record<SettingsKey, string>)
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'gungnr-settings.json'
  link.click()
  URL.revokeObjectURL(url)
  toastStore.success('Config exported.', 'Download started')
}

const applyImportedSettings = (payload: Partial<Settings>) => {
  settingsKeys.forEach((key) => {
    const value = payload[key]
    if (value !== undefined && value !== null) {
      settingsForm[key] = String(value)
    }
  })
}

const onImportClick = () => {
  importInput.value?.click()
}

const onImportFile = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    try {
      const payload = JSON.parse(String(reader.result ?? '{}')) as Partial<Settings>
      applyImportedSettings(payload)
      toastStore.success('Config imported.', 'Settings form updated')
    } catch {
      toastStore.error('Import failed.', 'Invalid JSON config')
    } finally {
      input.value = ''
    }
  }
  reader.onerror = () => {
    toastStore.error('Import failed.', 'Could not read config file')
    input.value = ''
  }
  reader.readAsText(file)
}

const actionStateFor = (container: DockerContainer): ContainerActionState => {
  if (!actionStates[container.id]) {
    actionStates[container.id] = {
      stopping: false,
      restarting: false,
      removing: false,
      error: null,
    }
  }
  return actionStates[container.id] as ContainerActionState
}


const ownershipLabel = (container: DockerContainer) => {
  if (localProjectsLoading.value) return 'Checking'
  if (localProjectsError.value) return 'Unknown'
  const project = container.project?.toLowerCase()
  if (project && localProjectNames.value.has(project)) {
    return 'Local template'
  }
  return 'External'
}

const loadSettings = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await settingsApi.get()
    Object.assign(settingsForm, data.settings)
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
    templatesDir.value = data.templatesDir ?? null
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  if (!isAdmin.value) {
    error.value = 'Admin access is required to update host or GitHub settings.'
    toastStore.error('Admin access required.', 'Read-only access')
    return
  }
  if (saving.value) return
  saving.value = true
  error.value = null
  success.value = null
  try {
    const { data } = await settingsApi.update({ ...settingsForm })
    Object.assign(settingsForm, data.settings)
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
    templatesDir.value = data.templatesDir ?? null
    success.value = 'Settings saved.'
    toastStore.success('Settings saved.', 'Settings updated')
    await loadPreview()
  } catch (err) {
    const message = apiErrorMessage(err)
    error.value = message
    toastStore.error(message, 'Save failed')
  } finally {
    saving.value = false
  }
}

const loadPreview = async () => {
  previewLoading.value = true
  previewError.value = null
  try {
    const { data } = await settingsApi.preview()
    preview.value = data.preview
  } catch (err) {
    previewError.value = apiErrorMessage(err)
    preview.value = null
  } finally {
    previewLoading.value = false
  }
}

const loadHealth = async () => {
  healthLoading.value = true
  const [dockerResult, tunnelResult] = await Promise.allSettled([
    healthApi.docker(),
    healthApi.tunnel(),
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

  healthLoading.value = false
}

const loadContainers = async () => {
  containersLoading.value = true
  containersError.value = null
  try {
    const { data } = await hostApi.listDocker()
    containers.value = data.containers
  } catch (err) {
    containersError.value = apiErrorMessage(err)
  } finally {
    containersLoading.value = false
  }
}

const loadDockerUsage = async () => {
  usageLoading.value = true
  usageError.value = null
  try {
    const project = projectFilter.value === 'all' ? undefined : projectFilter.value
    const { data } = await hostApi.dockerUsage(project)
    usageSummary.value = data.summary
  } catch (err) {
    usageError.value = apiErrorMessage(err)
    usageSummary.value = null
  } finally {
    usageLoading.value = false
  }
}

const loadLocalProjects = async () => {
  localProjectsLoading.value = true
  localProjectsError.value = null
  try {
    const { data } = await projectsApi.listLocal()
    localProjects.value = data.projects
  } catch (err) {
    localProjectsError.value = apiErrorMessage(err)
    localProjects.value = []
  } finally {
    localProjectsLoading.value = false
  }
}

const refreshHostData = async () => {
  await Promise.allSettled([loadContainers(), loadLocalProjects(), loadDockerUsage()])
}

const stopContainer = async (container: DockerContainer) => {
  const state = actionStateFor(container)
  if (state.stopping) return
  state.error = null
  state.stopping = true
  try {
    await hostApi.stopContainer(container.name)
    toastStore.success('Container stopped.', 'Docker')
    await loadContainers()
  } catch (err) {
    const message = apiErrorMessage(err)
    state.error = message
    toastStore.error(message, 'Stop failed')
  } finally {
    state.stopping = false
  }
}

const restartContainer = async (container: DockerContainer) => {
  const state = actionStateFor(container)
  if (state.restarting) return
  state.error = null
  state.restarting = true
  try {
    await hostApi.restartContainer(container.name)
    toastStore.success('Container restarted.', 'Docker')
    await loadContainers()
  } catch (err) {
    const message = apiErrorMessage(err)
    state.error = message
    toastStore.error(message, 'Restart failed')
  } finally {
    state.restarting = false
  }
}

const openRemoveModal = (container: DockerContainer) => {
  removeTarget.value = container
  removeVolumes.value = false
  removeModalOpen.value = true
}

const confirmRemove = async () => {
  const target = removeTarget.value
  if (!target) return
  if (removeVolumes.value && !removeVolumesConfirm.value) return
  const state = actionStateFor(target)
  if (state.removing) return
  state.error = null
  state.removing = true
  try {
    await hostApi.removeContainer(target.name, removeVolumes.value)
    toastStore.success('Container removed.', 'Docker')
    removeModalOpen.value = false
    removeTarget.value = null
    await loadContainers()
  } catch (err) {
    const message = apiErrorMessage(err)
    state.error = message
    toastStore.error(message, 'Remove failed')
  } finally {
    state.removing = false
  }
}

onMounted(async () => {
  pageLoading.start('Loading host settings...')
  await Promise.all([
    loadSettings(),
    loadPreview(),
    loadHealth(),
    loadContainers(),
    loadLocalProjects(),
    loadDockerUsage(),
  ])
  pageLoading.stop()
})

watch(projectOptions, (options) => {
  if (!options.find((option) => option.value === projectFilter.value)) {
    projectFilter.value = 'all'
  }
})

watch(projectFilter, () => {
  loadDockerUsage()
})
</script>

<template>
  <section class="page space-y-10">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Host settings
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Runtime configuration
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Override the backend defaults that power template deploys and host
          integrations.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="containersLoading"
          @click="refreshHostData"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="containersLoading" />
            Refresh host data
          </span>
        </UiButton>
        <UiButton
          variant="primary"
          size="sm"
          :disabled="!isAdmin"
          @click="settingsFormOpen = true"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="edit" class="h-3.5 w-3.5" />
            Edit settings
          </span>
        </UiButton>
        <UiButton
          variant="ghost"
          size="sm"
          @click="ingressPreviewOpen = true"
        >
          Ingress preview
        </UiButton>
      </div>
    </div>

    <UiInlineFeedback v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required to update host and GitHub settings.
    </UiInlineFeedback>

    <UiInlineFeedback v-if="error" tone="error">
      {{ error }}
    </UiInlineFeedback>

    <UiInlineFeedback v-if="success" tone="ok">
      {{ success }}
    </UiInlineFeedback>

    <hr />

    <div class="grid gap-6">
      <UiPanel as="section" class="space-y-6 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Host integrations
            </p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
              Containers
            </h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Stop, start, or remove containers while keeping their runtime ports visible.
            </p>
          </div>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="containersLoading"
          @click="refreshHostData"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="containersLoading" />
            Refresh list
            </span>
          </UiButton>
        </div>

        <UiState v-if="usageError" tone="error">
          {{ usageError }}
        </UiState>

        <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)]">
          <UiPanel variant="soft" class="space-y-2 p-3">
            <div class="flex flex-wrap items-center justify-between gap-2 break-words">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Disk usage
              </p>
              <UiInlineSpinner v-if="usageLoading" />
            </div>
            <p class="text-lg font-semibold text-[color:var(--text)]">
              {{ usageSummary?.totalSize || '—' }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              Overall Docker footprint.
            </p>
          </UiPanel>
          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Images
            </p>
            <p class="text-lg font-semibold text-[color:var(--text)]">
              {{ usageCounts.images }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ projectFilter === 'all' ? 'Total images' : 'Scoped to project' }}
            </p>
          </UiPanel>
          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Containers
            </p>
            <p class="text-lg font-semibold text-[color:var(--text)]">
              {{ usageCounts.containers }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ projectFilter === 'all' ? 'Total containers' : 'Scoped to project' }}
            </p>
          </UiPanel>
          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Volumes
            </p>
            <p class="text-lg font-semibold text-[color:var(--text)]">
              {{ usageCounts.volumes }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ projectFilter === 'all' ? 'Total volumes' : 'Scoped to project' }}
            </p>
          </UiPanel>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <UiButton
              variant="chip"
              size="chip"
              :class="statusFilter === 'all' ? 'border-[color:var(--accent)] text-[color:var(--accent-ink)]' : ''"
              @click="statusFilter = 'all'"
            >
              All ({{ containers.length }})
            </UiButton>
            <UiButton
              variant="chip"
              size="chip"
              :class="statusFilter === 'running' ? 'border-[color:var(--accent)] text-[color:var(--accent-ink)]' : ''"
              @click="statusFilter = 'running'"
            >
              Running ({{ runningCount }})
            </UiButton>
            <UiButton
              variant="chip"
              size="chip"
              :class="statusFilter === 'stopped' ? 'border-[color:var(--accent)] text-[color:var(--accent-ink)]' : ''"
              @click="statusFilter = 'stopped'"
            >
              Stopped ({{ stoppedCount }})
            </UiButton>
          </div>
          <div class="min-w-[200px]">
            <UiSelect v-model="projectFilter" :options="projectOptions" />
          </div>
        </div>

        <p v-if="projectFilter !== 'all'" class="text-xs text-[color:var(--muted)]">
          Showing resources for project "{{ projectFilter }}". Disk usage remains global.
        </p>

        <UiState v-if="containersError" tone="error">
          {{ containersError }}
        </UiState>

        <UiState v-else-if="containersLoading" loading>
          Loading Docker containers...
        </UiState>

        <UiState v-else-if="filteredContainers.length === 0">
          No containers match the current filters.
        </UiState>

        <div v-else class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <UiListRow
            v-for="container in filteredContainers"
            :key="container.id"
            as="article"
            class="space-y-4"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ container.service || 'Container' }}
                </p>
                <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
                  {{ container.name }}
                </h3>
                <p class="mt-1 text-xs text-[color:var(--muted)]">
                  {{ container.image }}
                </p>
              </div>
              <UiBadge :tone="statusTone(container.status)">
                {{ container.status }}
              </UiBadge>
            </div>

            <div class="space-y-2 text-xs text-[color:var(--muted)]">
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Ports</span>
                <span class="text-[color:var(--text)]">
                  {{ container.ports || '—' }}
                </span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Project</span>
                <span class="text-[color:var(--text)]">
                  {{ container.project || 'n/a' }}
                </span>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 break-words">
                <span>Ownership</span>
                <span class="text-[color:var(--text)]">
                  {{ ownershipLabel(container) }}
                </span>
              </div>
            </div>

            <UiPanel variant="soft" class="space-y-3 p-4">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Lifecycle actions
              </p>
              <div class="flex flex-wrap items-center gap-2">
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="actionStateFor(container).stopping"
                  @click="stopContainer(container)"
                >
                  <span class="flex items-center gap-2">
                    <NavIcon name="stop" class="h-3.5 w-3.5" />
                    <UiInlineSpinner v-if="actionStateFor(container).stopping" />
                    {{ actionStateFor(container).stopping ? 'Stopping...' : 'Stop' }}
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="actionStateFor(container).restarting || !isStoppedStatus(container.status)"
                  @click="restartContainer(container)"
                >
                  <span class="flex items-center gap-2">
                    <NavIcon name="restart" class="h-3.5 w-3.5" />
                    <UiInlineSpinner v-if="actionStateFor(container).restarting" />
                    {{
                      actionStateFor(container).restarting
                        ? 'Restarting...'
                        : isStoppedStatus(container.status)
                          ? 'Start'
                          : 'Restart'
                    }}
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  class="text-[color:var(--danger)]"
                  :disabled="actionStateFor(container).removing"
                  @click="openRemoveModal(container)"
                >
                  <span class="flex items-center gap-2">
                    <NavIcon name="trash" class="h-3.5 w-3.5" />
                    Remove
                  </span>
                </UiButton>
                <UiButton
                  :as="RouterLink"
                  :to="{ path: '/logs', query: { container: container.name } }"
                  variant="ghost"
                  size="sm"
                >
                  <span class="flex items-center gap-2">
                    <NavIcon name="logs" class="h-3.5 w-3.5" />
                    Logs
                  </span>
                </UiButton>
              </div>

              <UiInlineFeedback v-if="actionStateFor(container).error" tone="error">
                {{ actionStateFor(container).error }}
              </UiInlineFeedback>
            </UiPanel>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-4 p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Local templates
            </p>
            <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Local project folders
            </h3>
            <p class="mt-2 text-xs text-[color:var(--muted)]">
              These folders live in the templates directory. Lifecycle actions are safest for
              containers whose compose project matches one of these names.
            </p>
          </div>
          <UiButton
            variant="ghost"
            size="xs"
            :disabled="localProjectsLoading"
            @click="loadLocalProjects"
          >
            <span class="flex items-center gap-2">
              <NavIcon name="refresh" class="h-3 w-3" />
              <UiInlineSpinner v-if="localProjectsLoading" />
              Refresh folders
            </span>
          </UiButton>
        </div>

        <UiState v-if="localProjectsError" tone="error">
          {{ localProjectsError }}
        </UiState>

        <UiState v-else-if="localProjectsLoading" loading>
          Loading local folders...
        </UiState>

        <UiState v-else-if="localProjects.length === 0">
          No local template folders detected.
        </UiState>

        <div v-else class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <UiListRow
            v-for="project in localProjects"
            :key="project.path"
            class="flex flex-wrap items-center justify-between gap-2 break-words text-xs"
          >
            <span class="text-[color:var(--text)]">{{ project.name }}</span>
            <span class="truncate text-[color:var(--muted)]" :title="project.path">
              {{ project.path }}
            </span>
          </UiListRow>
        </div>
      </UiPanel>

    </div>

    <UiModal
      v-model="removeModalOpen"
      title="Remove container"
      :description="removeDescription"
    >
      <div class="space-y-4">
        <p class="text-sm text-[color:var(--muted)]">
          Remove <span class="text-[color:var(--text)]">{{ removeTargetName }}</span>? This cannot
          be undone for the container.
        </p>
        <UiToggle v-model="removeVolumes">Remove attached volumes</UiToggle>
        <div
          v-if="removeVolumes"
          class="space-y-2 rounded-xl border border-[color:var(--danger)]/40 bg-[color:var(--surface-inset)]/60 p-3"
        >
          <p class="text-xs text-[color:var(--danger)]">
            Attached volumes will be deleted and cannot be recovered.
          </p>
          <UiToggle v-model="removeVolumesConfirm">
            I confirm permanent volume deletion.
          </UiToggle>
        </div>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <UiButton variant="ghost" size="sm" @click="removeModalOpen = false">
            Cancel
          </UiButton>
          <UiButton
            variant="danger"
            size="sm"
            :disabled="!canConfirmRemove"
            @click="confirmRemove"
          >
            <span class="flex items-center gap-2">
              <UiInlineSpinner
                v-if="removeTarget && actionStateFor(removeTarget).removing"
              />
              {{ removeTarget && actionStateFor(removeTarget).removing ? 'Removing...' : 'Remove' }}
            </span>
          </UiButton>
        </div>
      </template>
    </UiModal>

    <UiFormSidePanel
      v-model="settingsFormOpen"
      title="Host configuration"
    >
      <div class="space-y-6">
        <div class="space-y-4">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Status + settings
            </p>
            <p class="mt-1 text-xs text-[color:var(--muted)]">
              Keep Docker and cloudflared healthy, then update the overrides that drive deploys.
            </p>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="healthLoading"
            @click="loadHealth"
          >
            <span class="flex items-center gap-2">
              <NavIcon name="refresh" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="healthLoading" />
              Refresh status
            </span>
          </UiButton>
        </div>

        <UiState v-if="healthLoading" loading>
          Checking host integrations...
        </UiState>

        <div v-else class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)]">
          <UiPanel variant="soft" class="space-y-2 p-3">
            <div class="flex flex-wrap items-center justify-between gap-2 break-words">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Docker
              </p>
              <UiBadge :tone="healthTone(dockerHealth?.status)">
                {{ dockerHealth?.status || 'unknown' }}
              </UiBadge>
            </div>
            <p class="text-xs text-[color:var(--muted)]">
              Containers
              <span class="ml-1 text-[color:var(--text)]">
                {{
                  dockerHealth && dockerHealth.status === 'ok'
                    ? dockerHealth.containers
                    : '—'
                }}
              </span>
            </p>
            <p
              v-if="dockerHealth?.detail"
              class="truncate text-xs text-[color:var(--muted)]"
              :title="dockerHealth.detail"
            >
              {{ dockerHealth.detail }}
            </p>
          </UiPanel>

          <UiPanel variant="soft" class="space-y-2 p-3">
            <div class="flex flex-wrap items-center justify-between gap-2 break-words">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Tunnel status
              </p>
              <UiBadge :tone="healthTone(tunnelHealth?.status)">
                {{ tunnelHealth?.status || 'unknown' }}
              </UiBadge>
            </div>
            <p class="text-xs text-[color:var(--muted)]">
              Connectors
              <span class="ml-1 text-[color:var(--text)]">
                {{
                  tunnelHealth &&
                  (tunnelHealth.status === 'ok' || tunnelHealth.status === 'warning')
                    ? tunnelHealth.connections
                    : '—'
                }}
              </span>
            </p>
            <p
              v-if="tunnelHealth?.detail"
              class="truncate text-xs text-[color:var(--muted)]"
              :title="tunnelHealth.detail"
            >
              {{ tunnelHealth.detail }}
            </p>
          </UiPanel>

          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Tunnel ref
            </p>
            <p
              class="truncate text-sm font-semibold text-[color:var(--text)]"
              :title="tunnelHealth?.tunnel || '—'"
            >
              {{ tunnelHealth?.tunnel || '—' }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              Source: {{ settingsSources?.cloudflaredTunnel || 'unset' }}
            </p>
          </UiPanel>

          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Config path
            </p>
            <p
              class="truncate text-sm font-semibold text-[color:var(--text)]"
              :title="tunnelHealth?.configPath || '—'"
            >
              {{ tunnelHealth?.configPath || '—' }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              Source: {{ settingsSources?.cloudflaredConfigPath || 'unset' }}
            </p>
          </UiPanel>

          <UiPanel variant="soft" class="space-y-2 p-3">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Templates dir
            </p>
            <p
              class="truncate text-sm font-semibold text-[color:var(--text)]"
              :title="templatesDir || '—'"
            >
              {{ templatesDir || '—' }}
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              Source: {{ settingsSources?.templatesDir || 'unset' }}
            </p>
          </UiPanel>
        </div>

        <hr />

        <form class="space-y-5" @submit.prevent="saveSettings">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Settings
            </p>
            <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Panel overrides
            </h3>
          </div>

          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Base domain
            </span>
            <UiInput
              v-model="settingsForm.baseDomain"
              type="text"
              placeholder="example.com"
              :disabled="loading"
              @focus="fieldGuidance.show({
                title: 'Base domain',
                description: 'Primary domain used to build subdomains for new services.',
              })"
              @blur="fieldGuidance.clear()"
            />
          </label>

          <div class="space-y-6">
            <div class="space-y-4">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                GitHub access
              </p>
              <p class="text-xs text-[color:var(--muted)]">
                Create-from-template uses GitHub App installation tokens minted from the credentials below.
              </p>

              <div class="grid gap-4">
                <label class="grid gap-2">
                  <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    GitHub App ID
                  </span>
                  <UiInput
                    v-model="settingsForm.githubAppId"
                    type="text"
                    placeholder="App ID"
                    :disabled="loading"
                    @focus="fieldGuidance.show({
                      title: 'GitHub App ID',
                      description: 'Numeric App ID used to mint installation tokens for template generation.',
                      links: [
                        { label: 'Create a GitHub App', href: 'https://github.com/settings/apps/new' },
                      ],
                    })"
                    @blur="fieldGuidance.clear()"
                  />
                </label>

                <label class="grid gap-2">
                  <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    GitHub App client ID
                  </span>
                  <UiInput
                    v-model="settingsForm.githubAppClientId"
                    type="text"
                    placeholder="Client ID"
                    :disabled="loading"
                    @focus="fieldGuidance.show({
                      title: 'GitHub App client ID',
                      description: 'Client ID shown in the GitHub App settings.',
                      links: [
                        { label: 'GitHub App settings', href: 'https://github.com/settings/apps' },
                      ],
                    })"
                    @blur="fieldGuidance.clear()"
                  />
                </label>

                <label class="grid gap-2">
                  <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    GitHub App client secret
                  </span>
                  <div class="relative">
                    <UiInput
                      v-model="settingsForm.githubAppClientSecret"
                      :type="revealGitHubSecret ? 'text' : 'password'"
                      class="pr-12"
                      placeholder="Client secret"
                      :disabled="loading"
                      @focus="fieldGuidance.show({
                        title: 'GitHub App client secret',
                        description: 'Client secret for GitHub App OAuth flows. Stored for future use.',
                        links: [
                          { label: 'GitHub App settings', href: 'https://github.com/settings/apps' },
                        ],
                      })"
                      @blur="fieldGuidance.clear()"
                    />
                    <UiButton
                      type="button"
                      variant="ghost"
                      size="xs"
                      class="absolute right-2 top-1/2 h-8 w-8 -translate-y-1/2 px-0 py-0"
                      :disabled="loading"
                      :aria-label="revealGitHubSecret ? 'Hide GitHub App client secret' : 'Show GitHub App client secret'"
                      @click="revealGitHubSecret = !revealGitHubSecret"
                    >
                      <NavIcon :name="revealGitHubSecret ? 'eye-off' : 'eye'" class="h-4 w-4" />
                    </UiButton>
                  </div>
                </label>

                <label class="grid gap-2">
                  <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                    GitHub App installation ID
                  </span>
                  <UiInput
                    v-model="settingsForm.githubAppInstallationId"
                    type="text"
                    placeholder="Installation ID"
                    :disabled="loading"
                    @focus="fieldGuidance.show({
                      title: 'GitHub App installation ID',
                      description: 'Installation ID for the org or user that will own the generated repos.',
                      links: [
                        { label: 'GitHub App installs', href: 'https://github.com/settings/installations' },
                      ],
                    })"
                    @blur="fieldGuidance.clear()"
                  />
                </label>
              </div>

              <label class="grid gap-2">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  GitHub App private key
                </span>
                <textarea
                  v-model="settingsForm.githubAppPrivateKey"
                  class="input min-h-[160px] resize-y"
                  placeholder="-----BEGIN PRIVATE KEY-----"
                  :disabled="loading"
                  @focus="fieldGuidance.show({
                    title: 'GitHub App private key',
                    description: 'Paste the PEM private key to sign installation token requests.',
                    links: [
                      { label: 'Create a GitHub App', href: 'https://github.com/settings/apps/new' },
                    ],
                  })"
                  @blur="fieldGuidance.clear()"
                />
              </label>

              <p class="text-xs text-[color:var(--muted)]">
                GitHub App permissions: Repository Administration (write), Contents (read), Metadata (read). Install the app on the template repo and the target owner.
              </p>
            </div>

            <div class="space-y-4">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Cloudflare access
              </p>

              <label class="grid gap-2">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Cloudflare API token
                </span>
                <div class="relative">
                  <UiInput
                    v-model="settingsForm.cloudflareToken"
                    :type="revealCloudflareToken ? 'text' : 'password'"
                    class="pr-12"
                    placeholder="cf_••••••"
                    :disabled="loading"
                    @focus="fieldGuidance.show({
                      title: 'Cloudflare API token',
                      description: 'Token with Tunnel:Edit and DNS:Edit for the selected account and zone.',
                      links: [
                        { label: 'Cloudflare API tokens', href: 'https://dash.cloudflare.com/profile/api-tokens' },
                      ],
                    })"
                    @blur="fieldGuidance.clear()"
                  />
                  <UiButton
                    type="button"
                    variant="ghost"
                    size="xs"
                    class="absolute right-2 top-1/2 h-8 w-8 -translate-y-1/2 px-0 py-0"
                    :disabled="loading"
                    :aria-label="revealCloudflareToken ? 'Hide Cloudflare API token' : 'Show Cloudflare API token'"
                    @click="revealCloudflareToken = !revealCloudflareToken"
                  >
                    <NavIcon :name="revealCloudflareToken ? 'eye-off' : 'eye'" class="h-4 w-4" />
                  </UiButton>
                </div>
              </label>

              <label class="grid gap-2">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Cloudflare account ID
                </span>
                <UiInput
                  v-model="settingsForm.cloudflareAccountId"
                  type="text"
                  placeholder="Account ID"
                  :disabled="loading"
                  @focus="fieldGuidance.show({
                    title: 'Cloudflare account ID',
                    description: 'Account identifier that owns the tunnel and DNS zone.',
                    links: [
                      { label: 'Cloudflare dashboard', href: 'https://dash.cloudflare.com' },
                    ],
                  })"
                  @blur="fieldGuidance.clear()"
                />
              </label>

              <label class="grid gap-2">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Cloudflare zone ID
                </span>
                <UiInput
                  v-model="settingsForm.cloudflareZoneId"
                  type="text"
                  placeholder="Zone ID"
                  :disabled="loading"
                  @focus="fieldGuidance.show({
                    title: 'Cloudflare zone ID',
                    description: 'Zone identifier for the base domain you are routing.',
                    links: [
                      { label: 'Cloudflare dashboard', href: 'https://dash.cloudflare.com' },
                    ],
                  })"
                  @blur="fieldGuidance.clear()"
                />
              </label>

              <label class="grid gap-2">
                <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Cloudflared tunnel (name or ID)
                </span>
                <UiInput
                  v-model="settingsForm.cloudflaredTunnel"
                  type="text"
                  placeholder="Tunnel name or UUID"
                  :disabled="loading"
                  @focus="fieldGuidance.show({
                    title: 'Cloudflared tunnel',
                    description: 'Name or UUID used to resolve and update ingress rules.',
                    links: [
                      { label: 'Tunnel guide', href: 'https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/tunnel-guide/' },
                    ],
                  })"
                  @blur="fieldGuidance.clear()"
                />
              </label>

              <p class="text-xs text-[color:var(--muted)]">
                Use a Cloudflare API token (not a global API key) with
                Account:Cloudflare Tunnel:Edit and Zone:DNS:Edit for the configured account
                and zone.
              </p>

              <div
                v-if="settingsSources || cloudflaredTunnelName"
                class="space-y-2 rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)]/80 p-3 text-[11px] text-[color:var(--muted)]"
              >
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Tunnel ref (resolved)</span>
                  <span class="text-[color:var(--text)]">
                    {{ cloudflaredTunnelName || '—' }}
                  </span>
                </UiListRow>
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Tunnel source</span>
                  <span class="text-[color:var(--text)]">
                    {{ settingsSources?.cloudflaredTunnel || 'unset' }}
                  </span>
                </UiListRow>
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Account ID source</span>
                  <span class="text-[color:var(--text)]">
                    {{ settingsSources?.cloudflareAccountId || 'unset' }}
                  </span>
                </UiListRow>
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Zone ID source</span>
                  <span class="text-[color:var(--text)]">
                    {{ settingsSources?.cloudflareZoneId || 'unset' }}
                  </span>
                </UiListRow>
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Token source</span>
                  <span class="text-[color:var(--text)]">
                    {{ settingsSources?.cloudflareToken || 'unset' }}
                  </span>
                </UiListRow>
                <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
                  <span>Config path source</span>
                  <span class="text-[color:var(--text)]">
                    {{ settingsSources?.cloudflaredConfigPath || 'unset' }}
                  </span>
                </UiListRow>
              </div>
            </div>
          </div>

          <label class="grid gap-2">
            <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflared config path
            </span>
            <UiInput
              v-model="settingsForm.cloudflaredConfigPath"
              type="text"
              placeholder="~/.cloudflared/config.yml"
              :disabled="loading"
              @focus="fieldGuidance.show({
                title: 'Cloudflared config path',
                description: 'Path to the config.yml used by the host cloudflared service.',
                links: [
                  { label: 'Tunnel guide', href: 'https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/tunnel-guide/' },
                ],
              })"
              @blur="fieldGuidance.clear()"
            />
          </label>

          <UiInlineFeedback v-if="error" tone="error">
            {{ error }}
          </UiInlineFeedback>

          <UiInlineFeedback v-if="success" tone="ok">
            {{ success }}
          </UiInlineFeedback>

          <input
            ref="importInput"
            type="file"
            accept="application/json"
            class="hidden"
            @change="onImportFile"
          />

          <div class="flex flex-wrap items-center gap-3">
            <UiButton
              type="button"
              variant="ghost"
              size="md"
              :disabled="loading"
              @click="exportSettings"
            >
              Export config
            </UiButton>
            <UiButton
              type="button"
              variant="ghost"
              size="md"
              :disabled="loading"
              @click="onImportClick"
            >
              Import config
            </UiButton>
            <UiButton
              type="submit"
              variant="primary"
              size="md"
              :disabled="saving || loading || !isAdmin"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="saving" />
                {{ saving ? 'Saving...' : 'Save settings' }}
              </span>
            </UiButton>
            <UiButton variant="ghost" size="md" :disabled="loading" @click="loadSettings">
              <span class="flex items-center gap-2">
                <NavIcon name="refresh" class="h-3.5 w-3.5" />
                Reload
              </span>
            </UiButton>
          </div>
        </form>
      </div>
    </UiFormSidePanel>

    <UiFormSidePanel
      v-model="ingressPreviewOpen"
      title="Ingress preview"
      eyebrow="Ingress"
    >
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-2 break-words">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflared config
            </p>
            <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Live ingress preview
            </h3>
          </div>
          <UiButton
            variant="ghost"
            size="xs"
            :disabled="previewLoading"
            @click="loadPreview"
          >
            <span class="flex items-center gap-2">
              <NavIcon name="refresh" class="h-3 w-3" />
              <UiInlineSpinner v-if="previewLoading" />
              Refresh
            </span>
          </UiButton>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Previewing {{ preview?.path || 'cloudflared config' }}.
        </p>

        <UiState v-if="previewLoading" loading>
          Loading config preview...
        </UiState>

        <UiState v-else-if="previewError" tone="error">
          {{ previewError }}
        </UiState>

        <pre
          v-else-if="hasPreview"
          class="max-h-80 overflow-auto border border-[color:var(--border)] bg-[color:var(--surface-inset)]/90 p-4 text-xs text-[color:var(--accent-ink)]"
        ><code>{{ preview?.contents }}</code></pre>

        <UiState v-else>
          Cloudflared config not loaded yet.
        </UiState>
      </div>
    </UiFormSidePanel>

    <UiFieldGuidance
      :model-value="fieldGuidance.open.value"
      :content="fieldGuidance.content.value"
    />
  </section>
</template>
