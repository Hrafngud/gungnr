<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import NavIcon from '@/components/NavIcon.vue'
import { getApiBaseUrl } from '@/services/api'
import { hostApi } from '@/services/host'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import type { DockerContainer } from '@/types/host'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'
type StreamState = 'idle' | 'connecting' | 'live' | 'paused' | 'error'
type ContainerState = 'running' | 'starting' | 'stopped' | 'other'
type ContainerType = 'web' | 'api' | 'database' | 'proxy' | 'worker' | 'other'
type ContainerTypeFilter = 'all' | ContainerType
type ContainerStatusFilter = 'all' | ContainerState
type LayoutState = 'grid-only' | 'grid-with-logs'

type UiMachineState = {
  layout: LayoutState
  selectedContainer: string
}

type UiEvent
  = | { type: 'select-container'; name: string }
    | { type: 'close-logs' }
    | { type: 'sync-available'; names: string[] }
    | { type: 'route-select'; name: string }

const GRID_COLUMNS = 4
const GRID_ROWS = 3
const PAGE_SIZE = GRID_COLUMNS * GRID_ROWS
const MAX_GRID_PAGE_BUTTONS = 7

const TYPE_LABELS: Record<ContainerType, string> = {
  web: 'Web',
  api: 'API',
  database: 'Database',
  proxy: 'Proxy',
  worker: 'Worker',
  other: 'Other',
}

const toastStore = useToastStore()
const route = useRoute()
const pageLoading = usePageLoadingStore()

const containers = ref<DockerContainer[]>([])
const loading = ref(false)
const error = ref('')
const streamError = ref('')
const streamState = ref<StreamState>('idle')
const logLines = ref<string[]>([])
const tailInput = ref('200')
const filterQuery = ref('')
const containerFilter = ref('')
const containerTypeFilter = ref<ContainerTypeFilter>('all')
const containerStatusFilter = ref<ContainerStatusFilter>('all')
const projectFilter = ref('all')
const gridPage = ref(1)
const followLive = ref(true)
const showTimestamps = ref(true)
const logViewport = ref<HTMLElement | null>(null)
const routeApplied = ref(false)
const uiMachine = ref<UiMachineState>({
  layout: 'grid-only',
  selectedContainer: '',
})

let streamSource: EventSource | null = null

const normalize = (value: string | null | undefined) => (value ?? '').trim().toLowerCase()

const reduceUiMachine = (state: UiMachineState, event: UiEvent): UiMachineState => {
  switch (event.type) {
    case 'select-container': {
      const name = event.name.trim()
      if (!name) return { layout: 'grid-only', selectedContainer: '' }
      return { layout: 'grid-with-logs', selectedContainer: name }
    }
    case 'route-select': {
      const name = event.name.trim()
      if (!name) return state
      return { layout: 'grid-with-logs', selectedContainer: name }
    }
    case 'close-logs': {
      if (!state.selectedContainer) return { layout: 'grid-only', selectedContainer: '' }
      return { layout: 'grid-only', selectedContainer: state.selectedContainer }
    }
    case 'sync-available': {
      if (event.names.length === 0) {
        return { layout: 'grid-only', selectedContainer: '' }
      }
      if (state.selectedContainer && event.names.includes(state.selectedContainer)) {
        return state
      }
      if (state.layout === 'grid-with-logs') {
        const fallback = event.names[0] ?? ''
        if (fallback) {
          return { layout: 'grid-with-logs', selectedContainer: fallback }
        }
      }
      return { layout: 'grid-only', selectedContainer: '' }
    }
    default:
      return state
  }
}

const dispatchUiEvent = (event: UiEvent) => {
  const next = reduceUiMachine(uiMachine.value, event)
  if (
    next.layout === uiMachine.value.layout
    && next.selectedContainer === uiMachine.value.selectedContainer
  ) {
    return
  }
  uiMachine.value = next
}

const selectedContainerName = computed(() => uiMachine.value.selectedContainer)

const selectedContainerModel = computed({
  get: () => uiMachine.value.selectedContainer,
  set: (name: string) => {
    dispatchUiEvent({ type: 'select-container', name })
  },
})

const hasSelection = computed(() => selectedContainerName.value !== '')
const isLogsVisible = computed(
  () => uiMachine.value.layout === 'grid-with-logs' && hasSelection.value,
)

const isRunningStatus = (status: string) => {
  const normalized = normalize(status)
  return normalized.startsWith('up') || normalized.includes('running')
}

