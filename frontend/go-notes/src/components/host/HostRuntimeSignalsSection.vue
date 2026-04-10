<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiRuntimeLedMeter from '@/components/ui/UiRuntimeLedMeter.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { hostApi } from '@/services/host'
import { apiErrorMessage, isMockEnabled } from '@/services/api'
import { clampPercent, formatBytes, formatPercent } from '@/utils/runtimeMetrics'
import type { HostRuntimeSnapshot, HostRuntimeStreamSample } from '@/types/host'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const runtimeSnapshot = ref<HostRuntimeSnapshot | null>(null)
const runtimeSnapshotLoading = ref(false)
const runtimeSnapshotError = ref<string | null>(null)
const runtimeStreamSample = ref<HostRuntimeStreamSample | null>(null)
const runtimeStreamState = ref<'idle' | 'connecting' | 'live' | 'error'>('idle')
const runtimeStreamError = ref<string | null>(null)

let runtimeStreamSource: EventSource | null = null

const formatClockSpeed = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return 'Speed unavailable'
  if (value >= 1000) return `${(value / 1000).toFixed(2)} GHz`
  return `${value.toFixed(value >= 100 ? 0 : 1)} MHz`
}

const formatMemorySpeed = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return 'Speed unavailable'
  return `${Math.round(value)} MT/s`
}

const runtimeIdentityCards = () => {
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
  ]
}

const runtimeLiveIndicators = () => {
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
}

const runtimeSnapshotIndicators = () => {
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
}

const runtimeSnapshotWarnings = () => {
  const warnings = runtimeSnapshot.value?.warnings ?? []
  return warnings.slice(0, 3)
}

const runtimeStreamWarnings = () => {
  const warnings = runtimeStreamSample.value?.warnings ?? []
  return warnings.slice(0, 3)
}

const runtimeStreamBadge = () => {
  const intervalMs = runtimeStreamSample.value?.intervalMs ?? 100
  switch (runtimeStreamState.value) {
    case 'live':
      return { tone: 'ok' as BadgeTone, label: `Live · ${intervalMs}ms` }
    case 'connecting':
      return { tone: 'warn' as BadgeTone, label: 'Connecting…' }
    case 'error':
      return { tone: 'error' as BadgeTone, label: 'Stream error' }
    default:
      return { tone: 'neutral' as BadgeTone, label: 'Stream idle' }
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
  if (mockStreamInterval) {
    clearInterval(mockStreamInterval)
    mockStreamInterval = null
  }
  if (!runtimeStreamSource) return
  runtimeStreamSource.close()
  runtimeStreamSource = null
}

let mockStreamInterval: ReturnType<typeof setInterval> | null = null

const startRuntimeSignalStream = () => {
  closeRuntimeSignalStream()
  runtimeStreamState.value = 'connecting'
  runtimeStreamError.value = null

  if (isMockEnabled()) {
    runtimeStreamState.value = 'live'
    mockStreamInterval = setInterval(() => {
      const baseSample: HostRuntimeStreamSample = {
        collectedAt: new Date().toISOString(),
        mode: 'mock',
        intervalMs: 500,
        host: {
          memoryUsedBytes: 16012345678 + Math.floor(Math.random() * 100000000),
          memoryUsedPercent: 46.6 + Math.random() * 0.5,
          memoryFreeBytes: 18347391290 - Math.floor(Math.random() * 100000000),
          memoryAvailableBytes: 18347391290 - Math.floor(Math.random() * 100000000),
        },
        panel: {
          cpuUsedPercent: 3.8 + Math.random() * 2,
          memoryUsedBytes: 754003200 + Math.floor(Math.random() * 5000000),
          memorySharePercent: 2.19 + Math.random() * 0.1,
        },
        projects: {
          cpuUsedPercent: 19.2 + Math.random() * 3,
          memoryUsedBytes: 2536890624 + Math.floor(Math.random() * 10000000),
          memorySharePercent: 7.38 + Math.random() * 0.2,
        },
        projectsByName: {
          'mock-service': {
            cpuUsedPercent: 19.2 + Math.random() * 3,
            memoryUsedBytes: 2536890624 + Math.floor(Math.random() * 10000000),
            memorySharePercent: 7.38 + Math.random() * 0.2,
          },
        },
        warnings: ['mock runtime data is synthetic'],
      }
      runtimeStreamSample.value = baseSample
    }, 500)
    return
  }

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

onMounted(() => {
  loadRuntimeSnapshot()
  startRuntimeSignalStream()
})

onBeforeUnmount(() => {
  closeRuntimeSignalStream()
})

defineExpose({
  loadRuntimeSnapshot,
})
</script>

<template>
  <UiPanel as="section" class="space-y-6 p-6">
    <div class="w-full flex flex-row items-center justify-between gap-3">
      <div class="w-full flex flex-col">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Host stats
        </p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
          Monitor
        </h2>
        <p class="mt-1 text-sm text-[color:var(--muted)]">
          Monitor real-time CPU, RAM, and disk usage for the host, Gungnr panel, and projects.
        </p>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <UiBadge :tone="runtimeStreamBadge().tone">
          {{ runtimeStreamBadge().label }}
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
      <div class="flex flex-col gap-6">
        <div class="grid grid-cols-1 md:grid-cols-3 items-center  gap-2">
          <UiPanel
            v-for="card in runtimeIdentityCards()"
            :key="card.key"
            variant="soft"
            class="w-full p-3"
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
                  Live Telemetry
                </p>
              </div>
              <UiBadge :tone="runtimeStreamBadge().tone">
                {{ runtimeStreamBadge().label }}
              </UiBadge>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3">
              <article
                v-for="indicator in runtimeLiveIndicators()"
                :key="indicator.key"
                class="w-full rounded-md border border-[color:var(--border-soft)] bg-[color:var(--surface)]/70 p-3"
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
            <UiInlineFeedback v-else-if="runtimeStreamWarnings().length > 0" tone="warn">
              {{ runtimeStreamWarnings().join(' · ') }}
            </UiInlineFeedback>
          </UiPanel>

          <UiPanel variant="soft" class="p-4">
            <div>
              <p class="text-xs uppercase mb-1 tracking-[0.24em] text-[color:var(--muted-2)]">
                Disk Usage
              </p>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <article
                v-for="indicator in runtimeSnapshotIndicators()"
                :key="indicator.key"
                class="space-y-2 rounded-md w-full border border-[color:var(--border-soft)] bg-[color:var(--surface)]/70 p-3"
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
            <UiInlineFeedback v-if="runtimeSnapshotWarnings().length > 0" tone="warn">
              {{ runtimeSnapshotWarnings().join(' · ') }}
            </UiInlineFeedback>
          </UiPanel>
        </div>
      </div>
    </template>

    <UiState v-else>
      Runtime snapshot not loaded yet.
    </UiState>
  </UiPanel>
</template>