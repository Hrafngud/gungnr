<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiRuntimeLedMeter from '@/components/ui/UiRuntimeLedMeter.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import type { BadgeTone } from '@/components/workbench/projectDetailWorkbenchTypes'
import { apiErrorMessage } from '@/services/api'
import { hostApi } from '@/services/host'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import { clampPercent, formatBytes, formatPercent } from '@/utils/runtimeMetrics'
import type { HostRuntimeWorkloadUsage } from '@/types/host'
import type { ProjectContainer, ProjectDetailDiagnostic } from '@/types/projects'

type ContainerActionKind = 'stop' | 'restart' | 'remove'

type ContainerActionState = {
  stopping: boolean
  restarting: boolean
  removing: boolean
  error: string | null
}

const props = defineProps<{
  projectName: string
  projectRuntimeKey: string
  containers: ProjectContainer[]
  runtimeDiagnostics: ProjectDetailDiagnostic[]
  projectStatus: string
  isAdmin: boolean
  stackRestarting: boolean
  stackRestartError: string | null
}>()

const emit = defineEmits<{
  restartStack: []
  containerActionCompleted: []
}>()

const toastStore = useToastStore()
const actionStates = reactive<Record<string, ContainerActionState>>({})
const lifecycleActionModalOpen = ref(false)
const lifecycleActionTarget = ref<ProjectContainer | null>(null)
const lifecycleActionKind = ref<ContainerActionKind | null>(null)
const usageLoading = ref(false)
const usageError = ref<string | null>(null)
const projectUsage = ref<HostRuntimeWorkloadUsage | null>(null)
const projectUsageWarnings = ref<string[]>([])

function containerTone(container: ProjectContainer): BadgeTone {
  const normalized = container.status.trim().toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  if (normalized.includes('paused') || normalized.includes('restarting')) return 'warn'
  return 'neutral'
}

function projectStatusTone(status: string): BadgeTone {
  const normalized = status.trim().toLowerCase()
  if (!normalized) return 'neutral'
  if (normalized === 'running' || normalized === 'up' || normalized.includes('running')) return 'ok'
  if (normalized.includes('failed') || normalized.includes('error')) return 'error'
  if (normalized.includes('pending') || normalized.includes('building')) return 'warn'
  return 'neutral'
}

const isStoppedStatus = (status: string) => {
  const normalized = status.toLowerCase()
  return normalized.startsWith('exited') || normalized.includes('dead') || normalized.includes('created')
}