const isStoppedStatus = (status: string) => {
  const normalized = normalize(status)
  return normalized.startsWith('exited') || normalized.includes('dead') || normalized.includes('created')
}

const isStartingStatus = (status: string) => {
  const normalized = normalize(status)
  return normalized.includes('restart') || normalized.includes('start') || normalized.includes('pause')
}

const resolveContainerState = (status: string): ContainerState => {
  if (isRunningStatus(status)) return 'running'
  if (isStartingStatus(status)) return 'starting'
  if (isStoppedStatus(status)) return 'stopped'
  return 'other'
}

const containerStateLabel = (status: string) => {
  const state = resolveContainerState(status)
  if (state === 'running') return 'Running'
  if (state === 'starting') return 'Starting'
  if (state === 'stopped') return 'Stopped'
  return 'Other'
}

const statusDotClass = (status: string) => {
  const state = resolveContainerState(status)
  if (state === 'running') return 'status-dot status-dot-ok'
  if (state === 'starting') return 'status-dot status-dot-warn'
  if (state === 'stopped') return 'status-dot status-dot-error'
  return 'status-dot status-dot-neutral'
}

const inferContainerType = (container: DockerContainer): ContainerType => {
  const haystack = normalize(
    [container.name, container.image, container.service, container.project].filter(Boolean).join(' '),
  )
  if (/postgres|mysql|mariadb|mongo|redis|duckdb|database|\bdb\b/.test(haystack)) return 'database'
  if (/nginx|proxy|traefik|haproxy|caddy|gateway|cloudflared/.test(haystack)) return 'proxy'
  if (/worker|queue|scheduler|cron|beat/.test(haystack)) return 'worker'
  if (/api|backend|server/.test(haystack)) return 'api'
  if (/web|frontend|ui|client/.test(haystack)) return 'web'
  return 'other'
}

const containerTypeLabel = (container: DockerContainer) => TYPE_LABELS[inferContainerType(container)]

const selectedInfo = computed(() =>
  containers.value.find((container) => container.name === selectedContainerName.value),
)

const routeContainer = computed(() =>
  typeof route.query.container === 'string' ? route.query.container : '',
)

const typeCounts = computed<Record<ContainerType, number>>(() => {
  const counts: Record<ContainerType, number> = {
    web: 0,
    api: 0,
    database: 0,
    proxy: 0,
    worker: 0,
    other: 0,
  }
  containers.value.forEach((container) => {
    counts[inferContainerType(container)] += 1
  })
  return counts
})

const statusCounts = computed<Record<ContainerState, number>>(() => {
  const counts: Record<ContainerState, number> = {
    running: 0,
    starting: 0,
    stopped: 0,
    other: 0,
  }
  containers.value.forEach((container) => {
    counts[resolveContainerState(container.status)] += 1
  })
  return counts
})

const typeOptions = computed(() => {
  const counts = typeCounts.value
  return [
    { value: 'all', label: `All types (${containers.value.length})` },
    { value: 'web', label: `Web (${counts.web})` },
    { value: 'api', label: `API (${counts.api})` },
    { value: 'database', label: `Database (${counts.database})` },
    { value: 'proxy', label: `Proxy (${counts.proxy})` },
    { value: 'worker', label: `Worker (${counts.worker})` },
    { value: 'other', label: `Other (${counts.other})` },
  ]
})

const statusOptions = computed(() => {
  const counts = statusCounts.value
  return [
    { value: 'all', label: `All status (${containers.value.length})` },
    { value: 'running', label: `Running (${counts.running})` },
    { value: 'starting', label: `Starting (${counts.starting})` },
    { value: 'stopped', label: `Stopped (${counts.stopped})` },
    { value: 'other', label: `Other (${counts.other})` },
  ]
})

const projectOptions = computed(() => {
  const counts = new Map<string, number>()
  containers.value.forEach((container) => {
    const projectName = container.project?.trim()
    if (!projectName) return
    counts.set(projectName, (counts.get(projectName) ?? 0) + 1)
  })
  const dynamic = Array.from(counts.entries())
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([projectName, count]) => ({
      value: projectName,
      label: `${projectName} (${count})`,
    }))
  return [{ value: 'all', label: 'All projects' }, ...dynamic]
})

