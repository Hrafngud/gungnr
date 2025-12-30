<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiState from '@/components/ui/UiState.vue'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { hostApi } from '@/services/host'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import { useToastStore } from '@/stores/toasts'
import type { CloudflaredPreview, Settings } from '@/types/settings'
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
  cloudflaredConfigPath: '',
})

const toastStore = useToastStore()

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
const onboardingKey = 'warp-panel-onboarding-host'
const onboardingOpen = ref(false)
const onboardingStep = ref(0)

const dockerHealth = ref<DockerHealth | null>(null)
const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)

const containers = ref<DockerContainer[]>([])
const containersLoading = ref(false)
const containersError = ref<string | null>(null)

type ForwardState = {
  subdomain: string
  port: string
  loading: boolean
  error: string | null
  jobId: number | null
}

const forwardTargets = reactive<Record<string, ForwardState>>({})

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'integrations',
    title: 'Verify host integrations',
    description: 'Confirm Docker and cloudflared connectivity before enabling automation.',
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
    description: 'Use the active config.yml so Warp Panel can update ingress safely.',
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

const hostPortsFor = (container: DockerContainer) => {
  const ports = (container.portBindings ?? [])
    .filter((binding) => binding.published && binding.hostPort)
    .map((binding) => binding.hostPort)
  return Array.from(new Set(ports))
}

const ensureForwardState = (container: DockerContainer) => {
  if (!forwardTargets[container.id]) {
    const ports = hostPortsFor(container)
    const firstPort = ports[0]
    forwardTargets[container.id] = {
      subdomain: '',
      port: typeof firstPort === 'number' ? firstPort.toString() : '',
      loading: false,
      error: null,
      jobId: null,
    }
  }
}

const forwardStateFor = (container: DockerContainer): ForwardState => {
  ensureForwardState(container)
  return forwardTargets[container.id] as ForwardState
}

const loadSettings = async () => {
  loading.value = true
  error.value = null
  try {
    const { data } = await settingsApi.get()
    Object.assign(settingsForm, data.settings)
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
    containers.value.forEach((container) => ensureForwardState(container))
  } catch (err) {
    containersError.value = apiErrorMessage(err)
  } finally {
    containersLoading.value = false
  }
}

const queueForward = async (container: DockerContainer) => {
  const state = forwardStateFor(container)
  state.error = null
  state.jobId = null

  const port = Number(state.port)
  if (!state.subdomain.trim()) {
    state.error = 'Subdomain is required.'
    return
  }
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    state.error = 'Select a valid host port.'
    return
  }

  state.loading = true
  try {
    const { data } = await projectsApi.quickService({
      subdomain: state.subdomain,
      port,
    })
    state.jobId = data.job.id
    toastStore.success('Forward queued.', 'Tunnel forwarding')
  } catch (err) {
    const message = apiErrorMessage(err)
    state.error = message
    toastStore.error(message, 'Forward failed')
  } finally {
    state.loading = false
  }
}

const startOnboarding = () => {
  onboardingStep.value = 0
  onboardingOpen.value = true
}

