<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiOnboardingOverlay from '@/components/ui/UiOnboardingOverlay.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import type { CloudflaredPreview, Settings } from '@/types/settings'
import type { TunnelHealth } from '@/types/health'
import type { OnboardingStep } from '@/types/onboarding'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)
const healthError = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
const settings = ref<Settings | null>(null)
const settingsLoading = ref(false)
const settingsError = ref<string | null>(null)
const onboardingKey = 'warp-panel-onboarding-networking'
const onboardingOpen = ref(false)
const onboardingStep = ref(0)

const onboardingSteps: OnboardingStep[] = [
  {
    id: 'tunnel-health',
    title: 'Verify tunnel health',
    description: 'Confirm cloudflared reports a healthy connection and active connectors.',
    target: "[data-onboard='network-tunnel']",
  },
  {
    id: 'ingress-preview',
    title: 'Review ingress rules',
    description: 'Double-check the active ingress config before routing DNS to new services.',
    target: "[data-onboard='network-ingress']",
    links: [
      {
        label: 'Cloudflared ingress docs',
        href: 'https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/configuration/local-management/ingress/',
      },
    ],
  },
  {
    id: 'refresh',
    title: 'Refresh after changes',
    description: 'Use the refresh controls whenever the tunnel or config changes.',
    target: "[data-onboard='network-actions']",
  },
]

const hasPreview = computed(() => Boolean(preview.value?.contents))
const cloudflareTokenConfigured = computed(() => Boolean(settings.value?.cloudflareToken))
const baseDomainLabel = computed(() => settings.value?.baseDomain || 'Not set')

type IngressRoute = {
  hostname: string
  service: string
}

const ingressRoutes = computed(() => parseIngressRoutes(preview.value?.contents ?? ''))

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