const filteredContainers = computed(() => {
  const textNeedle = normalize(containerFilter.value)
  const typeNeedle = containerTypeFilter.value
  const statusNeedle = containerStatusFilter.value
  const projectNeedle = projectFilter.value
  return containers.value
    .filter((container) => {
      if (typeNeedle !== 'all' && inferContainerType(container) !== typeNeedle) {
        return false
      }
      if (statusNeedle !== 'all' && resolveContainerState(container.status) !== statusNeedle) {
        return false
      }
      if (projectNeedle !== 'all' && container.project !== projectNeedle) {
        return false
      }
      if (!textNeedle) return true
      const haystack = normalize(
        [
          container.name,
          container.image,
          container.service,
          container.project,
          containerTypeLabel(container),
          containerStateLabel(container.status),
        ]
          .filter(Boolean)
          .join(' '),
      )
      return haystack.includes(textNeedle)
    })
    .sort((a, b) => a.name.localeCompare(b.name))
})

const containerOptions = computed(() =>
  filteredContainers.value.map((container) => ({
    value: container.name,
    label: container.name,
  })),
)

const totalPages = computed(() =>
  Math.max(1, Math.ceil(filteredContainers.value.length / PAGE_SIZE)),
)

const pagedContainers = computed(() => {
  const start = (gridPage.value - 1) * PAGE_SIZE
  return filteredContainers.value.slice(start, start + PAGE_SIZE)
})

const baseGridRows = computed(() => {
  if (pagedContainers.value.length === 0) return 0
  return Math.min(GRID_ROWS, Math.max(1, Math.ceil(pagedContainers.value.length / GRID_COLUMNS)))
})

const isGridCondensed = computed(() => isLogsVisible.value && pagedContainers.value.length > 0)
const effectiveGridRows = computed(() => (isGridCondensed.value ? 1 : baseGridRows.value))

const pageSummary = computed(() => {
  if (filteredContainers.value.length === 0) return '0-0 of 0'
  const from = (gridPage.value - 1) * PAGE_SIZE + 1
  const to = from + pagedContainers.value.length - 1
  return `${from}-${to} of ${filteredContainers.value.length}`
})

