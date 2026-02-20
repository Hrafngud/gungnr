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
import { hostApi } from '@/services/host'
import { getApiBaseUrl } from '@/services/api'
import { useToastStore } from '@/stores/toasts'
import { usePageLoadingStore } from '@/stores/pageLoading'
import type { DockerContainer } from '@/types/host'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'
type StreamState = 'idle' | 'connecting' | 'live' | 'paused' | 'error'
type ContainerState = 'running' | 'starting' | 'stopped' | 'other'
type ContainerType = 'web' | 'api' | 'database' | 'proxy' | 'worker' | 'other'
type ContainerTypeFilter = 'all' | ContainerType
type ContainerStatusFilter = 'all' | ContainerState

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
const selectedContainer = ref('')
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

let streamSource: EventSource | null = null

const normalize = (value: string | null | undefined) => (value ?? '').trim().toLowerCase()

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
  containers.value.find((container) => container.name === selectedContainer.value),
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
      if (!textNeedle) {
        return true
      }
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

const placeholderCount = computed(() => Math.max(0, PAGE_SIZE - pagedContainers.value.length))

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

const hasSelection = computed(() => selectedContainer.value !== '')
const hasLogs = computed(() => filteredLines.value.length > 0)
const logFontSizes = [11, 12, 13, 14, 15, 16] as const
const logFontSize = ref<number>(12)
const maxLogFontSize = logFontSizes[logFontSizes.length - 1] ?? 16
const canDecreaseLogFont = computed(() => logFontSize.value > logFontSizes[0])
const canIncreaseLogFont = computed(() => logFontSize.value < maxLogFontSize)

const loadContainers = async () => {
  loading.value = true
  error.value = ''
  try {
    const { data } = await hostApi.listDocker()
    containers.value = data.containers
    applyRouteSelection()
    if (!selectedContainer.value && data.containers.length > 0) {
      const firstRunning = data.containers.find((container) => isRunningStatus(container.status))
      const first = firstRunning ?? data.containers[0]
      if (first) {
        selectedContainer.value = first.name
      }
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load containers.'
  } finally {
    loading.value = false
  }
}

const resolveTail = () => {
  const parsed = Number.parseInt(tailInput.value, 10)
  if (Number.isNaN(parsed) || parsed <= 0) return 200
  return Math.min(parsed, 5000)
}

const applyRouteSelection = () => {
  if (!routeContainer.value || routeApplied.value) return
  const match = containers.value.find((container) => container.name === routeContainer.value)
  if (match) {
    selectedContainer.value = match.name
    routeApplied.value = true
  }
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
  if (streamSource) {
    streamSource.close()
    streamSource = null
  }
}

const startStream = () => {
  if (!hasSelection.value) return

  closeStream()
  streamError.value = ''
  streamState.value = 'connecting'

  const params = new URLSearchParams({
    container: selectedContainer.value,
    tail: resolveTail().toString(),
    follow: followLive.value ? 'true' : 'false',
    timestamps: showTimestamps.value ? 'true' : 'false',
  })

  const baseUrl = getApiBaseUrl()
  const url = `${baseUrl}/api/v1/host/docker/logs?${params.toString()}`
  streamSource = new EventSource(url, { withCredentials: true })

  streamSource.onopen = () => {
    streamState.value = 'live'
  }

  streamSource.addEventListener('log', (event) => {
    const message = event as MessageEvent
    try {
      const payload = JSON.parse(message.data) as { line: string }
      if (payload.line) {
        logLines.value.push(payload.line)
        if (logLines.value.length > 2000) {
          logLines.value.splice(0, logLines.value.length - 2000)
        }
      }
    } catch {
      // Ignore malformed log events.
    }
  })

  streamSource.addEventListener('done', () => {
    streamState.value = 'idle'
    closeStream()
  })

  streamSource.addEventListener('error', (event) => {
    const message = event as MessageEvent
    if (message?.data) {
      try {
        const payload = JSON.parse(message.data) as { message?: string }
        streamError.value = payload.message || 'Log stream error.'
      } catch {
        streamError.value = 'Log stream error.'
      }
      streamState.value = 'error'
    } else if (streamSource?.readyState === EventSource.CLOSED) {
      streamState.value = 'idle'
    }
  })
}

const pauseStream = () => {
  closeStream()
  streamState.value = 'paused'
}

const resumeStream = () => {
  if (!hasSelection.value) return
  startStream()
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
  if (next) {
    logFontSize.value = next
  }
}

const decreaseLogFontSize = () => {
  const currentIndex = logFontSizes.findIndex((size) => size === logFontSize.value)
  if (currentIndex === -1) {
    logFontSize.value = 12
    return
  }
  const next = logFontSizes[currentIndex - 1]
  if (next) {
    logFontSize.value = next
  }
}

const clearGridFilters = () => {
  containerFilter.value = ''
  containerTypeFilter.value = 'all'
  containerStatusFilter.value = 'all'
  projectFilter.value = 'all'
  gridPage.value = 1
}

watch([containerFilter, containerTypeFilter, containerStatusFilter, projectFilter], () => {
  gridPage.value = 1
})

watch(totalPages, (pages) => {
  if (gridPage.value > pages) {
    gridPage.value = pages
  }
})

watch(filteredContainers, (next) => {
  if (next.length === 0) {
    closeStream()
    streamState.value = 'idle'
    selectedContainer.value = ''
    clearLogs()
    return
  }
  if (!next.some((container) => container.name === selectedContainer.value)) {
    selectedContainer.value = next[0]?.name ?? ''
  }
})

watch(selectedContainer, () => {
  if (!hasSelection.value) return
  clearLogs()
  if (streamState.value === 'paused') {
    closeStream()
    return
  }
  startStream()
})

watch([followLive, showTimestamps], () => {
  if (streamState.value === 'paused') return
  if (hasSelection.value) {
    startStream()
  }
})

watch(tailInput, () => {
  if (streamState.value === 'paused') return
  if (hasSelection.value) {
    startStream()
  }
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
  if (hasSelection.value) {
    startStream()
  }
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
              v-model="selectedContainer"
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

          <div class="logs-grid-shell">
            <div class="logs-grid">
              <button
                v-for="container in pagedContainers"
                :key="container.id"
                type="button"
                class="logs-grid-card"
                :class="selectedContainer === container.name ? 'logs-grid-card-selected' : ''"
                @click="selectedContainer = container.name"
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

              <div
                v-for="slot in placeholderCount"
                :key="`placeholder-${slot}`"
                class="logs-grid-placeholder"
                aria-hidden="true"
              />
            </div>
          </div>
        </div>
      </UiPanel>

      <UiPanel class="flex h-full flex-col gap-4 p-4">
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

            <UiInput
              v-model="filterQuery"
              placeholder="Filter log lines"
              class="logs-stream-filter"
            />
          </div>
        </div>

        <UiState v-if="!hasSelection" class="flex-1">
          Choose a container to start streaming logs.
        </UiState>

        <UiState v-else-if="streamError" tone="error" class="flex-1">
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
    </div>
  </section>
</template>

<style scoped>
.logs-grid-shell {
  overflow-x: auto;
}

.logs-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  grid-template-rows: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
  min-width: 760px;
}

.logs-grid-card,
.logs-grid-placeholder {
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
  transition: background-color 160ms ease, transform 160ms ease, box-shadow 160ms ease;
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

.logs-grid-placeholder {
  opacity: 0.35;
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

.logs-stream-settings {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  white-space: nowrap;
}

.logs-stream-actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  white-space: nowrap;
}

.logs-stream-filter {
  width: min(26rem, 38vw);
  min-width: 11rem;
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
}
</style>
