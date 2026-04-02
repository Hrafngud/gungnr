<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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
import UiRuntimeLedMeter from '@/components/ui/UiRuntimeLedMeter.vue'
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
import { clampPercent, formatBytes, formatPercent } from '@/utils/runtimeMetrics'
import type { CloudflaredPreview, Settings, SettingsSources } from '@/types/settings'
import type {
  DockerContainer,
  DockerReadDiagnostic,
  DockerUsageSummary,
  HostRuntimeSnapshot,
  HostRuntimeStreamSample,
} from '@/types/host'
import type { LocalProject } from '@/types/projects'
import type { DockerHealth, TunnelHealth } from '@/types/health'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const settingsForm = reactive<Settings>({
  baseDomain: '',
  additionalDomains: [],
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
const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)

const containers = ref<DockerContainer[]>([])
const containerDiagnostics = ref<DockerReadDiagnostic[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)
const usageSummary = ref<DockerUsageSummary | null>(null)
const usageDiagnostics = ref<DockerReadDiagnostic[]>([])
const usageLoading = ref(false)
const usageError = ref<string | null>(null)
const runtimeSnapshot = ref<HostRuntimeSnapshot | null>(null)
const runtimeSnapshotLoading = ref(false)
const runtimeSnapshotError = ref<string | null>(null)
const runtimeStreamSample = ref<HostRuntimeStreamSample | null>(null)
const runtimeStreamState = ref<'idle' | 'connecting' | 'live' | 'error'>('idle')
const runtimeStreamError = ref<string | null>(null)
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
let runtimeStreamSource: EventSource | null = null
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
const selectedProjectRestarting = ref(false)
const selectedProjectRestartError = ref<string | null>(null)

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
const localProjectNames = computed(() => {
  const names = localProjects.value
    .map((project) => (typeof project?.name === 'string' ? project.name.trim().toLowerCase() : ''))
    .filter((name) => name.length > 0)
  return new Set(names)
})

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

const hostDataRefreshing = computed(() =>
  containersLoading.value || usageLoading.value || localProjectsLoading.value || runtimeSnapshotLoading.value,
)

const formatClockSpeed = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return 'Speed unavailable'
  if (value >= 1000) return `${(value / 1000).toFixed(2)} GHz`
  return `${value.toFixed(value >= 100 ? 0 : 1)} MHz`
}

const formatMemorySpeed = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return 'Speed unavailable'
  return `${Math.round(value)} MT/s`
}

const runtimeIdentityCards = computed(() => {
  const stats = runtimeSnapshot.value
  if (!stats) return []
  const diskAvailableBytes = stats.disk.availableBytes ?? stats.disk.freeBytes
  return [
    {
      key: 'uptime',
      label: 'Uptime',
      value: stats.uptimeHuman || '—',
      meta: `${stats.uptimeSeconds} seconds`,
    },
    {
      key: 'system-image',
      label: 'System image',
      value: stats.systemImage || 'Unknown system image',
      meta: stats.kernel || 'Kernel unknown',
    },
    {
      key: 'cpu',
      label: 'CPU',
      value: stats.cpu.model || 'Unknown CPU',
      meta: `${formatClockSpeed(stats.cpu.speedMHz)} · ${stats.cpu.threads} threads · ${stats.cpu.cores} cores`,
    },
    {
      key: 'gpu',
      label: 'GPU',
      value: stats.gpu?.model || 'Not detected',
      meta: stats.gpu ? `${formatClockSpeed(stats.gpu.speedMHz)} graphics clock` : 'Optional hardware probe',
    },
    {
      key: 'hostname',
      label: 'Hostname',
      value: stats.hostname || 'Unknown host',
      meta: 'Resolved from host runtime probe',
    },
    {
      key: 'total-ram',
      label: 'Total RAM',
      value: formatBytes(stats.memory.totalBytes),
      meta: `${formatMemorySpeed(stats.memory.speedMTs)} · ${formatBytes(stats.memory.freeBytes)} free`,
    },
    {
      key: 'disk-capacity',
      label: 'Disk capacity',
      value: formatBytes(stats.disk.totalBytes),
      meta: `${formatBytes(stats.disk.freeBytes)} free · ${formatBytes(diskAvailableBytes)} available`,
      wide: true,
    },
  ]
})