const pageButtons = computed(() => {
  const total = totalPages.value
  if (total <= MAX_GRID_PAGE_BUTTONS) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const half = Math.floor(MAX_GRID_PAGE_BUTTONS / 2)
  let start = Math.max(1, gridPage.value - half)
  const end = Math.min(total, start + MAX_GRID_PAGE_BUTTONS - 1)
  start = Math.max(1, end - MAX_GRID_PAGE_BUTTONS + 1)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

const hasActiveFilters = computed(
  () =>
    containerFilter.value.trim().length > 0
    || containerTypeFilter.value !== 'all'
    || containerStatusFilter.value !== 'all'
    || projectFilter.value !== 'all',
)

const filteredLines = computed(() => {
  const needle = normalize(filterQuery.value)
  if (!needle) return logLines.value
  return logLines.value.filter((line) => line.toLowerCase().includes(needle))
})

const streamBadge = computed<{ tone: BadgeTone; label: string }>(() => {
  switch (streamState.value) {
    case 'live':
      return { tone: 'ok', label: 'Live' }
    case 'connecting':
      return { tone: 'warn', label: 'Connecting' }
    case 'paused':
      return { tone: 'neutral', label: 'Paused' }
    case 'error':
      return { tone: 'error', label: 'Error' }
    default:
      return { tone: 'neutral', label: 'Idle' }
  }
})

const hasLogs = computed(() => filteredLines.value.length > 0)
const logFontSizes = [11, 12, 13, 14, 15, 16] as const
const logFontSize = ref<number>(12)
const maxLogFontSize = logFontSizes[logFontSizes.length - 1] ?? 16
const canDecreaseLogFont = computed(() => logFontSize.value > logFontSizes[0])
const canIncreaseLogFont = computed(() => logFontSize.value < maxLogFontSize)

const resolvedTail = computed(() => {
  const parsed = Number.parseInt(tailInput.value, 10)
  if (Number.isNaN(parsed) || parsed <= 0) return 200
  return Math.min(parsed, 5000)
})

const loadContainers = async () => {
  loading.value = true
  error.value = ''
  try {
    const { data } = await hostApi.listDocker()
    containers.value = data.containers
    applyRouteSelection()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load containers.'
  } finally {
    loading.value = false
  }
}

const applyRouteSelection = () => {
  if (!routeContainer.value || routeApplied.value) return
  const match = containers.value.find((container) => container.name === routeContainer.value)
  if (!match) return
  dispatchUiEvent({ type: 'route-select', name: match.name })
  routeApplied.value = true
}

const selectContainer = (name: string) => {
  dispatchUiEvent({ type: 'select-container', name })
}

const closeLogsPanel = () => {
  dispatchUiEvent({ type: 'close-logs' })
}

const clearLogs = () => {
  logLines.value = []
  streamError.value = ''
}

const copyLogs = async () => {
  if (!hasLogs.value) {
    toastStore.warn('No logs to copy yet.', 'Nothing to copy')
    return
  }
  const payload = filteredLines.value.join('\n')
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(payload)
    } else {
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
    toastStore.success('Logs copied to clipboard.', 'Copied')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const closeStream = () => {
  if (!streamSource) return
  streamSource.close()
  streamSource = null
}

const startStream = () => {
  if (!isLogsVisible.value || !hasSelection.value) return

  closeStream()
  streamError.value = ''
  streamState.value = 'connecting'

  const params = new URLSearchParams({
    container: selectedContainerName.value,
    tail: resolvedTail.value.toString(),
    follow: followLive.value ? 'true' : 'false',
    timestamps: showTimestamps.value ? 'true' : 'false',
  })

  const baseUrl = getApiBaseUrl()
  const url = `${baseUrl}/api/v1/host/docker/logs?${params.toString()}`
  const source = new EventSource(url, { withCredentials: true })
  streamSource = source

  source.onopen = () => {
    if (streamSource !== source) return
    streamState.value = 'live'
  }

  source.addEventListener('log', (event) => {
    if (streamSource !== source) return
    const message = event as MessageEvent
    try {
      const payload = JSON.parse(message.data) as { line: string }
      if (!payload.line) return
      logLines.value.push(payload.line)
      if (logLines.value.length > 2000) {
        logLines.value.splice(0, logLines.value.length - 2000)
      }
    } catch {
      // Ignore malformed log events.
    }
  })

  source.addEventListener('done', () => {
    if (streamSource !== source) return
    streamState.value = 'idle'
    closeStream()
  })

  source.addEventListener('error', (event) => {
    if (streamSource !== source) return
    const message = event as MessageEvent
    if (message?.data) {
      try {
        const payload = JSON.parse(message.data) as { message?: string }
        streamError.value = payload.message || 'Log stream error.'
      } catch {
        streamError.value = 'Log stream error.'
      }
      streamState.value = 'error'
      return
    }
    if (streamSource?.readyState === EventSource.CLOSED) {
      streamState.value = 'idle'
    }
  })
}

const pauseStream = () => {
  if (!isLogsVisible.value || !hasSelection.value) return
  closeStream()
  streamState.value = 'paused'
}

const resumeStream = () => {
  if (!isLogsVisible.value || !hasSelection.value) return
  streamState.value = 'idle'
}

const scrollToBottom = async () => {
  if (!followLive.value) return
  await nextTick()
  if (!logViewport.value) return
  logViewport.value.scrollTop = logViewport.value.scrollHeight
}

const goToPage = (page: number) => {
  if (page < 1 || page > totalPages.value) return
  gridPage.value = page
}

const increaseLogFontSize = () => {
  const currentIndex = logFontSizes.findIndex((size) => size === logFontSize.value)
  if (currentIndex === -1) {
    logFontSize.value = 12
    return
  }
  const next = logFontSizes[currentIndex + 1]
  if (next) logFontSize.value = next
}

const decreaseLogFontSize = () => {
  const currentIndex = logFontSizes.findIndex((size) => size === logFontSize.value)
  if (currentIndex === -1) {
    logFontSize.value = 12
    return
  }
  const next = logFontSizes[currentIndex - 1]
  if (next) logFontSize.value = next
}

const clearGridFilters = () => {
  containerFilter.value = ''
  containerTypeFilter.value = 'all'
  containerStatusFilter.value = 'all'
  projectFilter.value = 'all'
  gridPage.value = 1
}

const streamConfigKey = computed(() => {
  if (!isLogsVisible.value || !hasSelection.value) return 'hidden'
  const runMode = streamState.value === 'paused' ? 'paused' : 'active'
  return [
    selectedContainerName.value,
    resolvedTail.value,
    followLive.value ? '1' : '0',
    showTimestamps.value ? '1' : '0',
    runMode,
  ].join('|')
})

watch([containerFilter, containerTypeFilter, containerStatusFilter, projectFilter], () => {
  gridPage.value = 1
})

watch(totalPages, (pages) => {
  if (gridPage.value > pages) {
    gridPage.value = pages
  }
})

watch(filteredContainers, (next) => {
  dispatchUiEvent({
    type: 'sync-available',
    names: next.map((container) => container.name),
  })
})

watch(selectedContainerName, (next, prev) => {
  if (next === prev) return
  clearLogs()
  if (streamState.value === 'paused') {
    streamState.value = 'idle'
  }
  if (!next) {
    streamState.value = 'idle'
    streamError.value = ''
  }
})

watch(streamConfigKey, (next) => {
  if (next === 'hidden') {
    closeStream()
    streamState.value = 'idle'
    streamError.value = ''
    return
  }
  if (next.endsWith('|paused')) {
    closeStream()
    return
  }
  startStream()
})

watch(logLines, () => {
  void scrollToBottom()
})

watch(routeContainer, () => {
  routeApplied.value = false
  applyRouteSelection()
})

watch(containers, () => {
  if (!loading.value) {
    applyRouteSelection()
  }
})

onMounted(async () => {
  pageLoading.start('Loading containers...')
  await loadContainers()
  pageLoading.stop()
})

onBeforeUnmount(() => {
  closeStream()
})
</script>

<template>
  <section class="page space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Container logs
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Live host log stream
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Filter by type and status, then stream logs from any container on the current page.
        </p>
      </div>
      <UiButton
        variant="ghost"
        size="sm"
        :disabled="loading"
        @click="loadContainers"
      >
        <span class="flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="loading" />
          Refresh containers
        </span>
      </UiButton>
    </div>

    <UiPanel
      variant="soft"
      class="flex flex-wrap items-center justify-between gap-3 p-4 text-xs text-[color:var(--muted)]"
    >
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Day-to-day guidance
        </p>
        <p class="mt-1 text-sm text-[color:var(--muted)]">
          Keep this open while deploys or routing changes run to confirm container health.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UiBadge :tone="streamBadge.tone">
          {{ streamBadge.label }}
        </UiBadge>
        <span class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Stream
        </span>
      </div>
    </UiPanel>

    <UiState v-if="error" tone="error">
      {{ error }}
    </UiState>

    <div class="flex flex-col gap-6">
      <UiPanel class="space-y-4 p-4">
        <div class="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Container grid
            </p>
            <p class="text-sm text-[color:var(--muted)]">
              {{ filteredContainers.length }} filtered · {{ containers.length }} total · page {{ gridPage }} / {{ totalPages }}
            </p>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            Showing {{ pageSummary }}
            <span v-if="effectiveGridRows > 0"> · {{ effectiveGridRows }}-row mode</span>
          </p>
        </div>

        <UiState v-if="loading" loading>
          Loading container list...
        </UiState>

        <UiState v-else-if="containers.length === 0">
          <p class="text-base font-semibold text-[color:var(--text)]">No containers yet</p>
          <p class="mt-2">Start a deployment or Docker compose stack to see logs here.</p>
        </UiState>

        <div v-else class="space-y-4">
          <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-12">
            <UiInput
              v-model="containerFilter"
              placeholder="Search name, image, service, project..."
              class="xl:col-span-4"
            />
            <UiSelect
              v-model="containerTypeFilter"
              placeholder="Container type"
              :options="typeOptions"
              class="xl:col-span-2"
            />
            <UiSelect
              v-model="containerStatusFilter"
              placeholder="Status"
              :options="statusOptions"
              class="xl:col-span-2"
            />
            <UiSelect
              v-model="projectFilter"
              placeholder="Project"
              :options="projectOptions"
              class="xl:col-span-2"
            />
            <UiSelect
              v-model="selectedContainerModel"
              placeholder="Selected container"
              :options="containerOptions"
              class="xl:col-span-2"
            />
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2 text-xs text-[color:var(--muted)]">
              <span class="inline-flex items-center gap-1 rounded-[8px] border border-[color:var(--border)] px-2 py-1">
                <span class="status-dot status-dot-ok" />
                {{ statusCounts.running }} running
              </span>
              <span class="inline-flex items-center gap-1 rounded-[8px] border border-[color:var(--border)] px-2 py-1">
                <span class="status-dot status-dot-warn" />
                {{ statusCounts.starting }} starting
              </span>
              <span class="inline-flex items-center gap-1 rounded-[8px] border border-[color:var(--border)] px-2 py-1">
                <span class="status-dot status-dot-error" />
                {{ statusCounts.stopped }} stopped
              </span>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <UiButton
                variant="ghost"
                size="xs"
                :disabled="!hasActiveFilters"
                @click="clearGridFilters"
              >
                Clear filters
              </UiButton>
              <UiButton
                variant="ghost"
                size="xs"
                :disabled="gridPage <= 1"
                @click="goToPage(gridPage - 1)"
              >
                Prev
              </UiButton>
              <UiButton
                v-for="page in pageButtons"
                :key="page"
                :variant="page === gridPage ? 'primary' : 'ghost'"
                size="xs"
                @click="goToPage(page)"
              >
                {{ page }}
              </UiButton>
              <UiButton
                variant="ghost"
                size="xs"
                :disabled="gridPage >= totalPages"
                @click="goToPage(gridPage + 1)"
              >
                Next
              </UiButton>
            </div>
          </div>

          <div
            class="logs-grid-shell"
            :class="{ 'logs-grid-shell-condensed': isGridCondensed }"
          >
            <TransitionGroup
              v-if="pagedContainers.length > 0"
              name="logs-grid-item"
              tag="div"
              class="logs-grid"
              :class="[
                isGridCondensed ? 'logs-grid-condensed' : `logs-grid-mode-${effectiveGridRows}`,
              ]"
            >
              <button
                v-for="(container, index) in pagedContainers"
                :key="container.id"
                type="button"
                class="logs-grid-card"
                :class="selectedContainerName === container.name ? 'logs-grid-card-selected' : ''"
                :style="`--grid-item-index: ${index}`"
                @click="selectContainer(container.name)"
              >
                <div class="flex items-start justify-between gap-2">
                  <p class="truncate text-sm font-semibold text-[color:var(--text)]">
                    {{ container.name }}
                  </p>
                  <span class="inline-flex items-center gap-1 text-[10px] text-[color:var(--muted)]">
                    <span :class="statusDotClass(container.status)" />
                    {{ containerStateLabel(container.status) }}
                  </span>
                </div>
                <p class="mt-1 truncate text-[11px] text-[color:var(--muted)]">
                  {{ containerTypeLabel(container) }} · {{ container.service || 'no-service' }}
                </p>
                <p class="mt-1 truncate text-[11px] text-[color:var(--muted)]">
                  {{ container.project || 'no-project' }}
                </p>
                <p class="mt-1 truncate text-[11px] text-[color:var(--muted-2)]">
                  {{ container.image }}
                </p>
              </button>
            </TransitionGroup>
            <UiState v-else class="logs-grid-empty-state">
              No containers match the current filters.
            </UiState>
          </div>
        </div>
      </UiPanel>

      <Transition name="logs-panel">
        <UiPanel
          v-if="isLogsVisible"
          class="flex h-full flex-col gap-4 p-4"
        >
          <div class="logs-toolbar text-xs text-[color:var(--muted)]">
            <div class="logs-toolbar-row logs-toolbar-row-top">
              <div class="logs-selected-summary">
                <span class="logs-selected-name">{{ selectedInfo?.name || 'Select a container' }}</span>
                <span class="logs-selected-meta">
                  {{ selectedInfo ? `${containerTypeLabel(selectedInfo)} · ${selectedInfo.service || 'No compose service'}` : 'Choose from grid' }}
                </span>
                <span class="logs-runtime-chip">
                  <span :class="statusDotClass(selectedInfo?.status || '')" />
                  {{ selectedInfo?.status || 'Unknown' }}
                </span>
                <span class="logs-runtime-chip">
                  Ports: {{ selectedInfo?.ports || 'No published ports' }}
                </span>
              </div>

              <div class="logs-stream-meta">
                <UiBadge :tone="streamBadge.tone">
                  {{ streamBadge.label }}
                </UiBadge>
                <span class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Stream
                </span>
                <span class="text-[color:var(--muted-2)]">·</span>
                <span class="text-[color:var(--muted)]">{{ filteredLines.length }} lines</span>
                <span class="text-[color:var(--muted-2)]">·</span>
                <span class="text-[color:var(--muted)]">Tail</span>
                <UiInput
                  v-model="tailInput"
                  type="number"
                  min="1"
                  max="5000"
                  class="w-[5.25rem]"
                />
                <UiToggle v-model="followLive">Follow live</UiToggle>
                <UiToggle v-model="showTimestamps">Timestamps</UiToggle>
                <span
                  v-if="streamState === 'connecting'"
                  class="inline-flex items-center gap-2 text-[color:var(--muted)]"
                >
                  <UiInlineSpinner />
                  Connecting...
                </span>
              </div>
            </div>

            <div class="logs-toolbar-row logs-toolbar-row-bottom">
              <div class="logs-stream-actions">
                <UiButton
                  variant="ghost"
                  size="xs"
                  :disabled="!hasSelection || streamState === 'paused'"
                  @click="pauseStream"
                >
                  Pause
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="xs"
                  :disabled="!hasSelection || streamState !== 'paused'"
                  @click="resumeStream"
                >
                  Resume
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="xs"
                  :disabled="!hasSelection"
                  @click="clearLogs"
                >
                  Clear
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="xs"
                  :disabled="!hasSelection || !hasLogs"
                  @click="copyLogs"
                >
                  Copy
                </UiButton>
                <div class="inline-flex items-center gap-1 rounded-[8px] bg-[color:var(--surface-2)] px-2 py-1">
                  <UiButton
                    variant="ghost"
                    size="xs"
                    :disabled="!canDecreaseLogFont"
                    @click="decreaseLogFontSize"
                  >
                    A-
                  </UiButton>
                  <span class="text-[11px] text-[color:var(--muted)]">Log {{ logFontSize }}px</span>
                  <UiButton
                    variant="ghost"
                    size="xs"
                    :disabled="!canIncreaseLogFont"
                    @click="increaseLogFontSize"
                  >
                    A+
                  </UiButton>
                </div>
              </div>

              <div class="logs-toolbar-right-edge">
                <UiInput
                  v-model="filterQuery"
                  placeholder="Filter log lines"
                  class="logs-stream-filter"
                />
                <UiButton
                  variant="ghost"
                  size="xs"
                  class="logs-close-button"
                  aria-label="Close logs panel"
                  @click="closeLogsPanel"
                >
                  X
                </UiButton>
              </div>
            </div>
          </div>

          <UiState v-if="streamError" tone="error" class="flex-1">
            {{ streamError }}
          </UiState>

          <UiPanel
            v-else
            variant="raise"
            class="logs-output-panel flex-1 overflow-hidden"
          >
            <div
              ref="logViewport"
              class="logs-output-viewport overflow-auto px-4 py-3 font-mono text-[color:var(--text)]"
              :style="{ fontSize: `${logFontSize}px`, lineHeight: '1.5' }"
            >
              <p v-if="filteredLines.length === 0" class="text-[color:var(--muted)]">
                No logs yet. Deploy a service or wait for new output.
              </p>
              <pre v-else class="whitespace-pre-wrap break-words">
{{ filteredLines.join('\n') }}
              </pre>
            </div>
          </UiPanel>
        </UiPanel>
      </Transition>
    </div>
  </section>
</template>

<style scoped>
.logs-grid-shell {
  overflow-x: auto;
  overflow-y: visible;
}

.logs-grid-shell-condensed {
  overflow-x: auto;
  overflow-y: visible;
  overscroll-behavior-x: contain;
  padding-bottom: 0.15rem;
}

.logs-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  align-content: start;
  gap: 0.5rem;
  min-width: 760px;
  transform-origin: top center;
  will-change: transform, opacity;
  transition:
    grid-template-rows 320ms cubic-bezier(0.25, 1, 0.5, 1),
    transform 320ms cubic-bezier(0.22, 1, 0.36, 1),
    opacity 320ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-grid-mode-1 {
  grid-template-rows: repeat(1, minmax(120px, auto));
}

.logs-grid-mode-2 {
  grid-template-rows: repeat(2, minmax(120px, auto));
}

.logs-grid-mode-3 {
  grid-template-rows: repeat(3, minmax(120px, auto));
}

.logs-grid-condensed {
  grid-template-columns: none;
  grid-template-rows: repeat(1, minmax(120px, auto));
  grid-auto-flow: column;
  grid-auto-columns: minmax(18rem, 1fr);
  min-width: max-content;
  transform: translateY(-2px);
}

.logs-grid-card {
  min-height: 120px;
  border-radius: 10px;
  border: 1px solid transparent;
  background: color-mix(in oklab, var(--surface-2) 90%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--border) 34%, transparent);
}

.logs-grid-card {
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  text-align: left;
  padding: 0.65rem;
  transition:
    background-color 160ms cubic-bezier(0.22, 1, 0.36, 1),
    transform 160ms cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 160ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-grid-card:hover {
  background: color-mix(in oklab, var(--surface-2) 74%, var(--accent));
  box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--accent) 28%, var(--border));
  transform: translateY(-1px);
}