const loadHealth = async () => {
  healthLoading.value = true
  healthError.value = null
  try {
    const { data } = await healthApi.tunnel()
    tunnelHealth.value = data
  } catch (err) {
    healthError.value = apiErrorMessage(err)
    tunnelHealth.value = { status: 'error', detail: healthError.value }
  } finally {
    healthLoading.value = false
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

const loadSettings = async () => {
  settingsLoading.value = true
  settingsError.value = null
  try {
    const { data } = await settingsApi.get()
    settings.value = data.settings
  } catch (err) {
    settingsError.value = apiErrorMessage(err)
    settings.value = null
  } finally {
    settingsLoading.value = false
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
  await Promise.all([loadHealth(), loadPreview(), loadSettings()])
  if (typeof window !== 'undefined') {
    const seen = window.localStorage.getItem(onboardingKey)
    if (seen !== 'done') {
      onboardingOpen.value = true
    }
  }
})

function parseIngressRoutes(contents: string): IngressRoute[] {
  if (!contents) return []
  const routes: IngressRoute[] = []
  const lines = contents.split('\n')
  let current: IngressRoute | null = null

  for (const rawLine of lines) {
    const trimmed = rawLine.trim()
    if (trimmed.startsWith('- hostname:')) {
      if (current?.hostname && current.service) {
        routes.push(current)
      }
      const hostname = trimmed.replace('- hostname:', '').trim()
      current = hostname ? { hostname, service: '' } : null
      continue
    }
    if (trimmed.startsWith('hostname:')) {
      if (current?.hostname && current.service) {
        routes.push(current)
      }
      const hostname = trimmed.replace('hostname:', '').trim()
      current = hostname ? { hostname, service: '' } : null
      continue
    }
    if (trimmed.startsWith('service:') && current) {
      const service = trimmed.replace('service:', '').trim()
      current.service = service
    }
  }

  if (current?.hostname && current.service) {
    routes.push(current)
  }

  return routes
}
</script>

<template>
  <section class="page space-y-10">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Networking
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Tunnel and DNS
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Monitor cloudflared connectivity and the active ingress configuration.
        </p>
      </div>
      <div class="flex flex-wrap gap-3" data-onboard="network-actions">
        <UiButton variant="ghost" size="sm" @click="startOnboarding">
          View guide
        </UiButton>
        <UiButton variant="ghost" size="sm" :disabled="healthLoading" @click="loadHealth">
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="healthLoading" />
            Refresh status
          </span>
        </UiButton>
        <UiButton variant="ghost" size="sm" :disabled="previewLoading" @click="loadPreview">
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="previewLoading" />
            Refresh preview
          </span>
        </UiButton>
      </div>
    </div>

    <UiState v-if="healthError" tone="error">
      {{ healthError }}
    </UiState>

    <div class="grid gap-6 lg:grid-cols-[0.9fr,1.1fr]">
      <UiPanel as="article" class="space-y-4 p-6" data-onboard="network-tunnel">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Tunnel
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Cloudflared status
            </h2>
          </div>
          <UiBadge :tone="healthTone(tunnelHealth?.status)">
            {{ tunnelHealth?.status || 'unknown' }}
          </UiBadge>
        </div>

        <UiState v-if="healthLoading" loading>
          Checking tunnel health...
        </UiState>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Tunnel name</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth?.tunnel || '--' }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Connectors</span>
            <span class="text-[color:var(--text)]">
              {{
                tunnelHealth &&
                (tunnelHealth.status === 'ok' || tunnelHealth.status === 'warning')
                  ? tunnelHealth.connections
                  : '--'
              }}
            </span>
          </UiListRow>
        </div>

        <p v-if="tunnelHealth?.configPath" class="text-xs text-[color:var(--muted)]">
          {{ tunnelHealth.configPath }}
        </p>
        <p v-if="tunnelHealth?.detail" class="text-xs text-[color:var(--muted)]">
          {{ tunnelHealth.detail }}
        </p>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="space-y-4 p-6" data-onboard="network-ingress">
        <div class="flex items-center justify-between gap-2">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Ingress
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Cloudflared config preview
            </h2>
          </div>
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

    <div class="grid gap-6 lg:grid-cols-[1fr,1fr]">
      <UiPanel as="article" class="space-y-4 p-6">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              DNS
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Expected DNS records
            </h2>
          </div>
          <UiBadge :tone="ingressRoutes.length > 0 ? 'ok' : 'neutral'">
            {{ ingressRoutes.length > 0 ? `${ingressRoutes.length} routes` : 'No routes' }}
          </UiBadge>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Based on ingress rules in {{ baseDomainLabel }}.
        </p>

        <UiState v-if="previewLoading" loading>
          Parsing ingress rules...
        </UiState>

        <UiState v-else-if="previewError" tone="error">
          {{ previewError }}
        </UiState>

        <UiState v-else-if="ingressRoutes.length === 0">
          No ingress hostnames found yet.
        </UiState>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow v-for="route in ingressRoutes" :key="route.hostname" class="space-y-2">
            <p class="text-[color:var(--text)]">
              {{ route.hostname }}
            </p>
            <p>Service: {{ route.service }}</p>
          </UiListRow>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="space-y-4 p-6">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflare
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              API readiness
            </h2>
          </div>
          <UiBadge :tone="cloudflareTokenConfigured ? 'ok' : 'warn'">
            {{ cloudflareTokenConfigured ? 'Token set' : 'Token missing' }}
          </UiBadge>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Cloudflare credentials power DNS automation and tunnel routing updates.
        </p>

        <UiState v-if="settingsLoading" loading>
          Loading Cloudflare settings...
        </UiState>

        <UiState v-else-if="settingsError" tone="error">
          {{ settingsError }}
        </UiState>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Base domain</span>
            <span class="text-[color:var(--text)]">
              {{ baseDomainLabel }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>API token</span>
            <span class="text-[color:var(--text)]">
              {{ cloudflareTokenConfigured ? 'Configured' : 'Missing' }}
            </span>
          </UiListRow>
          <UiListRow class="flex items-center justify-between gap-2">
            <span>Tunnel name</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth?.tunnel || '--' }}
            </span>
          </UiListRow>
        </div>
      </UiPanel>
    </div>

    <UiOnboardingOverlay
      v-model="onboardingOpen"
      v-model:stepIndex="onboardingStep"
      :steps="onboardingSteps"
      @finish="markOnboardingComplete"
      @skip="markOnboardingComplete"
    />
  </section>
</template>
