<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { cloudflareApi } from '@/services/cloudflare'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import { usePageLoadingStore } from '@/stores/pageLoading'
import type { CloudflarePreflight } from '@/types/cloudflare'
import type { CloudflaredPreview, Settings, SettingsSources } from '@/types/settings'
import type { TunnelHealth } from '@/types/health'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const tunnelHealth = ref<TunnelHealth | null>(null)
const healthLoading = ref(false)
const healthError = ref<string | null>(null)

const preview = ref<CloudflaredPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
const settings = ref<Settings | null>(null)
const settingsSources = ref<SettingsSources | null>(null)
const cloudflaredTunnelName = ref<string | null>(null)
const settingsLoading = ref(false)
const settingsError = ref<string | null>(null)
const preflight = ref<CloudflarePreflight | null>(null)
const preflightLoading = ref(false)
const preflightError = ref<string | null>(null)
const ingressPreviewOpen = ref(false)
const domainFormOpen = ref(false)
const domainInput = ref('')
const domainSaving = ref(false)
const domainError = ref<string | null>(null)
const domainSuccess = ref<string | null>(null)
const pageLoading = usePageLoadingStore()

const hasPreview = computed(() => Boolean(preview.value?.contents))
const cloudflareTokenConfigured = computed(() => Boolean(settings.value?.cloudflareToken))
const baseDomainLabel = computed(() => settings.value?.baseDomain || 'Unavailable')
const baseDomainValue = computed(() => settings.value?.baseDomain?.trim().toLowerCase() || '')
const additionalDomains = computed(() => {
  const normalized = (settings.value?.additionalDomains ?? [])
    .map((domain) => domain.trim().toLowerCase())
    .filter(Boolean)
  const seen = new Set<string>()
  const output: string[] = []
  normalized.forEach((domain) => {
    if (domain === baseDomainValue.value || seen.has(domain)) return
    seen.add(domain)
    output.push(domain)
  })
  return output
})
const domainCountLabel = computed(() => {
  const total = (baseDomainValue.value ? 1 : 0) + additionalDomains.value.length
  return total === 1 ? '1 domain' : `${total} domains`
})

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

const preflightTone = (status?: string): BadgeTone => {
  switch (status) {
    case 'ok':
      return 'ok'
    case 'warning':
      return 'warn'
    case 'missing':
      return 'neutral'
    case 'error':
      return 'error'
    case 'skipped':
      return 'neutral'
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
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
  } catch (err) {
    settingsError.value = apiErrorMessage(err)
    settings.value = null
  } finally {
    settingsLoading.value = false
  }
}

const saveDomains = async (domains: string[]) => {
  if (!settings.value) {
    domainError.value = 'Settings are unavailable.'
    return
  }
  domainSaving.value = true
  domainError.value = null
  domainSuccess.value = null
  try {
    const { data } = await settingsApi.update({
      ...settings.value,
      additionalDomains: domains,
    })
    settings.value = data.settings
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
    domainSuccess.value = 'Domains updated.'
  } catch (err) {
    domainError.value = apiErrorMessage(err)
  } finally {
    domainSaving.value = false
  }
}

const addDomain = async () => {
  domainError.value = null
  domainSuccess.value = null
  const candidate = domainInput.value.trim().toLowerCase()
  if (!candidate) {
    domainError.value = 'Domain is required.'
    return
  }
  if (candidate === baseDomainValue.value) {
    domainError.value = 'That domain is already the base domain.'
    return
  }
  if (additionalDomains.value.includes(candidate)) {
    domainError.value = 'That domain is already listed.'
    return
  }
  await saveDomains([...additionalDomains.value, candidate])
  if (!domainError.value) {
    domainInput.value = ''
  }
}

const removeDomain = async (domain: string) => {
  domainError.value = null
  domainSuccess.value = null
  const next = additionalDomains.value.filter((item) => item !== domain)
  await saveDomains(next)
}

const loadPreflight = async () => {
  preflightLoading.value = true
  preflightError.value = null
  try {
    const { data } = await cloudflareApi.preflight()
    preflight.value = data
  } catch (err) {
    preflightError.value = apiErrorMessage(err)
    preflight.value = null
  } finally {
    preflightLoading.value = false
  }
}

