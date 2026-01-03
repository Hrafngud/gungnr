<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { hostApi } from '@/services/host'
import { apiErrorMessage } from '@/services/api'
import { useToastStore } from '@/stores/toasts'
import { useOnboardingStore } from '@/stores/onboarding'
import type { CloudflaredPreview, Settings, SettingsSources } from '@/types/settings'
import type { OnboardingStep } from '@/types/onboarding'
import type { DockerContainer } from '@/types/host'
import type { DockerHealth, TunnelHealth } from '@/types/health'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const settingsForm = reactive<Settings>({
  baseDomain: '',
  githubToken: '',
  cloudflareToken: '',
  cloudflareAccountId: '',
  cloudflareZoneId: '',
  cloudflaredTunnel: '',
  cloudflaredConfigPath: '',
})

const settingsSources = ref<SettingsSources | null>(null)
const cloudflaredTunnelName = ref<string | null>(null)

const toastStore = useToastStore()
const onboardingStore = useOnboardingStore()

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
const onboardingOpen = ref(false)
const onboardingStep = ref(0)

const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)

const containers = ref<DockerContainer[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)

const settingsFormOpen = ref(false)
const previewPanelOpen = ref(true)

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

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'integrations',
    title: 'Verify host integrations',
    description: 'Confirm Docker and the host cloudflared service before enabling automation.',
    target: "[data-onboard='host-integrations']",
  },
  {
    id: 'tokens',
    title: 'Add API tokens',
    description: 'Save GitHub and Cloudflare tokens so template creation and DNS updates can run.',
    target: "[data-onboard='host-api-tokens']",
    links: [
      {
        label: 'GitHub tokens',
        href: 'https://github.com/settings/tokens',
      },
      {
        label: 'Cloudflare API tokens',
        href: 'https://dash.cloudflare.com/profile/api-tokens',
      },
    ],
  },
  {
    id: 'cloudflared-config',
    title: 'Point to cloudflared config',
    description: 'Use the active config.yml from the host service so Warp Panel can update ingress safely.',
    target: "[data-onboard='host-cloudflared-config']",
    links: [
      {
        label: 'Cloudflare tunnel guide',
        href: 'https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/tunnel-guide/',
      },
    ],
  },
]

const hasPreview = computed(() => Boolean(preview.value?.contents))