const markOnboardingComplete = () => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(onboardingKey, 'done')
  }
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadPreview(), loadHealth(), loadContainers()])
  if (typeof window !== 'undefined') {
    const seen = window.localStorage.getItem(onboardingKey)
    if (seen !== 'done') {
      onboardingOpen.value = true
    }
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
      </div>
    </div>

    <UiInlineFeedback v-if="error" tone="error">
      {{ error }}
    </UiInlineFeedback>

    <UiInlineFeedback v-if="success" tone="ok">
      {{ success }}
    </UiInlineFeedback>

    <UiPanel as="section" class="space-y-6 p-6" data-onboard="host-integrations">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Status
          </p>
          <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
            Host integrations
          </h2>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Keep Docker and cloudflared connectivity ready for automation.
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

      <div v-else class="grid gap-4 md:grid-cols-2">
        <UiPanel as="article" variant="soft" class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Docker
              </p>
              <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                Engine status
              </h3>
            </div>
            <UiBadge :tone="healthTone(dockerHealth?.status)">
              {{ dockerHealth?.status || 'unknown' }}
            </UiBadge>
          </div>

          <div class="space-y-2 text-xs text-[color:var(--muted)]">
            <div class="flex items-center justify-between gap-2">
              <span>Containers</span>
              <span class="text-[color:var(--text)]">
                {{
                  dockerHealth && dockerHealth.status === 'ok'
                    ? dockerHealth.containers
                    : '—'
                }}
              </span>
            </div>
          </div>

          <p v-if="dockerHealth?.detail" class="text-xs text-[color:var(--muted)]">
            {{ dockerHealth.detail }}
          </p>
        </UiPanel>

        <UiPanel as="article" variant="soft" class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                Tunnel
              </p>
              <h3 class="mt-2 text-base font-semibold text-[color:var(--text)]">
                Cloudflared status
              </h3>
            </div>
            <UiBadge :tone="healthTone(tunnelHealth?.status)">
              {{ tunnelHealth?.status || 'unknown' }}
            </UiBadge>
          </div>

          <div class="space-y-2 text-xs text-[color:var(--muted)]">
            <div class="flex items-center justify-between gap-2">
              <span>Tunnel</span>
              <span class="text-[color:var(--text)]">
                {{ tunnelHealth?.tunnel || '—' }}
              </span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>Connectors</span>
              <span class="text-[color:var(--text)]">
                {{
                  tunnelHealth &&
                  (tunnelHealth.status === 'ok' || tunnelHealth.status === 'warning')
                    ? tunnelHealth.connections
                    : '—'
                }}
              </span>
            </div>
          </div>

          <p v-if="tunnelHealth?.configPath" class="text-xs text-[color:var(--muted)]">
            {{ tunnelHealth.configPath }}
          </p>
          <p v-if="tunnelHealth?.detail" class="text-xs text-[color:var(--muted)]">
            {{ tunnelHealth.detail }}
          </p>
        </UiPanel>
      </div>
    </UiPanel>

    <div class="grid gap-6 lg:grid-cols-[1.1fr,0.9fr]">
      <UiPanel as="form" variant="raise" class="space-y-6 p-6" @submit.prevent="saveSettings">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Settings
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Panel overrides
            </h2>
          </div>
          <UiBadge tone="neutral">Overrides</UiBadge>
        </div>

        <div class="grid gap-4 text-sm text-[color:var(--muted)]">
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

            <p class="text-xs text-[color:var(--muted)]">
              Use a Cloudflare API token (not a global API key) with Account:Cloudflare Tunnel:Edit
              and Zone:DNS:Edit for the configured account and zone.
            </p>
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
        </div>

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
      </UiPanel>

      <UiPanel variant="raise" class="space-y-4 p-6">
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
    </div>

    <section class="space-y-6">
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Host
          </p>
          <h2 class="mt-2 text-2xl font-semibold text-[color:var(--text)]">
            Running containers
          </h2>
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
              Tunnel forward
            </p>
            <div class="grid gap-3 text-xs text-[color:var(--muted)] sm:grid-cols-[1.2fr,0.8fr]">
              <UiInput
                v-model="forwardStateFor(container).subdomain"
                type="text"
                placeholder="subdomain"
                :disabled="forwardStateFor(container).loading"
              />
              <UiSelect
                v-model="forwardStateFor(container).port"
                :disabled="forwardStateFor(container).loading"
              >
                <option value="">Select port</option>
                <option
                  v-for="port in hostPortsFor(container)"
                  :key="port"
                  :value="port"
                >
                  {{ port }}
                </option>
              </UiSelect>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <UiButton
                variant="primary"
                size="md"
                :disabled="forwardStateFor(container).loading"
                @click="queueForward(container)"
              >
                <span class="flex items-center gap-2">
                  <UiInlineSpinner v-if="forwardStateFor(container).loading" />
                  {{
                    forwardStateFor(container).loading
                      ? 'Forwarding...'
                      : 'Forward via tunnel'
                  }}
                </span>
              </UiButton>

              <UiButton
                v-if="forwardStateFor(container).jobId"
                :as="RouterLink"
                :to="`/jobs/${forwardStateFor(container).jobId}`"
                variant="ghost"
                size="md"
              >
                View job
              </UiButton>
            </div>

            <UiInlineFeedback v-if="forwardStateFor(container).error" tone="error">
              {{ forwardStateFor(container).error }}
            </UiInlineFeedback>
          </UiPanel>
        </UiListRow>
      </div>
    </section>

    <UiOnboardingOverlay
      v-model="onboardingOpen"
      v-model:stepIndex="onboardingStep"
      :steps="onboardingSteps"
      @finish="markOnboardingComplete"
      @skip="markOnboardingComplete"
    />
  </section>
</template>