.logs-grid-card-selected {
  background: color-mix(in oklab, var(--surface-2) 66%, var(--accent));
  box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--accent) 42%, var(--border));
}

.logs-grid-item-enter-active {
  transition:
    opacity 240ms cubic-bezier(0.25, 1, 0.5, 1),
    transform 240ms cubic-bezier(0.25, 1, 0.5, 1);
  transition-delay: calc(var(--grid-item-index, 0) * 36ms);
}

.logs-grid-item-leave-active {
  transition:
    opacity 180ms cubic-bezier(0.25, 1, 0.5, 1),
    transform 180ms cubic-bezier(0.25, 1, 0.5, 1);
}

.logs-grid-item-move {
  transition: transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-grid-item-enter-from,
.logs-grid-item-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

.logs-grid-empty-state {
  min-height: 7.5rem;
}

.logs-panel-enter-active {
  transition:
    opacity 280ms cubic-bezier(0.25, 1, 0.5, 1),
    transform 320ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-panel-leave-active {
  transition:
    opacity 190ms cubic-bezier(0.22, 1, 0.36, 1),
    transform 190ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-panel-enter-from,
.logs-panel-leave-to {
  opacity: 0;
  transform: translateY(10px) scale(0.995);
}

.status-dot {
  display: inline-flex;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
}

.status-dot-ok {
  background: #22c55e;
}

.status-dot-warn {
  background: #f59e0b;
}

.status-dot-error {
  background: #ef4444;
}

.status-dot-neutral {
  background: color-mix(in oklab, var(--muted) 75%, transparent);
}

.logs-runtime-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.6rem;
  border-radius: 8px;
  background: color-mix(in oklab, var(--surface-2) 88%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--border) 26%, transparent);
  font-size: 11px;
  color: var(--muted);
}

.logs-selected-summary {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
  white-space: nowrap;
}

.logs-selected-name {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}

.logs-selected-meta {
  color: var(--muted);
}

.logs-toolbar {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.6rem;
  border-radius: 10px;
  background: color-mix(in oklab, var(--surface-2) 90%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--border) 28%, transparent);
}