const runtimeLiveIndicators = computed(() => {
  const sample = runtimeStreamSample.value
  if (!sample) return []
  const snapshot = runtimeSnapshot.value
  const panelSummary = snapshot?.panel
  const projectsSummary = snapshot?.projects
  return [
    {
      key: 'host-memory',
      scope: 'Host',
      metric: 'RAM',
      value: formatBytes(sample.host.memoryUsedBytes),
      meta: snapshot
        ? `${formatPercent(sample.host.memoryUsedPercent)} of ${formatBytes(snapshot.memory.totalBytes)}`
        : formatPercent(sample.host.memoryUsedPercent),
      percent: clampPercent(sample.host.memoryUsedPercent),
    },
    {
      key: 'panel-cpu',
      scope: 'Gungnr panel',
      metric: 'CPU',
      value: formatPercent(sample.panel.cpuUsedPercent),
      meta: panelSummary
        ? `${panelSummary.runningContainers}/${panelSummary.containers} running containers`
        : 'Live container CPU stream',
      percent: clampPercent(sample.panel.cpuUsedPercent),
    },
    {
      key: 'panel-memory',
      scope: 'Gungnr panel',
      metric: 'RAM',
      value: formatBytes(sample.panel.memoryUsedBytes),
      meta: `${formatPercent(sample.panel.memorySharePercent)} of host memory`,
      percent: clampPercent(sample.panel.memorySharePercent),
    },
    {
      key: 'projects-cpu',
      scope: 'Projects',
      metric: 'CPU',
      value: formatPercent(sample.projects.cpuUsedPercent),
      meta: projectsSummary
        ? `${projectsSummary.runningContainers}/${projectsSummary.containers} running containers`
        : 'Live container CPU stream',
      percent: clampPercent(sample.projects.cpuUsedPercent),
    },
    {
      key: 'projects-memory',
      scope: 'Projects',
      metric: 'RAM',
      value: formatBytes(sample.projects.memoryUsedBytes),
      meta: `${formatPercent(sample.projects.memorySharePercent)} of host memory`,
      percent: clampPercent(sample.projects.memorySharePercent),
    },
  ]
})

const runtimeSnapshotIndicators = computed(() => {
  const snapshot = runtimeSnapshot.value
  if (!snapshot) return []
  return [
    {
      key: 'host-disk',
      scope: 'Host',
      metric: 'Disk',
      value: formatBytes(snapshot.disk.usedBytes),
      meta: `${formatPercent(snapshot.disk.usedPercent)} of ${formatBytes(snapshot.disk.totalBytes)}`,
      percent: clampPercent(snapshot.disk.usedPercent),
    },
    {
      key: 'panel-disk',
      scope: 'Gungnr panel',
      metric: 'Disk',
      value: formatBytes(snapshot.panel.diskUsedBytes),
      meta: `${formatPercent(snapshot.panel.diskSharePercent)} of host disk`,
      percent: clampPercent(snapshot.panel.diskSharePercent),
    },
    {
      key: 'projects-disk',
      scope: 'Projects',
      metric: 'Disk',
      value: formatBytes(snapshot.projects.diskUsedBytes),
      meta: `${formatPercent(snapshot.projects.diskSharePercent)} of host disk`,
      percent: clampPercent(snapshot.projects.diskSharePercent),
    },
  ]
})

const runtimeSnapshotWarnings = computed(() => {
  const warnings = runtimeSnapshot.value?.warnings ?? []
  return warnings.slice(0, 3)
})

const runtimeStreamWarnings = computed(() => {
  const warnings = runtimeStreamSample.value?.warnings ?? []
  return warnings.slice(0, 3)
})

const runtimeStreamBadge = computed<{ tone: BadgeTone; label: string }>(() => {
  const intervalMs = runtimeStreamSample.value?.intervalMs ?? 100
  switch (runtimeStreamState.value) {
    case 'live':
      return { tone: 'ok', label: `Live · ${intervalMs}ms` }
    case 'connecting':
      return { tone: 'warn', label: 'Connecting…' }
    case 'error':
      return { tone: 'error', label: 'Stream error' }
    default:
      return { tone: 'neutral', label: 'Stream idle' }
  }
})

const containerDiagnosticMessage = computed(() => {
  const diagnostics = containerDiagnostics.value
  if (diagnostics.length === 0) return ''
  return diagnostics.map((diagnostic) => diagnostic.message).join(' ')
})

const usageDiagnosticMessage = computed(() => {
  const diagnostics = usageDiagnostics.value
  if (diagnostics.length === 0) return ''
  return diagnostics.map((diagnostic) => diagnostic.message).join(' ')
})