onMounted(async () => {
  pageLoading.start('Loading networking status...')
  await Promise.all([loadHealth(), loadPreview(), loadSettings(), loadPreflight()])
  pageLoading.stop()
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
          Monitor the host cloudflared service and the active ingress configuration.
        </p>
      </div>
      <div class="flex flex-wrap gap-3">
        <UiButton variant="ghost" size="sm" :disabled="healthLoading" @click="loadHealth">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="healthLoading" />
            Refresh status
          </span>
        </UiButton>
        <UiButton variant="ghost" size="sm" @click="domainFormOpen = true">
          Domains config
        </UiButton>
        <UiButton variant="ghost" size="sm" @click="ingressPreviewOpen = true">
          Ingress preview
        </UiButton>
      </div>
    </div>

    <UiState v-if="healthError" tone="error">
      {{ healthError }}
    </UiState>

    <hr />

    <div class="grid gap-6">
      <UiPanel as="article" class="space-y-4 p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
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
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Tunnel name</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth?.tunnel || '--' }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
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
        <div
          v-if="tunnelHealth?.diagnostics"
          class="space-y-2 rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)]/80 p-3 text-[11px] text-[color:var(--muted)]"
        >
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Account ID</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth.diagnostics.accountId || '—' }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Zone ID</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth.diagnostics.zoneId || '—' }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Tunnel ref</span>
            <span class="text-[color:var(--text)]">
              {{
                tunnelHealth.diagnostics.tunnel
                  ? `${tunnelHealth.diagnostics.tunnel} (${tunnelHealth.diagnostics.tunnelRefType || 'unknown'})`
                  : '—'
              }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Token set</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth.diagnostics.tokenSet ? 'yes' : 'no' }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Source</span>
            <span class="text-[color:var(--text)]">
              {{
                tunnelHealth.diagnostics.sources
                  ? `acct=${tunnelHealth.diagnostics.sources.cloudflareAccountId}, zone=${tunnelHealth.diagnostics.sources.cloudflareZoneId}, token=${tunnelHealth.diagnostics.sources.cloudflareToken}`
                  : '—'
              }}
            </span>
          </UiListRow>
        </div>
        <div
          v-else-if="settingsSources || cloudflaredTunnelName"
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
      </UiPanel>

      <UiPanel as="article" class="space-y-4 p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Domains
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Available domains
            </h2>
          </div>
          <UiBadge :tone="baseDomainValue ? 'ok' : 'warn'">
            {{ domainCountLabel }}
          </UiBadge>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Base domain plus any additional domains you add. All domains must be in the same Cloudflare
          zone and tunnel account.
        </p>

        <UiState v-if="settingsLoading" loading>
          Loading domains...
        </UiState>

        <UiState v-else-if="settingsError" tone="error">
          {{ settingsError }}
        </UiState>

        <UiState v-else-if="!baseDomainValue && additionalDomains.length === 0">
          No domains configured yet.
        </UiState>

        <div v-else class="space-y-2 text-xs text-[color:var(--muted)]">
          <UiListRow
            v-if="baseDomainValue"
            class="flex flex-wrap items-center justify-between gap-2 break-words"
          >
            <span>Base domain</span>
            <span class="text-[color:var(--text)]">
              {{ baseDomainValue }}
            </span>
          </UiListRow>
          <UiListRow
            v-for="domain in additionalDomains"
            :key="domain"
            class="flex flex-wrap items-center justify-between gap-2 break-words"
          >
            <span>Secondary domain</span>
            <span class="text-[color:var(--text)]">
              {{ domain }}
            </span>
          </UiListRow>
        </div>
      </UiPanel>
    </div>

    <hr />

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <UiPanel as="article" class="space-y-4 p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
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
          Based on the current ingress rules reported by cloudflared.
        </p>

        <UiState v-if="previewLoading" loading>
          Parsing ingress rules...
        </UiState>

        <UiState v-else-if="previewError" tone="error">
          {{ previewError }}
        </UiState>

        <UiState v-else-if="ingressRoutes.length === 0">
          No ingress hostnames reported yet.
        </UiState>

        <div v-else class="space-y-2">
          <div class="grid min-w-0 grid-cols-[minmax(0,0.8fr)_minmax(0,1.4fr)_minmax(0,0.6fr)_minmax(0,1.2fr)] gap-3 text-xs font-semibold uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            <div>Subdomain</div>
            <div>Full hostname</div>
            <div>Type</div>
            <div>Target service</div>
          </div>
          <div class="space-y-1">
            <div
              v-for="route in ingressRoutes"
              :key="route.hostname"
              class="grid min-w-0 grid-cols-[minmax(0,0.8fr)_minmax(0,1.4fr)_minmax(0,0.6fr)_minmax(0,1.2fr)] gap-3 rounded border border-[color:var(--border)] bg-[color:var(--bg-soft)] px-3 py-2 text-xs text-[color:var(--text)]"
            >
              <div class="truncate font-medium">
                {{ route.hostname.split('.')[0] }}
              </div>
              <div class="truncate text-[color:var(--muted)]">
                {{ route.hostname }}
              </div>
              <div class="text-[color:var(--muted)]">
                CNAME
              </div>
              <div class="truncate text-[color:var(--muted)]">
                {{ route.service }}
              </div>
            </div>
          </div>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="space-y-4 p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflare
            </p>
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              API availability
            </h2>
          </div>
          <div class="flex items-center gap-2">
            <UiBadge :tone="cloudflareTokenConfigured ? 'ok' : 'warn'">
              {{ cloudflareTokenConfigured ? 'Token available' : 'Token unavailable' }}
            </UiBadge>
            <UiButton
              variant="ghost"
              size="xs"
              :disabled="preflightLoading"
              @click="loadPreflight"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="preflightLoading" />
                Run preflight
              </span>
            </UiButton>
          </div>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Cloudflare credentials power DNS automation for the host-managed tunnel.
        </p>
        <p class="text-xs text-[color:var(--muted)]">
          Credentials are seeded during bootstrap; this panel reports availability. Required scopes:
          Account:Cloudflare Tunnel:Edit and Zone:DNS:Edit.
        </p>

        <UiState v-if="settingsLoading" loading>
          Loading Cloudflare settings...
        </UiState>

        <UiState v-else-if="settingsError" tone="error">
          {{ settingsError }}
        </UiState>

        <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Base domain</span>
            <span class="text-[color:var(--text)]">
              {{ baseDomainLabel }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>API token</span>
            <span class="text-[color:var(--text)]">
              {{ cloudflareTokenConfigured ? 'Available' : 'Unavailable' }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Tunnel name</span>
            <span class="text-[color:var(--text)]">
              {{ tunnelHealth?.tunnel || '--' }}
            </span>
          </UiListRow>
        </div>

        <UiState v-if="preflightLoading" loading>
          Running Cloudflare preflight...
        </UiState>

        <UiState v-else-if="preflightError" tone="error">
          {{ preflightError }}
        </UiState>

        <div v-else-if="preflight" class="space-y-3 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Token check</span>
            <UiBadge :tone="preflightTone(preflight.token.status)">
              {{ preflight.token.status }}
            </UiBadge>
          </UiListRow>
          <p v-if="preflight.token.detail" class="text-[color:var(--muted)]">
            {{ preflight.token.detail }}
          </p>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Account check</span>
            <UiBadge :tone="preflightTone(preflight.account.status)">
              {{ preflight.account.status }}
            </UiBadge>
          </UiListRow>
          <p v-if="preflight.account.detail" class="text-[color:var(--muted)]">
            {{ preflight.account.detail }}
          </p>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Zone check</span>
            <UiBadge :tone="preflightTone(preflight.zone.status)">
              {{ preflight.zone.status }}
            </UiBadge>
          </UiListRow>
          <p v-if="preflight.zone.detail" class="text-[color:var(--muted)]">
            {{ preflight.zone.detail }}
          </p>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Tunnel ref</span>
            <span class="text-[color:var(--text)]">
              {{
                preflight.tunnelRef
                  ? `${preflight.tunnelRef} (${preflight.tunnelRefType || 'unknown'})`
                  : '--'
              }}
            </span>
          </UiListRow>
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Tunnel check</span>
            <UiBadge :tone="preflightTone(preflight.tunnel.status)">
              {{ preflight.tunnel.status }}
            </UiBadge>
          </UiListRow>
          <p v-if="preflight.tunnel.detail" class="text-[color:var(--muted)]">
            {{ preflight.tunnel.detail }}
          </p>
        </div>
      </UiPanel>
    </div>

    <UiFormSidePanel
      v-model="domainFormOpen"
      title="Domains config"
      eyebrow="Domains"
    >
      <form class="space-y-4" @submit.prevent="addDomain">
        <div class="space-y-2">
          <p class="text-xs text-[color:var(--muted)]">
            Add secondary domains that share the same Cloudflare zone and tunnel account. The base
            domain remains required for the panel itself.
          </p>
        </div>

        <label class="grid gap-2 text-sm">
          <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            New domain
          </span>
          <div class="flex flex-wrap gap-2">
            <UiInput
              v-model="domainInput"
              type="text"
              placeholder="secondary.example.com"
              :disabled="domainSaving"
            />
            <UiButton type="submit" variant="ghost" :disabled="domainSaving">
              {{ domainSaving ? 'Saving...' : 'Add domain' }}
            </UiButton>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            Domains must already exist in Cloudflare and belong to the configured account/zone.
          </p>
        </label>

        <UiInlineFeedback v-if="domainError" tone="error">
          {{ domainError }}
        </UiInlineFeedback>
        <UiInlineFeedback v-if="domainSuccess" tone="ok">
          {{ domainSuccess }}
        </UiInlineFeedback>

        <div class="space-y-2 text-xs text-[color:var(--muted)]">
          <UiListRow class="flex flex-wrap items-center justify-between gap-2 break-words">
            <span>Base domain</span>
            <span class="text-[color:var(--text)]">
              {{ baseDomainValue || 'Unavailable' }}
            </span>
          </UiListRow>
          <UiState v-if="additionalDomains.length === 0">
            No secondary domains added yet.
          </UiState>
          <UiListRow
            v-for="domain in additionalDomains"
            :key="domain"
            class="flex flex-wrap items-center justify-between gap-2 break-words"
          >
            <span>{{ domain }}</span>
            <UiButton
              type="button"
              variant="ghost"
              size="xs"
              :disabled="domainSaving"
              @click="removeDomain(domain)"
            >
              Remove
            </UiButton>
          </UiListRow>
        </div>
      </form>
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
            <h2 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              Cloudflared config preview
            </h2>
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
  </section>
</template>