const actionStateFor = (container: ProjectContainer): ContainerActionState => {
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

const activeUsageProjectKey = computed(() => {
  const runtimeKey = props.projectRuntimeKey.trim().toLowerCase()
  if (runtimeKey) return runtimeKey
  const containerProject = props.containers
    .map((container) => container.project.trim().toLowerCase())
    .find((value) => value.length > 0)
  if (containerProject) return containerProject
  return props.projectName.trim().toLowerCase()
})

const usageContainerSignature = computed(() =>
  props.containers.map((container) => `${container.id}:${container.status}`).join('|'),
)

const runtimeDiagnosticsMessage = computed(() => {
  const messages = props.runtimeDiagnostics
    .map((diagnostic) => diagnostic.message.trim())
    .filter((message, index, items) => message.length > 0 && items.indexOf(message) === index)

  return messages.join(' · ')
})

const usageIndicators = computed(() => {
  const usage = projectUsage.value
  if (!usage) return []
  return [
    {
      key: 'cpu',
      label: 'CPU usage',
      value: formatPercent(usage.cpuUsedPercent),
      percent: clampPercent(usage.cpuUsedPercent),
      meta: 'Aggregated from docker stats for project containers.',
    },
    {
      key: 'memory',
      label: 'RAM usage',
      value: formatBytes(usage.memoryUsedBytes),
      percent: clampPercent(usage.memorySharePercent),
      meta: `${formatPercent(usage.memorySharePercent)} of host memory`,
    },
    {
      key: 'disk',
      label: 'Disk usage',
      value: formatBytes(usage.diskUsedBytes),
      percent: clampPercent(usage.diskSharePercent),
      meta: `${formatPercent(usage.diskSharePercent)} of host disk`,
    },
  ]
})

const resolveProjectUsage = (projectsByName: Record<string, HostRuntimeWorkloadUsage> | undefined) => {
  if (!projectsByName) return null
  const key = activeUsageProjectKey.value
  if (key && projectsByName[key]) return projectsByName[key]
  const fallbackKey = Object.keys(projectsByName).find(
    (projectKey) => projectKey.toLowerCase() === key,
  )
  if (fallbackKey) return projectsByName[fallbackKey] ?? null
  return null
}

const lifecycleActionText = computed(() => {
  const target = lifecycleActionTarget.value
  const kind = lifecycleActionKind.value
  if (!target || !kind) return ''
  if (kind === 'stop') return 'stop'
  if (kind === 'remove') return 'remove'
  return isStoppedStatus(target.status) ? 'start' : 'restart'
})

const lifecycleActionDescription = computed(() => {
  if (!lifecycleActionText.value) return ''
  return `This container belongs to a Project, directly interfering on it's lifecycle may lead to fail, are you sure you wanna ${lifecycleActionText.value} the container?`
})

const lifecycleActionError = computed(() => {
  const target = lifecycleActionTarget.value
  if (!target) return null
  return actionStateFor(target).error
})

const lifecycleActionLoading = computed(() => {
  const target = lifecycleActionTarget.value
  const kind = lifecycleActionKind.value
  if (!target || !kind) return false
  const state = actionStateFor(target)
  if (kind === 'stop') return state.stopping
  if (kind === 'restart') return state.restarting
  return state.removing
})

const lifecycleActionConfirmVariant = computed(() =>
  lifecycleActionKind.value === 'remove' ? 'danger' : 'primary',
)

const lifecycleActionConfirmLabel = computed(() => {
  if (lifecycleActionLoading.value) {
    if (lifecycleActionKind.value === 'stop') return 'Stopping...'
    if (lifecycleActionKind.value === 'remove') return 'Removing...'
    return isStoppedStatus(lifecycleActionTarget.value?.status ?? '') ? 'Starting...' : 'Restarting...'
  }
  if (lifecycleActionKind.value === 'stop') return 'Stop container'
  if (lifecycleActionKind.value === 'remove') return 'Remove container'
  return isStoppedStatus(lifecycleActionTarget.value?.status ?? '') ? 'Start container' : 'Restart container'
})

const openLifecycleActionModal = (container: ProjectContainer, action: ContainerActionKind) => {
  lifecycleActionTarget.value = container
  lifecycleActionKind.value = action
  lifecycleActionModalOpen.value = true
}

const stopContainer = (container: ProjectContainer) => {
  openLifecycleActionModal(container, 'stop')
}

const restartContainer = (container: ProjectContainer) => {
  openLifecycleActionModal(container, 'restart')
}

const removeContainer = (container: ProjectContainer) => {
  openLifecycleActionModal(container, 'remove')
}

const confirmLifecycleAction = async () => {
  const projectName = props.projectName.trim()
  const target = lifecycleActionTarget.value
  const action = lifecycleActionKind.value
  if (!projectName || !target || !action) return

  const state = actionStateFor(target)
  if (action === 'stop' && state.stopping) return
  if (action === 'restart' && state.restarting) return
  if (action === 'remove' && state.removing) return

  state.error = null
  if (action === 'stop') state.stopping = true
  if (action === 'restart') state.restarting = true
  if (action === 'remove') state.removing = true

  try {
    if (action === 'stop') {
      await projectsApi.stopContainer(projectName, target.name)
      toastStore.success('Container stopped.', 'Docker')
    } else if (action === 'restart') {
      await projectsApi.restartContainer(projectName, target.name)
      const started = isStoppedStatus(target.status)
      toastStore.success(started ? 'Container started.' : 'Container restarted.', 'Docker')
    } else {
      await projectsApi.removeContainer(projectName, target.name)
      toastStore.success('Container removed.', 'Docker')
    }

    lifecycleActionModalOpen.value = false
    emit('containerActionCompleted')
  } catch (err) {
    const message = apiErrorMessage(err)
    state.error = message
    const failureTitle =
      action === 'stop' ? 'Stop failed' : action === 'restart' ? 'Restart failed' : 'Remove failed'
    toastStore.error(message, failureTitle)
  } finally {
    if (action === 'stop') state.stopping = false
    if (action === 'restart') state.restarting = false
    if (action === 'remove') state.removing = false
  }
}

const loadProjectUsage = async () => {
  usageLoading.value = true
  usageError.value = null
  try {
    const { data } = await hostApi.runtimeStats()
    projectUsage.value = resolveProjectUsage(data.stats.projectsByName)
    projectUsageWarnings.value = (data.stats.warnings ?? []).slice(0, 2)
  } catch (err) {
    usageError.value = apiErrorMessage(err)
    projectUsage.value = null
    projectUsageWarnings.value = []
  } finally {
    usageLoading.value = false
  }
}

watch(lifecycleActionModalOpen, (open) => {
  if (open) return
  lifecycleActionTarget.value = null
  lifecycleActionKind.value = null
})

watch(activeUsageProjectKey, () => {
  void loadProjectUsage()
}, { immediate: true })

watch(usageContainerSignature, () => {
  void loadProjectUsage()
})
</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
          Runtime units ({{ containers.length }})
        </h2>
      </div>
      <UiPanel
        variant="soft"
        class="p-4"
      >
        <div class="flex flex-wrap items-center gap-2">
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="stackRestarting || !isAdmin"
            @click="emit('restartStack')"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="restart" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="stackRestarting" />
              {{ stackRestarting ? 'Restarting stack...' : 'Restart stack' }}
            </span>
          </UiButton>
          <UiBadge :tone="projectStatusTone(projectStatus)">
            {{ projectStatus || 'unknown' }}
          </UiBadge>
        </div>
      </UiPanel>
    </div>

    <UiPanel variant="soft" class="space-y-4 p-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Usage
          </p>
          <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
            Project footprint
          </h3>
          <p class="mt-1 text-xs text-[color:var(--muted)]">
            Project-specific CPU, RAM, and disk usage from the host runtime telemetry path.
          </p>
        </div>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="usageLoading"
          @click="loadProjectUsage"
        >
          <span class="inline-flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="usageLoading" />
            Refresh usage
          </span>
        </UiButton>
      </div>

      <UiState v-if="usageError" tone="error">
        {{ usageError }}
      </UiState>
      <UiState v-else-if="usageLoading" loading>
        Loading project usage...
      </UiState>
      <UiState v-else-if="!projectUsage">
        Usage metrics are not available for this project yet.
      </UiState>
      <template v-else>
        <UiPanel variant="soft" class="grid gap-3 p-3 sm:grid-cols-2 lg:grid-cols-3">
          <div class="space-y-1">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Containers
            </p>
            <p class="text-base font-semibold text-[color:var(--text)]">
              {{ projectUsage.runningContainers }}/{{ projectUsage.containers }} running
            </p>
          </div>
          <div class="space-y-1">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Runtime key
            </p>
            <p class="break-all text-xs text-[color:var(--muted)]">
              {{ activeUsageProjectKey || 'n/a' }}
            </p>
          </div>
          <div class="space-y-1">
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Source
            </p>
            <p class="text-xs text-[color:var(--muted)]">
              Host worker runtime stats
            </p>
          </div>
        </UiPanel>

        <div class="grid gap-3 xl:grid-cols-3">
          <article
            v-for="indicator in usageIndicators"
            :key="indicator.key"
            class="space-y-2 rounded-md border border-[color:var(--border-soft)] bg-[color:var(--surface-inset)]/55 p-3"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                {{ indicator.label }}
              </p>
              <p class="text-sm font-semibold text-[color:var(--text)]">
                {{ indicator.value }}
              </p>
            </div>
            <UiRuntimeLedMeter :label="indicator.label" :percent="indicator.percent" />
            <p class="text-xs text-[color:var(--muted)]">
              {{ indicator.meta }}
            </p>
          </article>
        </div>

        <UiInlineFeedback v-if="projectUsageWarnings.length > 0" tone="warn">
          {{ projectUsageWarnings.join(' · ') }}
        </UiInlineFeedback>
      </template>
    </UiPanel>

    <UiInlineFeedback v-if="props.stackRestartError" tone="error">
      {{ props.stackRestartError }}
    </UiInlineFeedback>
    <UiInlineFeedback v-if="runtimeDiagnosticsMessage" tone="warn">
      {{ runtimeDiagnosticsMessage }}
    </UiInlineFeedback>
    <UiState v-if="containers.length === 0">No containers currently match this compose project label.</UiState>
    <div v-else class="grid gap-4 xl:grid-cols-2">
      <UiListRow
        v-for="container in containers"
        :key="container.id"
        as="article"
        class="space-y-4"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              {{ container.service || 'Container' }}
            </p>
            <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">{{ container.name }}</h3>
            <p class="mt-1 font-mono text-[11px] text-[color:var(--muted-2)]">{{ container.id }}</p>
          </div>
          <UiBadge :tone="containerTone(container)">{{ container.status || 'unknown' }}</UiBadge>
        </div>
        <div class="space-y-2 text-xs text-[color:var(--muted)]">
          <div class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Image</span>
            <span class="text-[color:var(--text)] break-all">{{ container.image }}</span>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Ports</span>
            <span class="text-[color:var(--text)]">{{ container.ports || '—' }}</span>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Service</span>
            <span class="text-[color:var(--text)]">{{ container.service || '—' }}</span>
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
              @click="removeContainer(container)"
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

    <UiModal
      v-model="lifecycleActionModalOpen"
      title="Confirm container lifecycle action"
    >
      <div class="space-y-4">
        <p class="text-sm text-[color:var(--muted)]">
          {{ lifecycleActionDescription }}
        </p>
        <p class="font-mono text-xs text-[color:var(--text)] break-all">
          {{ lifecycleActionTarget?.name || '' }}
        </p>
        <UiInlineFeedback v-if="lifecycleActionError" tone="error">
          {{ lifecycleActionError }}
        </UiInlineFeedback>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <UiButton variant="ghost" size="sm" :disabled="lifecycleActionLoading" @click="lifecycleActionModalOpen = false">
            Cancel
          </UiButton>
          <UiButton
            :variant="lifecycleActionConfirmVariant"
            size="sm"
            :disabled="lifecycleActionLoading"
            @click="confirmLifecycleAction"
          >
            <span class="inline-flex items-center gap-2">
              <UiInlineSpinner v-if="lifecycleActionLoading" />
              {{ lifecycleActionConfirmLabel }}
            </span>
          </UiButton>
        </div>
      </template>
    </UiModal>
  </UiPanel>
</template>