const statusTone = (status: string): BadgeTone => {
  const normalized = status.toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
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

const loadSettings = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await settingsApi.get()
    Object.assign(settingsForm, data.settings)
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
  } catch (err) {
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  if (saving.value) return
  saving.value = true
  error.value = null
  success.value = null
  try {
    const { data } = await settingsApi.update({ ...settingsForm })
    Object.assign(settingsForm, data.settings)
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
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

const startOnboarding = () => {
  onboardingStep.value = 0
  onboardingOpen.value = true
}

const markOnboardingComplete = () => {
  onboardingStore.updateState({ hostSettings: true })
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadPreview(), loadHealth(), loadContainers()])
  await onboardingStore.fetchState()
  if (!onboardingStore.state.hostSettings) {
    onboardingOpen.value = true
  }
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
        <UiButton variant="ghost" size="sm" @click="startOnboarding">
          View guide
        </UiButton>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="containersLoading"
          @click="loadContainers"
        >
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="containersLoading" />
            Refresh host data
          </span>
        </UiButton>
        <UiButton
          variant="primary"
          size="sm"
          @click="settingsFormOpen = true"
        >
          Edit settings
        </UiButton>
        <UiButton
          variant="ghost"
          size="sm"
          @click="previewPanelOpen = !previewPanelOpen"
        >
          {{ previewPanelOpen ? 'Hide ingress preview' : 'Show ingress preview' }}
        </UiButton>
      </div>
    </div>

    <UiInlineFeedback v-if="error" tone="error">
      {{ error }}
    </UiInlineFeedback>

    <UiInlineFeedback v-if="success" tone="ok">
      {{ success }}
    </UiInlineFeedback>

    <hr />

    <div class="grid gap-6 lg:grid-cols-[1.25fr,0.75fr]">
      <UiPanel as="section" class="space-y-6 p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Host integrations
            </p>
            <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
              Running containers
            </h2>
            <p class="mt-2 text-sm text-[color:var(--muted)]">
              Stop, restart, or remove containers without losing track of their ports.
            </p>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="containersLoading"
            @click="loadContainers"
          >
            <span class="flex items-center gap-2">
              <UiInlineSpinner v-if="containersLoading" />
              Refresh list
            </span>
          </UiButton>
        </div>

        <UiState v-if="containersError" tone="error">
          {{ containersError }}
        </UiState>

        <UiState v-else-if="containersLoading" loading>
          Loading Docker containers...
        </UiState>

        <UiState v-else-if="containers.length === 0">
          No running containers detected on the host.
        </UiState>

        <div v-else class="grid gap-4 lg:grid-cols-2">
          <UiListRow
            v-for="container in containers"
            :key="container.id"
            as="article"
            class="space-y-4"
          >
            <div class="flex items-start justify-between gap-3">
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
              <div class="flex items-center justify-between gap-2">
                <span>Ports</span>
                <span class="text-[color:var(--text)]">
                  {{ container.ports || '—' }}
                </span>
              </div>
              <div class="flex items-center justify-between gap-2">
                <span>Project</span>
                <span class="text-[color:var(--text)]">
                  {{ container.project || 'n/a' }}
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
                    <UiInlineSpinner v-if="actionStateFor(container).stopping" />
                    {{ actionStateFor(container).stopping ? 'Stopping...' : 'Stop' }}
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="actionStateFor(container).restarting"
                  @click="restartContainer(container)"
                >
                  <span class="flex items-center gap-2">
                    <UiInlineSpinner v-if="actionStateFor(container).restarting" />
                    {{ actionStateFor(container).restarting ? 'Restarting...' : 'Restart' }}
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  class="text-[color:var(--danger)]"
                  :disabled="actionStateFor(container).removing"
                  @click="openRemoveModal(container)"
                >
                  Remove
                </UiButton>
                <UiButton
                  :as="RouterLink"
                  :to="{ path: '/logs', query: { container: container.name } }"
                  variant="ghost"
                  size="sm"
                >
                  Logs
                </UiButton>
              </div>

              <UiInlineFeedback v-if="actionStateFor(container).error" tone="error">
                {{ actionStateFor(container).error }}
              </UiInlineFeedback>
            </UiPanel>
          </UiListRow>
        </div>
      </UiPanel>

      <div class="space-y-6">
        <Transition name="panel-slide">
          <UiPanel v-if="previewPanelOpen" variant="raise" class="space-y-4 p-6">
            <div class="flex items-center justify-between gap-2">
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
              class="max-h-80 overflow-auto rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)]/90 p-4 text-xs text-[color:var(--accent-ink)]"
            ><code>{{ preview?.contents }}</code></pre>

            <UiState v-else>
              Cloudflared config not loaded yet.
            </UiState>
          </UiPanel>
        </Transition>
      </div>
    </div>

    <UiOnboardingOverlay
      v-model="onboardingOpen"
      v-model:stepIndex="onboardingStep"
      :steps="onboardingSteps"
      @finish="markOnboardingComplete"
      @skip="markOnboardingComplete"
    />

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
              <UiInlineSpinner v-if="healthLoading" />
              Refresh status
            </span>
          </UiButton>
        </div>

        <UiState v-if="healthLoading" loading>
          Checking host integrations...
        </UiState>

        <div v-else class="grid gap-3 sm:grid-cols-2" data-onboard="host-integrations">
          <UiPanel variant="soft" class="space-y-2 p-3">
            <div class="flex items-center justify-between gap-2">
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
            <div class="flex items-center justify-between gap-2">
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
            />
          </label>

          <div class="grid gap-4" data-onboard="host-api-tokens">
            <label class="grid gap-2">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                GitHub token
              </span>
              <UiInput
                v-model="settingsForm.githubToken"
                type="password"
                placeholder="ghp_••••••"
                :disabled="loading"
              />
            </label>

            <label class="grid gap-2">
              <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Cloudflare API token
              </span>
              <UiInput
                v-model="settingsForm.cloudflareToken"
                type="password"
                placeholder="cf_••••••"
                :disabled="loading"
              />
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
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Tunnel ref (resolved)</span>
                <span class="text-[color:var(--text)]">
                  {{ cloudflaredTunnelName || '—' }}
                </span>
              </UiListRow>
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Tunnel source</span>
                <span class="text-[color:var(--text)]">
                  {{ settingsSources?.cloudflaredTunnel || 'unset' }}
                </span>
              </UiListRow>
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Account ID source</span>
                <span class="text-[color:var(--text)]">
                  {{ settingsSources?.cloudflareAccountId || 'unset' }}
                </span>
              </UiListRow>
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Zone ID source</span>
                <span class="text-[color:var(--text)]">
                  {{ settingsSources?.cloudflareZoneId || 'unset' }}
                </span>
              </UiListRow>
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Token source</span>
                <span class="text-[color:var(--text)]">
                  {{ settingsSources?.cloudflareToken || 'unset' }}
                </span>
              </UiListRow>
              <UiListRow class="flex items-center justify-between gap-2">
                <span>Config path source</span>
                <span class="text-[color:var(--text)]">
                  {{ settingsSources?.cloudflaredConfigPath || 'unset' }}
                </span>
              </UiListRow>
            </div>
          </div>

          <label class="grid gap-2" data-onboard="host-cloudflared-config">
            <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflared config path
            </span>
            <UiInput
              v-model="settingsForm.cloudflaredConfigPath"
              type="text"
              placeholder="~/.cloudflared/config.yml"
              :disabled="loading"
            />
          </label>

          <UiInlineFeedback v-if="error" tone="error">
            {{ error }}
          </UiInlineFeedback>

          <UiInlineFeedback v-if="success" tone="ok">
            {{ success }}
          </UiInlineFeedback>

          <div class="flex flex-wrap gap-3">
            <UiButton
              type="submit"
              variant="primary"
              size="md"
              :disabled="saving || loading"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="saving" />
                {{ saving ? 'Saving...' : 'Save settings' }}
              </span>
            </UiButton>
            <UiButton variant="ghost" size="md" :disabled="loading" @click="loadSettings">
              Reload
            </UiButton>
          </div>
        </form>
      </div>
    </UiFormSidePanel>
  </section>
</template>