const statusTone = (status: string): BadgeTone => {
  const normalized = status.toLowerCase()
  if (isRunningStatus(normalized)) return 'ok'
  if (isStoppedStatus(normalized)) return 'error'
  if (normalized.includes('restarting') || normalized.includes('paused')) return 'warn'
  return 'neutral'
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
  const localNames = localProjectNames.value
  if (project && localNames.has(project)) {
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
    error.value = 'Admin access is required to update template settings.'
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
  containerDiagnostics.value = []
  try {
    const { data } = await hostApi.listDocker()
    containers.value = Array.isArray(data.containers) ? data.containers : []
    containerDiagnostics.value = Array.isArray(data.diagnostics) ? data.diagnostics : []
  } catch (err) {
    containersError.value = apiErrorMessage(err)
    containers.value = []
    containerDiagnostics.value = []
  } finally {
    containersLoading.value = false
  }
}

const loadDockerUsage = async () => {
  usageLoading.value = true
  usageError.value = null
  usageDiagnostics.value = []
  try {
    const project = projectFilter.value === 'all' ? undefined : projectFilter.value
    const { data } = await hostApi.dockerUsage(project)
    usageSummary.value = data.summary
    usageDiagnostics.value = Array.isArray(data.diagnostics) ? data.diagnostics : []
  } catch (err) {
    usageError.value = apiErrorMessage(err)
    usageSummary.value = null
    usageDiagnostics.value = []
  } finally {
    usageLoading.value = false
  }
}

const loadRuntimeSnapshot = async () => {
  runtimeSnapshotLoading.value = true
  runtimeSnapshotError.value = null
  try {
    const { data } = await hostApi.runtimeSnapshot()
    runtimeSnapshot.value = data.snapshot ?? null
  } catch (err) {
    runtimeSnapshotError.value = apiErrorMessage(err)
  } finally {
    runtimeSnapshotLoading.value = false
  }
}

const closeRuntimeSignalStream = () => {
  if (!runtimeStreamSource) return
  runtimeStreamSource.close()
  runtimeStreamSource = null
}

const startRuntimeSignalStream = () => {
  closeRuntimeSignalStream()
  runtimeStreamState.value = 'connecting'
  runtimeStreamError.value = null

  const source = new EventSource(hostApi.runtimeStatsStreamUrl(), { withCredentials: true })
  runtimeStreamSource = source

  source.onopen = () => {
    if (runtimeStreamSource !== source) return
    runtimeStreamState.value = 'live'
  }

  source.addEventListener('sample', (event) => {
    if (runtimeStreamSource !== source) return
    const message = event as MessageEvent
    try {
      runtimeStreamSample.value = JSON.parse(message.data) as HostRuntimeStreamSample
      runtimeStreamError.value = null
      runtimeStreamState.value = 'live'
    } catch {
      runtimeStreamError.value = 'Malformed runtime signal sample.'
      runtimeStreamState.value = 'error'
    }
  })

  source.addEventListener('error', (event) => {
    if (runtimeStreamSource !== source) return
    const message = event as MessageEvent
    if (message?.data) {
      try {
        const payload = JSON.parse(message.data) as { message?: string }
        runtimeStreamError.value = payload.message || 'Runtime signal stream error.'
      } catch {
        runtimeStreamError.value = 'Runtime signal stream error.'
      }
      runtimeStreamState.value = 'error'
      return
    }
    if (source.readyState === EventSource.CLOSED) {
      runtimeStreamState.value = 'idle'
    }
  })
}

const loadLocalProjects = async () => {
  localProjectsLoading.value = true
  localProjectsError.value = null
  try {
    const { data } = await projectsApi.listLocal()
    localProjects.value = Array.isArray(data.projects) ? data.projects : []
  } catch (err) {
    localProjectsError.value = apiErrorMessage(err)
    localProjects.value = []
  } finally {
    localProjectsLoading.value = false
  }
}

const refreshHostData = async () => {
  await Promise.allSettled([loadContainers(), loadLocalProjects(), loadDockerUsage(), loadRuntimeSnapshot()])
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

const restartSelectedProjectStack = async () => {
  if (!isAdmin.value) {
    toastStore.error('Admin access required.', 'Restart blocked')
    return
  }
  if (projectFilter.value === 'all') {
    toastStore.error('Select a project first.', 'Restart blocked')
    return
  }
  if (selectedProjectRestarting.value) return
  selectedProjectRestartError.value = null
  selectedProjectRestarting.value = true
  try {
    const { data } = await hostApi.restartProject(projectFilter.value)
    toastStore.success(
      `Project "${projectFilter.value}" restart queued (job #${data.job.id}).`,
      'Docker compose',
    )
  } catch (err) {
    const message = apiErrorMessage(err)
    selectedProjectRestartError.value = message
    toastStore.error(message, 'Queue failed')
  } finally {
    selectedProjectRestarting.value = false
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
  startRuntimeSignalStream()
  await Promise.all([
    loadSettings(),
    loadPreview(),
    loadHealth(),
    loadContainers(),
    loadLocalProjects(),
    loadDockerUsage(),
    loadRuntimeSnapshot(),
  ])
  pageLoading.stop()
})

onBeforeUnmount(() => {
  closeRuntimeSignalStream()
})

watch(projectOptions, (options) => {
  if (!options.find((option) => option.value === projectFilter.value)) {
    projectFilter.value = 'all'
  }
})

watch(projectFilter, () => {
  selectedProjectRestartError.value = null
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
          Runtime overview
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Review host integrations and template credentials that power deploy workflows.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="hostDataRefreshing"
          @click="refreshHostData"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="hostDataRefreshing" />
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
            Template settings
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
      Read-only access: admin permissions are required to update template settings.
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
        <div class="w-full flex flex-wrap items-center justify-between gap-3">
          <div class="w-full flex flex-col">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Host stats
            </p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
              Runtime signals
            </h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Refresh-only host snapshot plus a debounced live stream for CPU and RAM signals.
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <UiBadge :tone="runtimeStreamBadge.tone">
              {{ runtimeStreamBadge.label }}
            </UiBadge>
            <UiButton
              variant="ghost"
              size="sm"
              :disabled="runtimeSnapshotLoading"
              @click="loadRuntimeSnapshot"
            >
              <span class="flex items-center gap-2">
                <NavIcon name="refresh" class="h-3.5 w-3.5" />
                <UiInlineSpinner v-if="runtimeSnapshotLoading" />
                Refresh snapshot
              </span>
            </UiButton>
          </div>
        </div>

        <UiState v-if="runtimeSnapshotError" tone="error">
          {{ runtimeSnapshotError }}
        </UiState>

        <UiState v-else-if="runtimeSnapshotLoading && !runtimeSnapshot" loading>
          Loading runtime snapshot...
        </UiState>

        <template v-else-if="runtimeSnapshot">
          <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
            <div class="grid gap-3 md:grid-cols-2 h-fit">
              <UiPanel
                v-for="card in runtimeIdentityCards"
                :key="card.key"
                variant="soft"
                :class="['space-y-2 p-3', card.wide ? 'md:col-span-2' : '']"
              >
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ card.label }}
                </p>
                <p class="text-sm font-semibold text-[color:var(--text)] break-words">
                  {{ card.value }}
                </p>
                <p class="text-xs text-[color:var(--muted)] break-words">
                  {{ card.meta }}
                </p>
              </UiPanel>
            </div>

            <div class="grid gap-4 h-fit">
              <UiPanel variant="soft" class="space-y-4 p-4">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p class="text-xs uppercase tracking-[0.24em] text-[color:var(--muted-2)]">
                      Live stream
                    </p>
                    <p class="mt-1 text-xs text-[color:var(--muted)]">
                      Debounced {{ runtimeStreamSample?.intervalMs ?? 100 }}ms updates for CPU and RAM.
                    </p>
                  </div>
                  <UiBadge :tone="runtimeStreamBadge.tone">
                    {{ runtimeStreamBadge.label }}
                  </UiBadge>
                </div>
                <div class="grid gap-3 md:grid-cols-2">
                  <article
                    v-for="indicator in runtimeLiveIndicators"
                    :key="indicator.key"
                    class="space-y-2 rounded-md border border-[color:var(--border-soft)] bg-[color:var(--surface)]/70 p-3"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <p class="text-xs uppercase tracking-[0.24em] text-[color:var(--muted-2)]">
                        {{ indicator.scope }}
                      </p>
                      <p class="text-[11px] uppercase tracking-[0.24em] text-[color:var(--muted)]">
                        {{ indicator.metric }}
                      </p>
                    </div>
                    <p class="text-sm font-semibold text-[color:var(--text)]">
                      {{ indicator.value }}
                    </p>
                    <UiRuntimeLedMeter
                      :label="`${indicator.scope} ${indicator.metric}`"
                      :percent="indicator.percent"
                    />
                    <p class="text-xs text-[color:var(--muted)]">
                      {{ indicator.meta }}
                    </p>
                  </article>
                </div>
                <UiInlineFeedback v-if="runtimeStreamError" tone="error">
                  {{ runtimeStreamError }}
                </UiInlineFeedback>
                <UiInlineFeedback v-else-if="runtimeStreamWarnings.length > 0" tone="warn">
                  {{ runtimeStreamWarnings.join(' · ') }}
                </UiInlineFeedback>
              </UiPanel>

              <UiPanel variant="soft" class="space-y-4 p-4">
                <div>
                  <p class="text-xs uppercase tracking-[0.24em] text-[color:var(--muted-2)]">
                    Refresh-only snapshot
                  </p>
                  <p class="mt-1 text-xs text-[color:var(--muted)]">
                    Disk and identity data only change when the snapshot is refreshed.
                  </p>
                </div>
                <div class="grid gap-3 md:grid-cols-2">
                  <article
                    v-for="indicator in runtimeSnapshotIndicators"
                    :key="indicator.key"
                    class="space-y-2 rounded-md border border-[color:var(--border-soft)] bg-[color:var(--surface)]/70 p-3"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <p class="text-xs uppercase tracking-[0.24em] text-[color:var(--muted-2)]">
                        {{ indicator.scope }}
                      </p>
                      <p class="text-[11px] uppercase tracking-[0.24em] text-[color:var(--muted)]">
                        {{ indicator.metric }}
                      </p>
                    </div>
                    <p class="text-sm font-semibold text-[color:var(--text)]">
                      {{ indicator.value }}
                    </p>
                    <UiRuntimeLedMeter
                      :label="`${indicator.scope} ${indicator.metric}`"
                      :percent="indicator.percent"
                    />
                    <p class="text-xs text-[color:var(--muted)]">
                      {{ indicator.meta }}
                    </p>
                  </article>
                </div>
                <UiInlineFeedback v-if="runtimeSnapshotWarnings.length > 0" tone="warn">
                  {{ runtimeSnapshotWarnings.join(' · ') }}
                </UiInlineFeedback>
              </UiPanel>
            </div>
          </div>
        </template>

        <UiState v-else>
          Runtime snapshot not loaded yet.
        </UiState>
      </UiPanel>

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
          :disabled="hostDataRefreshing"
          @click="refreshHostData"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="hostDataRefreshing" />
            Refresh list
            </span>
          </UiButton>
        </div>

        <UiState v-if="usageError" tone="error">
          {{ usageError }}
        </UiState>
        <UiInlineFeedback v-else-if="usageDiagnosticMessage" tone="warn">
          {{ usageDiagnosticMessage }}
        </UiInlineFeedback>

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
          <div class="flex items-center gap-2">
            <div class="min-w-[200px]">
              <UiSelect v-model="projectFilter" :options="projectOptions" />
            </div>
            <UiButton
              v-if="projectFilter !== 'all'"
              variant="ghost"
              size="sm"
              :disabled="selectedProjectRestarting || !isAdmin"
              @click="restartSelectedProjectStack"
            >
              <span class="flex items-center gap-2">
                <NavIcon name="restart" class="h-3.5 w-3.5" />
                <UiInlineSpinner v-if="selectedProjectRestarting" />
                {{ selectedProjectRestarting ? 'Restarting stack...' : 'Restart stack' }}
              </span>
            </UiButton>
          </div>
        </div>

        <p v-if="projectFilter !== 'all'" class="text-xs text-[color:var(--muted)]">
          Showing resources for project "{{ projectFilter }}". Disk usage remains global.
        </p>
        <UiInlineFeedback v-if="selectedProjectRestartError" tone="error">
          {{ selectedProjectRestartError }}
        </UiInlineFeedback>

        <UiState v-if="containersError" tone="error">
          {{ containersError }}
        </UiState>
        <UiInlineFeedback v-else-if="containerDiagnosticMessage" tone="warn">
          {{ containerDiagnosticMessage }}
        </UiInlineFeedback>

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
      title="Host status & templates"
    >
      <div class="space-y-6">
        <form class="space-y-5" @submit.prevent="saveSettings">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Settings
            </p>
            <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Template configuration
            </h3>
          </div>

          <div class="space-y-6">
            <div class="space-y-4">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                GitHub App (template creation only)
              </p>
              <p class="text-xs text-[color:var(--muted)]">
                Required only when creating new repos from templates. Deploys and forwarding work without it.
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

          </div>

          <UiInlineFeedback v-if="error" tone="error">
            {{ error }}
          </UiInlineFeedback>

          <UiInlineFeedback v-if="success" tone="ok">
            {{ success }}
          </UiInlineFeedback>

          <div class="flex flex-wrap items-center gap-3">
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
