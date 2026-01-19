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

const toastStore = useToastStore()
const route = useRoute()
const pageLoading = usePageLoadingStore()
const containers = ref<DockerContainer[]>([])
const loading = ref(false)
const error = ref('')
const selectedContainer = ref('')
const streamError = ref('')
const streamState = ref<'idle' | 'connecting' | 'live' | 'paused' | 'error'>('idle')
const logLines = ref<string[]>([])
const tailInput = ref('200')
const filterQuery = ref('')
const containerFilter = ref('')
const followLive = ref(true)
const showTimestamps = ref(true)
const logViewport = ref<HTMLElement | null>(null)
const routeApplied = ref(false)

let streamSource: EventSource | null = null

const isRunningStatus = (status: string) => {
  const normalized = status.toLowerCase()
  return normalized.startsWith('up') || normalized.includes('running')
}

const selectedInfo = computed(() =>
  containers.value.find((container) => container.name === selectedContainer.value),
)
const routeContainer = computed(() =>
  typeof route.query.container === 'string' ? route.query.container : '',
)

const filteredContainers = computed(() => {
  const needle = containerFilter.value.trim().toLowerCase()
  if (!needle) return containers.value
  return containers.value.filter((container) => {
    const haystack = [
      container.name,
      container.image,
      container.service,
      container.project,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    return haystack.includes(needle)
  })
})

const runningCount = computed(() =>
  containers.value.filter((container) => isRunningStatus(container.status)).length,
)

const containerOptions = computed(() =>
  filteredContainers.value.map((container) => ({
    value: container.name,
    label: container.name,
  })),
)

const filteredLines = computed(() => {
  const needle = filterQuery.value.trim().toLowerCase()
  if (!needle) return logLines.value
  return logLines.value.filter((line) => line.toLowerCase().includes(needle))
})

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

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
const tailValue = computed(() => resolveTail())
const hasLogs = computed(() => filteredLines.value.length > 0)

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

watch([selectedContainer, followLive, showTimestamps], () => {
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
          Pick a container and tail its output while you deploy or troubleshoot.
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

    <div class="gap-6 flex flex-col">
      <UiPanel class="space-y-4 p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Containers
            </p>
            <p class="text-sm text-[color:var(--muted)]">
              {{ filteredContainers.length }} shown · {{ runningCount }} running ·
              {{ containers.length }} total
            </p>
          </div>
        </div>

        <UiState v-if="loading" loading>
          Loading container list...
        </UiState>

        <UiState v-else-if="containers.length === 0">
          <p class="text-base font-semibold text-[color:var(--text)]">No containers yet</p>
          <p class="mt-2">Start a deployment or Docker compose stack to see logs here.</p>
        </UiState>

        <div v-else class="space-y-3">
          <div class="flex flex-row gap-2 flex-warp">
          <div class="w-10/12">
          <UiInput
            v-model="containerFilter"
            placeholder="Filter containers"
          />
          </div>
          <div class="w-2/12">
          <UiSelect
            v-model="selectedContainer"
            placeholder="Select a container"
            :options="containerOptions"
          />

          </div>
          </div>
          <div class="space-y-2 flex flex-row flex-wrap">
            <div
              class="w-1/6 cursor-pointer flex flex-col gap-2 p-4"
              v-for="container in filteredContainers"
              :key="container.id"
              :class="selectedContainer === container.name
                ? 'border border-[color:var(--accent)]/40 bg-[color:var(--surface-2)]'
                : ''"
              @click="selectedContainer = container.name"
            >
              <div class="flex items-center justify-between gap-3">
                <p class="text-sm font-semibold text-[color:var(--text)]">
                  {{ container.name }}
                </p>
                <UiBadge tone="neutral">{{ container.status }}</UiBadge>
              </div>
              <p class="text-xs text-[color:var(--muted)]">
                {{ container.image }}
              </p>
              <p class="text-xs text-[color:var(--muted-2)]">
                {{ container.ports || 'No published ports' }}
              </p>
            </div>
          </div>
        </div>
      </UiPanel>

      <UiPanel class="flex h-full flex-col gap-4 p-4">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Live output
          </p>
          <h2 class="mt-1 text-lg font-semibold text-[color:var(--text)]">
            {{ selectedInfo?.name || 'Select a container' }}
          </h2>
          <p class="text-xs text-[color:var(--muted)]">
            {{ selectedInfo?.service || 'No compose service detected' }}
          </p>
        </div>
        <div class="flex flex-row gap-2">
          <div class="w-3/12 flex flex-col gap-4">       
          <div>
            <p class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Image</p>
            <p class="mt-1 text-sm text-[color:var(--text)]">{{ selectedInfo?.image }}</p>
          </div>
          <div class="flex flex-row justify-between">
          <div class="flex flex-row gap-4 items-center">
            <div>
            <p class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Runtime
            </p>
            <p class=" text-[color:var(--muted-2)]">{{ selectedInfo?.status }}</p>
                        </div>

            <p class="text-xs text-[color:var(--text)]">
              ({{ selectedInfo?.runningFor || 'Unknown' }})
            </p>
          </div>
          <div>
            <p class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Ports</p>
            <p class="text-sm text-[color:var(--text)]">
              {{ selectedInfo?.ports || 'No published ports' }}
            </p>
          </div>
          </div>
          </div>
      <div class="w-9/12">
<UiPanel
  variant="soft"
  class="flex flex-col gap-3 p-3 text-xs text-[color:var(--muted)]"
>
  <!-- Row 1: Stream status -->
  <div class="flex flex-row items-center gap-2 w-full">
    <UiBadge :tone="streamBadge.tone">
      {{ streamBadge.label }}
    </UiBadge>

    <span class="text-[11px] uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
      Stream
    </span>

    <span class="text-[color:var(--muted-2)]">·</span>

    <span>
      {{ filteredLines.length }} lines · tail {{ tailValue }}
    </span>

    <span
      v-if="streamState === 'connecting'"
      class="flex flex-row items-center gap-2"
    >
      <UiInlineSpinner />
      Connecting...
    </span>
  </div>

  <!-- Row 2: Controls -->
  <div class="flex flex-col items-center gap-3 w-full">
    <!-- Left: playback + tail -->
    <div class="flex flex-row items-center justify-around gap-2 w-full">
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
      <UiToggle v-model="followLive">Follow live</UiToggle>
      <UiToggle v-model="showTimestamps">Timestamps</UiToggle>

      <label class="flex flex-row gap-2 items-center">
        <span class="text-nowrap">Tail</span>
        <UiInput
          v-model="tailInput"
          type="number"
          min="1"
          max="5000"
        />
      </label>
    </div>

    <!-- Middle: toggles + filter -->
    <div class="flex flex-row items-center gap-3 w-full">

      <UiInput
        v-model="filterQuery"
        placeholder="Filter log lines"
        class="w-full"
      />
    </div>

    <!-- Right: actions -->
    <div class="flex flex-row items-center justify-end gap-2 w-full">

    </div>
  </div>
</UiPanel>

</div>
        </div>


        <UiState v-if="!hasSelection" class="flex-1">
          Choose a running container to start streaming logs.
        </UiState>

        <UiState v-else-if="streamError" tone="error" class="flex-1">
          {{ streamError }}
        </UiState>

        <UiPanel
          v-else
          variant="raise"
          class="flex-1 overflow-hidden"
        >
          <div
            ref="logViewport"
            class="max-h-[60vh] overflow-auto px-4 py-3 font-mono text-xs leading-relaxed text-[color:var(--text)]"
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