.logs-toolbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.55rem;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 0.15rem;
  white-space: nowrap;
}

.logs-stream-meta {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  white-space: nowrap;
}

.logs-stream-actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  white-space: nowrap;
}

.logs-toolbar-right-edge {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-left: auto;
}

.logs-stream-filter {
  width: min(26rem, 38vw);
  min-width: 11rem;
}

.logs-close-button {
  min-width: 2.2rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  transition:
    transform 160ms cubic-bezier(0.22, 1, 0.36, 1),
    border-color 160ms cubic-bezier(0.22, 1, 0.36, 1),
    color 160ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logs-close-button:hover {
  transform: translateY(-1px);
}

.logs-toolbar-row::-webkit-scrollbar {
  height: 6px;
}

.logs-toolbar-row::-webkit-scrollbar-thumb {
  background: color-mix(in oklab, var(--border) 60%, transparent);
  border-radius: 999px;
}

.logs-output-panel {
  min-height: 0;
}

.logs-output-viewport {
  min-height: 32rem;
  max-height: 78vh;
}

@media (max-width: 1023px) {
  .logs-selected-name {
    font-size: 0.95rem;
  }

  .logs-selected-meta {
    font-size: 11px;
  }

  .logs-stream-filter {
    width: min(16rem, 48vw);
  }

  .logs-grid-condensed {
    grid-auto-columns: minmax(15rem, 1fr);
  }
}

@media (prefers-reduced-motion: reduce) {
  .logs-grid,
  .logs-grid-card,
  .logs-panel-enter-active,
  .logs-panel-leave-active,
  .logs-grid-item-enter-active,
  .logs-grid-item-leave-active,
  .logs-grid-item-move {
    transition-duration: 0.01ms !important;
    transition-delay: 0ms !important;
  }

  .logs-grid-item-enter-from,
  .logs-grid-item-leave-to,
  .logs-panel-enter-from,
  .logs-panel-leave-to {
    opacity: 1;
    transform: none;
  }
}
</style>
