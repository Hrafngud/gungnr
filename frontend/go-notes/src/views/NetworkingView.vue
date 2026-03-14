<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiStatusDot from '@/components/ui/UiStatusDot.vue'
import UiState from '@/components/ui/UiState.vue'
import UiCopyableValue from '@/components/ui/UiCopyableValue.vue'
import NavIcon from '@/components/NavIcon.vue'
import NetworkingReadonlyRow from '@/components/networking/NetworkingReadonlyRow.vue'
import { cloudflareApi } from '@/services/cloudflare'
import { healthApi } from '@/services/health'
import { settingsApi } from '@/services/settings'
import { apiErrorMessage } from '@/services/api'
import { usePageLoadingStore } from '@/stores/pageLoading'
import { useToastStore } from '@/stores/toasts'
import { useAuthStore } from '@/stores/auth'
import { isCopyValueAllowed as isClipboardValueAllowed, writeTextToClipboard } from '@/utils/clipboard'
import type { CloudflarePreflight, CloudflareZone } from '@/types/cloudflare'
import type { CloudflaredPreview, Settings, SettingsSources } from '@/types/settings'
import type { TunnelHealth } from '@/types/health'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

interface IngressRoute {
  hostname: string
  service: string
}

interface NetworkingFieldRow {
  key: string
  label: string
  value: string
  copyLabel: string
  copyable?: boolean
}

interface PreflightInventoryRow {
  key: 'token' | 'account' | 'zone' | 'tunnel'
  label: string
  status: string
  detail: string
}

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

const syncLoading = ref(false)
const zones = ref<CloudflareZone[]>([])
const zonesLoading = ref(false)
const zonesError = ref<string | null>(null)

const refreshing = ref(false)
const copiedFieldKey = ref<string | null>(null)

const pageLoading = usePageLoadingStore()
const toastStore = useToastStore()
const authStore = useAuthStore()

let copiedFieldTimer: ReturnType<typeof setTimeout> | null = null

const hasPreview = computed(() => Boolean(preview.value?.contents))
const cloudflareTokenConfigured = computed(() => Boolean(settings.value?.cloudflareToken?.trim()))
const tokenStatusLabel = computed(() => (cloudflareTokenConfigured.value ? 'Available' : 'Unavailable'))
const baseDomainValue = computed(() => settings.value?.baseDomain?.trim().toLowerCase() || '')
const baseDomainLabel = computed(() => baseDomainValue.value || 'Unavailable')
const canSyncCloudflare = computed(() => authStore.isAdmin)

const availableDomains = computed(() => {
  const normalized = zones.value
    .map((zone) => zone.name?.trim().toLowerCase())
    .filter(Boolean) as string[]

  const seen = new Set<string>()
  const output: string[] = []

  normalized.forEach((domain) => {
    if (seen.has(domain)) return
    seen.add(domain)
    output.push(domain)
  })

  return output
})

const listedDomains = computed(() => {
  const seen = new Set<string>()
  const output: string[] = []

  if (baseDomainValue.value) {
    seen.add(baseDomainValue.value)
    output.push(baseDomainValue.value)
  }

  availableDomains.value.forEach((domain) => {
    if (seen.has(domain)) return
    seen.add(domain)
    output.push(domain)
  })

  return output
})

const secondaryDomains = computed(() => (
  listedDomains.value.filter((domain) => domain !== baseDomainValue.value)
))

const domainCountLabel = computed(() => {
  const total = listedDomains.value.length
  return total === 1 ? '1 domain' : `${total} domains`
})

const ingressRoutes = computed(() => parseIngressRoutes(preview.value?.contents ?? ''))

const routeInventory = computed(() => ingressRoutes.value.map((route, index) => ({
  key: `route-${index}-${route.hostname}-${route.service}`,
  subdomain: route.hostname.includes('.') ? route.hostname.split('.')[0] : route.hostname,
  hostname: route.hostname,
  service: route.service,
})))

const tunnelConnectionsLabel = computed(() => {
  if (!tunnelHealth.value) return '—'
  if (tunnelHealth.value.status !== 'ok' && tunnelHealth.value.status !== 'warning') return '—'
  if (typeof tunnelHealth.value.connections !== 'number') return '—'
  return String(tunnelHealth.value.connections)
})

const tunnelDetailMessage = computed(() => tunnelHealth.value?.detail?.trim() || '')

const tunnelReferenceLabel = computed(() => {
  const diagnostics = tunnelHealth.value?.diagnostics
  if (diagnostics?.tunnel?.trim()) {
    return `${diagnostics.tunnel.trim()} (${diagnostics.tunnelRefType || 'unknown'})`
  }
  return cloudflaredTunnelName.value?.trim() || '—'
})

const preflightRows = computed<PreflightInventoryRow[]>(() => {
  if (!preflight.value) return []

  return [
    {
      key: 'token',
      label: 'Token check',
      status: preflight.value.token.status,
      detail: preflight.value.token.detail?.trim() || '',
    },
    {
      key: 'account',
      label: 'Account check',
      status: preflight.value.account.status,
      detail: preflight.value.account.detail?.trim() || '',
    },
    {
      key: 'zone',
      label: 'Zone check',
      status: preflight.value.zone.status,
      detail: preflight.value.zone.detail?.trim() || '',
    },
    {
      key: 'tunnel',
      label: 'Tunnel check',
      status: preflight.value.tunnel.status,
      detail: preflight.value.tunnel.detail?.trim() || '',
    },
  ]
})

const preflightPassingCount = computed(() => (
  preflightRows.value.filter((row) => row.status === 'ok').length
))

const preflightSummaryTone = computed<BadgeTone>(() => {
  if (preflightError.value) return 'error'
  if (preflightLoading.value || preflightRows.value.length === 0) return 'neutral'
  if (preflightRows.value.some((row) => row.status === 'error')) return 'error'
  if (preflightRows.value.some((row) => row.status === 'warning')) return 'warn'
  return 'ok'
})

const preflightSummaryLabel = computed(() => {
  if (preflightLoading.value) return 'Running checks'
  if (preflightError.value) return 'Preflight failed'
  if (preflightRows.value.length === 0) return 'Not run yet'

  const healthy = preflightPassingCount.value
  const total = preflightRows.value.length
  return `${healthy}/${total} passing`
})

const settingsSourcePayload = computed(() => (
  tunnelHealth.value?.diagnostics?.sources ?? settingsSources.value ?? null
))

const tunnelRuntimeRows = computed<NetworkingFieldRow[]>(() => ([
  {
    key: 'runtime-tunnel-name',
    label: 'Tunnel name',
    value: tunnelHealth.value?.tunnel?.trim() || cloudflaredTunnelName.value?.trim() || '—',
    copyLabel: 'Tunnel name',
  },
  {
    key: 'runtime-connectors',
    label: 'Connectors',
    value: tunnelConnectionsLabel.value,
    copyLabel: 'Connector count',
    copyable: false,
  },
  {
    key: 'runtime-config-path',
    label: 'Config path',
    value: tunnelHealth.value?.configPath?.trim() || settings.value?.cloudflaredConfigPath?.trim() || '—',
    copyLabel: 'Cloudflared config path',
  },
  {
    key: 'runtime-domain',
    label: 'Domain',
    value: tunnelHealth.value?.diagnostics?.domain?.trim() || baseDomainValue.value || '—',
    copyLabel: 'Tunnel domain',
  },
]))

const tunnelIdentityRows = computed<NetworkingFieldRow[]>(() => {
  const diagnostics = tunnelHealth.value?.diagnostics
  const accountId = diagnostics?.accountId?.trim() || settings.value?.cloudflareAccountId?.trim() || '—'
  const zoneId = diagnostics?.zoneId?.trim() || settings.value?.cloudflareZoneId?.trim() || '—'
  const tokenSetValue = diagnostics ? (diagnostics.tokenSet ? 'yes' : 'no') : '—'

  return [
    {
      key: 'identity-account-id',
      label: 'Account ID',
      value: accountId,
      copyLabel: 'Cloudflare account ID',
    },
    {
      key: 'identity-zone-id',
      label: 'Zone ID',
      value: zoneId,
      copyLabel: 'Cloudflare zone ID',
    },
    {
      key: 'identity-tunnel-ref',
      label: 'Tunnel ref',
      value: tunnelReferenceLabel.value,
      copyLabel: 'Tunnel reference',
    },
    {
      key: 'identity-token-set',
      label: 'Token set',
      value: tokenSetValue,
      copyLabel: 'Token set status',
      copyable: false,
    },
  ]
})

const sourceEvidenceRows = computed<NetworkingFieldRow[]>(() => {
  const sources = settingsSourcePayload.value
  if (!sources) return []

  return [
    {
      key: 'source-tunnel',
      label: 'Tunnel source',
      value: sources.cloudflaredTunnel || 'unset',
      copyLabel: 'Tunnel source',
    },
    {
      key: 'source-account',
      label: 'Account ID source',
      value: sources.cloudflareAccountId || 'unset',
      copyLabel: 'Cloudflare account ID source',
    },
    {
      key: 'source-zone',
      label: 'Zone ID source',
      value: sources.cloudflareZoneId || 'unset',
      copyLabel: 'Cloudflare zone ID source',
    },
    {
      key: 'source-token',
      label: 'Token source',
      value: sources.cloudflareToken || 'unset',
      copyLabel: 'Cloudflare token source',
    },
    {
      key: 'source-config-path',
      label: 'Config path source',
      value: sources.cloudflaredConfigPath || 'unset',
      copyLabel: 'Cloudflared config path source',
    },
  ]
})

const cloudflareContextRows = computed<NetworkingFieldRow[]>(() => ([
  {
    key: 'cloudflare-base-domain',
    label: 'Base domain',
    value: baseDomainLabel.value,
    copyLabel: 'Base domain',
  },
  {
    key: 'cloudflare-token',
    label: 'API token',
    value: tokenStatusLabel.value,
    copyLabel: 'API token status',
    copyable: false,
  },
  {
    key: 'cloudflare-tunnel',
    label: 'Tunnel name',
    value: tunnelHealth.value?.tunnel?.trim() || cloudflaredTunnelName.value?.trim() || '—',
    copyLabel: 'Tunnel name',
  },
  {
    key: 'cloudflare-tunnel-ref',
    label: 'Tunnel ref',
    value: preflight.value?.tunnelRef
      ? `${preflight.value.tunnelRef} (${preflight.value.tunnelRefType || 'unknown'})`
      : tunnelReferenceLabel.value,
    copyLabel: 'Tunnel reference',
  },
]))

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

const statusDotTone = (status?: string): BadgeTone => {
  switch (status) {
    case 'ok':
      return 'ok'
    case 'warning':
      return 'warn'
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
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
  } catch (err) {
    settingsError.value = apiErrorMessage(err)
    settings.value = null
  } finally {
    settingsLoading.value = false
  }
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

const loadZones = async () => {
  zonesLoading.value = true
  zonesError.value = null

  try {
    const { data } = await cloudflareApi.zones()
    zones.value = data.zones ?? []
  } catch (err) {
    zonesError.value = apiErrorMessage(err)
    zones.value = []
  } finally {
    zonesLoading.value = false
  }
}

const refreshAll = async () => {
  if (refreshing.value) return

  refreshing.value = true
  await Promise.allSettled([loadHealth(), loadPreview(), loadSettings(), loadPreflight(), loadZones()])
  refreshing.value = false
}

const syncCloudflareEnv = async () => {
  if (!canSyncCloudflare.value || syncLoading.value) return

  syncLoading.value = true

  try {
    const { data } = await settingsApi.syncCloudflareEnv()
    settings.value = data.settings
    settingsSources.value = data.sources ?? null
    cloudflaredTunnelName.value = data.cloudflaredTunnelName ?? null
    toastStore.success('Cloudflare settings synced from env.', 'Sync complete')
    await Promise.allSettled([loadPreflight(), loadHealth(), loadZones()])
  } catch (err) {
    toastStore.error(apiErrorMessage(err), 'Sync failed')
  } finally {
    syncLoading.value = false
  }
}

onMounted(async () => {
  pageLoading.start('Loading networking status...')
  await refreshAll()
  pageLoading.stop()
})

onBeforeUnmount(() => {
  if (copiedFieldTimer) {
    clearTimeout(copiedFieldTimer)
    copiedFieldTimer = null
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
      if (current?.hostname && current.service) routes.push(current)
      const hostname = trimmed.replace('- hostname:', '').trim()
      current = hostname ? { hostname, service: '' } : null
      continue
    }

    if (trimmed.startsWith('hostname:')) {
      if (current?.hostname && current.service) routes.push(current)
      const hostname = trimmed.replace('hostname:', '').trim()
      current = hostname ? { hostname, service: '' } : null
      continue
    }

    if (trimmed.startsWith('service:') && current) {
      current.service = trimmed.replace('service:', '').trim()
    }
  }

  if (current?.hostname && current.service) routes.push(current)

  return routes
}

function isCopyValueAllowed(value: string) {
  return isClipboardValueAllowed(value)
}

function isRowCopyable(row: NetworkingFieldRow) {
  if (row.copyable === false) return false
  return isCopyValueAllowed(row.value)
}

async function copyReadonlyValue(payload: string, label: string, fieldKey: string) {
  const value = payload.trim()
  if (!isCopyValueAllowed(value)) {
    toastStore.warn(`${label} is not available for this host.`, 'Copy value')
    return
  }

  try {
    await writeTextToClipboard(value)
    copiedFieldKey.value = fieldKey

    if (copiedFieldTimer) clearTimeout(copiedFieldTimer)
    copiedFieldTimer = setTimeout(() => {
      copiedFieldKey.value = null
      copiedFieldTimer = null
    }, 1500)

    toastStore.success(`${label} copied to clipboard.`, 'Copy value')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const copyPreviewContents = async () => {
  const value = preview.value?.contents?.trim() || ''
  if (!value) {
    toastStore.warn('Ingress preview is empty.', 'Copy preview')
    return
  }

  await copyReadonlyValue(value, 'Ingress preview', 'preview-contents')
}
</script>

<template>
  <section class="page networking-page space-y-8">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Networking
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Tunnel and DNS
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Inspect tunnel health, DNS routes, Cloudflare checks, and ingress config from one screen.
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="refreshing"
          @click="refreshAll"
        >
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="refreshing" />
            Refresh
          </span>
        </UiButton>

        <UiButton
          variant="ghost"
          size="sm"
          :disabled="syncLoading || !canSyncCloudflare"
          @click="syncCloudflareEnv"
        >
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="syncLoading" />
            Sync env
          </span>
        </UiButton>

        <UiButton
          variant="ghost"
          size="sm"
          :disabled="preflightLoading"
          @click="loadPreflight"
        >
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="preflightLoading" />
            Run preflight
          </span>
        </UiButton>
      </div>
    </header>

    <UiState v-if="healthError" tone="error">
      {{ healthError }}
    </UiState>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      <UiPanel variant="soft" class="networking-summary-card">
        <div class="networking-summary-card__head">
          <p class="networking-summary-card__label">Tunnel</p>
        </div>
        <div class="networking-summary-card__body">
          <p class="networking-summary-card__value">{{ tunnelHealth?.status || 'unknown' }}</p>
          <UiStatusDot :tone="statusDotTone(tunnelHealth?.status)" />
        </div>
      </UiPanel>

      <UiPanel variant="soft" class="networking-summary-card">
        <div class="networking-summary-card__head">
          <p class="networking-summary-card__label">Connectors</p>
        </div>
        <div class="networking-summary-card__body">
          <p class="networking-summary-card__value">{{ tunnelConnectionsLabel }}</p>
          <p class="networking-summary-card__meta">Active cloudflared sessions</p>
        </div>
      </UiPanel>

      <UiPanel variant="soft" class="networking-summary-card">
        <div class="networking-summary-card__head">
          <p class="networking-summary-card__label">DNS routes</p>
        </div>
        <div class="networking-summary-card__body">
          <p class="networking-summary-card__value">{{ ingressRoutes.length }}</p>
          <p class="networking-summary-card__meta">Hostnames discovered from ingress</p>
        </div>
      </UiPanel>

      <UiPanel variant="soft" class="networking-summary-card">
        <div class="networking-summary-card__head">
          <p class="networking-summary-card__label">Domains</p>
        </div>
        <div class="networking-summary-card__body">
          <p class="networking-summary-card__value">{{ listedDomains.length }}</p>
          <p class="networking-summary-card__meta">Base + Cloudflare account zones</p>
        </div>
      </UiPanel>

      <UiPanel variant="soft" class="networking-summary-card">
        <div class="networking-summary-card__head">
          <p class="networking-summary-card__label">Cloudflare checks</p>
        </div>
        <div class="networking-summary-card__body">
          <p class="networking-summary-card__value">{{ preflightPassingCount }}/{{ preflightRows.length || 4 }}</p>
          <UiBadge :tone="preflightSummaryTone">
            {{ preflightSummaryLabel }}
          </UiBadge>
        </div>
      </UiPanel>
    </div>

    <div class="grid items-stretch gap-6 2xl:grid-cols-[minmax(0,1.38fr)_minmax(0,1fr)]">
      <UiPanel as="article" class="networking-density-card h-full space-y-5 p-5 lg:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Tunnel
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Runtime and identity
            </h2>
          </div>
          <UiBadge :tone="healthTone(tunnelHealth?.status)">
            {{ tunnelHealth?.status || 'unknown' }}
          </UiBadge>
        </div>

        <p v-if="tunnelDetailMessage" class="text-xs text-[color:var(--muted)]">
          {{ tunnelDetailMessage }}
        </p>

        <UiState v-if="healthLoading" loading>
          Checking tunnel health...
        </UiState>

        <div v-else class="space-y-4">
          <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            <section class="space-y-2">
              <p class="text-[11px] uppercase tracking-[0.28em] text-[color:var(--muted-2)]">
                Runtime
              </p>
              <NetworkingReadonlyRow
                v-for="row in tunnelRuntimeRows"
                :key="row.key"
                :label="row.label"
                :value="row.value"
                :copyable="isRowCopyable(row)"
                :copied="copiedFieldKey === row.key"
                @copy="copyReadonlyValue(row.value, row.copyLabel, row.key)"
              />
            </section>

            <section class="space-y-2">
              <p class="text-[11px] uppercase tracking-[0.28em] text-[color:var(--muted-2)]">
                Tunnel identity
              </p>
              <NetworkingReadonlyRow
                v-for="row in tunnelIdentityRows"
                :key="row.key"
                :label="row.label"
                :value="row.value"
                :copyable="isRowCopyable(row)"
                :copied="copiedFieldKey === row.key"
                @copy="copyReadonlyValue(row.value, row.copyLabel, row.key)"
              />
            </section>
          </div>
        </div>
      </UiPanel>

      <UiPanel as="article" variant="raise" class="networking-density-card h-full space-y-5 p-5 lg:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Cloudflare
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              API availability
            </h2>
          </div>
          <UiBadge :tone="cloudflareTokenConfigured ? 'ok' : 'warn'">
            {{ cloudflareTokenConfigured ? 'Token available' : 'Token unavailable' }}
          </UiBadge>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Credentials are seeded during bootstrap. This panel shows evidence for token/account/zone/tunnel checks.
        </p>

        <div class="networking-scroll-region networking-scroll-region--compact">
          <div class="networking-quad-grid">
            <article
              v-for="row in cloudflareContextRows"
              :key="row.key"
              class="networking-quad-item"
            >
              <p class="networking-quad-item__label">{{ row.label }}</p>
              <UiCopyableValue
                :value="row.value"
                :copyable="isRowCopyable(row)"
                :copied="copiedFieldKey === row.key"
                button-class="networking-quad-copy-btn"
                static-class="networking-quad-item__value"
                @copy="copyReadonlyValue(row.value, row.copyLabel, row.key)"
              />
            </article>
          </div>
        </div>

        <UiState v-if="preflightLoading" loading>
          Running Cloudflare preflight...
        </UiState>

        <UiState v-else-if="preflightError" tone="error">
          {{ preflightError }}
        </UiState>

        <UiState v-else-if="preflightRows.length === 0">
          Run preflight to verify token scope and tunnel permissions.
        </UiState>

        <div v-else class="networking-scroll-region networking-scroll-region--checks">
          <div class="networking-quad-grid">
            <article
              v-for="check in preflightRows"
              :key="check.key"
              class="networking-quad-item networking-quad-item--check"
            >
              <div class="networking-check-row">
                <span class="networking-check-label">
                  <UiStatusDot :tone="statusDotTone(check.status)" />
                  {{ check.label }}
                </span>
                <span class="networking-check-status">{{ check.status }}</span>
              </div>

              <p class="networking-check-detail">
                {{ check.detail || 'No issue reported by this check.' }}
              </p>
            </article>
          </div>
        </div>

        <UiState v-if="settingsError" tone="error">
          {{ settingsError }}
        </UiState>
      </UiPanel>
    </div>

    <UiPanel
      v-if="sourceEvidenceRows.length > 0"
      as="article"
      class="space-y-2 p-5 lg:p-6"
    >
      <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
        Source evidence
      </p>

      <div class="networking-source-lane">
        <div class="networking-source-grid">
          <NetworkingReadonlyRow
            v-for="row in sourceEvidenceRows"
            :key="row.key"
            class="networking-source-item"
            :label="row.label"
            :value="row.value"
            :copyable="isRowCopyable(row)"
            :copied="copiedFieldKey === row.key"
            @copy="copyReadonlyValue(row.value, row.copyLabel, row.key)"
          />
        </div>
      </div>
    </UiPanel>

    <div class="grid items-start gap-6 2xl:grid-cols-[minmax(0,1.46fr)_minmax(0,1fr)]">
      <UiPanel as="article" class="networking-density-card space-y-5 p-5 lg:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              DNS
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Expected DNS records
            </h2>
          </div>

          <div class="flex items-center gap-2">
            <UiBadge :tone="ingressRoutes.length > 0 ? 'ok' : 'neutral'">
              {{ ingressRoutes.length > 0 ? `${ingressRoutes.length} routes` : 'No routes' }}
            </UiBadge>
            <UiButton
              variant="ghost"
              size="xs"
              :disabled="previewLoading"
              @click="loadPreview"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="previewLoading" />
                Refresh routes
              </span>
            </UiButton>
          </div>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Records inferred from the active cloudflared ingress preview.
        </p>

        <UiState v-if="previewLoading" loading>
          Parsing ingress rules...
        </UiState>

        <UiState v-else-if="previewError" tone="error">
          {{ previewError }}
        </UiState>

        <UiState v-else-if="routeInventory.length === 0">
          No ingress hostnames reported yet.
        </UiState>

        <div v-else class="networking-scroll-region networking-scroll-region--routes space-y-1">
          <div class="networking-routes-head hidden md:grid">
            <span>Subdomain</span>
            <span>Full hostname</span>
            <span>Type</span>
            <span>Target service</span>
          </div>

          <article
            v-for="route in routeInventory"
            :key="route.key"
            class="networking-route-row"
          >
            <div class="networking-route-cell">
              <span class="networking-route-mobile-label">Subdomain</span>
              <span class="font-semibold text-[color:var(--text)]">{{ route.subdomain }}</span>
            </div>

            <div class="networking-route-cell">
              <span class="networking-route-mobile-label">Full hostname</span>
              <UiCopyableValue
                :value="route.hostname"
                :copied="copiedFieldKey === `${route.key}-hostname`"
                button-class="networking-route-copy-btn"
                value-class="networking-route-copy-btn__text"
                static-class="networking-route-copy-btn__text"
                @copy="copyReadonlyValue(route.hostname, 'Hostname', `${route.key}-hostname`)"
              />
            </div>

            <div class="networking-route-cell">
              <span class="networking-route-mobile-label">Type</span>
              <span class="text-[color:var(--muted)]">CNAME</span>
            </div>

            <div class="networking-route-cell">
              <span class="networking-route-mobile-label">Target service</span>
              <UiCopyableValue
                :value="route.service"
                :copied="copiedFieldKey === `${route.key}-service`"
                button-class="networking-route-copy-btn"
                value-class="networking-route-copy-btn__text"
                static-class="networking-route-copy-btn__text"
                @copy="copyReadonlyValue(route.service, 'Target service', `${route.key}-service`)"
              />
            </div>
          </article>
        </div>
      </UiPanel>

      <UiPanel as="article" class="networking-density-card space-y-5 p-5 lg:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Domains
            </p>
            <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
              Domain inventory
            </h2>
          </div>

          <div class="flex items-center gap-2">
            <UiBadge :tone="baseDomainValue ? 'ok' : 'warn'">
              {{ domainCountLabel }}
            </UiBadge>
            <UiButton
              variant="ghost"
              size="xs"
              :disabled="zonesLoading"
              @click="loadZones"
            >
              <span class="flex items-center gap-2">
                <UiInlineSpinner v-if="zonesLoading" />
                Refresh domains
              </span>
            </UiButton>
          </div>
        </div>

        <p class="text-xs text-[color:var(--muted)]">
          Base domain and all Cloudflare-managed zones tied to current credentials.
        </p>

        <UiState v-if="zonesError" tone="error">
          {{ zonesError }}
        </UiState>

        <UiState v-else-if="settingsLoading" loading>
          Loading domains...
        </UiState>

        <UiState v-else-if="listedDomains.length === 0">
          No domains configured yet.
        </UiState>

        <div v-else class="networking-scroll-region networking-scroll-region--domains space-y-2">
          <NetworkingReadonlyRow
            :label="'Base domain'"
            :value="baseDomainValue || 'Unavailable'"
            :copyable="isCopyValueAllowed(baseDomainValue || '')"
            :copied="copiedFieldKey === 'domain-base'"
            @copy="copyReadonlyValue(baseDomainValue, 'Base domain', 'domain-base')"
          />

          <NetworkingReadonlyRow
            v-for="domain in secondaryDomains"
            :key="domain"
            :label="'Domain'"
            :value="domain"
            :copyable="isCopyValueAllowed(domain)"
            :copied="copiedFieldKey === `domain-${domain}`"
            @copy="copyReadonlyValue(domain, 'Domain', `domain-${domain}`)"
          />
        </div>
      </UiPanel>
    </div>

    <UiPanel as="article" class="space-y-5 p-5 lg:p-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Ingress config
          </p>
          <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
            Cloudflared preview
          </h2>
        </div>

        <div class="flex items-center gap-2">
          <UiButton
            variant="ghost"
            size="xs"
            :disabled="previewLoading"
            @click="loadPreview"
          >
            <span class="flex items-center gap-2">
              <NavIcon name="refresh" class="h-3 w-3" />
              <UiInlineSpinner v-if="previewLoading" />
              Refresh config
            </span>
          </UiButton>

          <UiButton
            variant="ghost"
            size="xs"
            :disabled="!hasPreview"
            @click="copyPreviewContents"
          >
            Copy preview
          </UiButton>
        </div>
      </div>

      <NetworkingReadonlyRow
        label="Config path"
        :value="preview?.path || settings?.cloudflaredConfigPath || 'Unavailable'"
        :copyable="isCopyValueAllowed(preview?.path || settings?.cloudflaredConfigPath || '')"
        :copied="copiedFieldKey === 'preview-path'"
        @copy="copyReadonlyValue(preview?.path || settings?.cloudflaredConfigPath || '', 'Config path', 'preview-path')"
      />

      <UiState v-if="previewLoading" loading>
        Loading config preview...
      </UiState>

      <UiState v-else-if="previewError" tone="error">
        {{ previewError }}
      </UiState>

      <pre
        v-else-if="hasPreview"
        class="networking-preview-block"
      ><code>{{ preview?.contents }}</code></pre>

      <UiState v-else>
        Cloudflared config not loaded yet.
      </UiState>
    </UiPanel>
  </section>
</template>

<style scoped>
.networking-summary-card {
  display: flex;
  min-height: 5.6rem;
  flex-direction: column;
  justify-content: flex-start;
  gap: 0.35rem;
  padding: 0.7rem 0.9rem;
}

.networking-summary-card__head {
  display: flex;
  align-items: center;
}

.networking-summary-card__body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  min-height: 2rem;
}

.networking-summary-card__label {
  margin: 0;
  color: var(--muted-2);
  font-size: 0.62rem;
  letter-spacing: 0.26em;
  text-transform: uppercase;
}

.networking-summary-card__value {
  margin: 0;
  color: var(--text);
  font-size: clamp(1.05rem, 1.6vw, 1.38rem);
  font-weight: 600;
  letter-spacing: -0.01em;
  text-transform: capitalize;
}

.networking-summary-card__meta {
  margin: 0;
  color: var(--muted);
  font-size: 0.63rem;
  line-height: 1.3;
  max-width: 65%;
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.networking-source-lane {
  overflow-x: auto;
  padding-bottom: 0.18rem;
}

.networking-source-lane::-webkit-scrollbar {
  height: 0.42rem;
}

.networking-source-lane::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: color-mix(in srgb, var(--border) 75%, var(--accent) 25%);
}

.networking-source-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(10.5rem, 1fr));
  gap: 0.55rem;
  min-width: 56rem;
}

.networking-source-item {
  min-width: 0;
}

.networking-source-item:deep(.networking-readonly-row__value-wrap),
.networking-source-item:deep(.networking-readonly-row__value-static) {
  max-width: 100%;
}

.networking-density-card {
  min-height: clamp(23rem, 40vh, 28rem);
}

.networking-scroll-region {
  overflow-y: auto;
  padding-right: 0.2rem;
}

.networking-scroll-region--compact {
  max-height: 10.5rem;
}

.networking-scroll-region--checks {
  max-height: 14.6rem;
}

.networking-scroll-region--routes,
.networking-scroll-region--domains {
  max-height: 17.4rem;
}

.networking-scroll-region::-webkit-scrollbar {
  width: 0.44rem;
}

.networking-scroll-region::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: color-mix(in srgb, var(--border) 75%, var(--accent) 25%);
}

.networking-quad-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0.55rem;
}

.networking-quad-item {
  min-width: 0;
  border: 1px solid color-mix(in srgb, var(--border) 88%, var(--accent) 12%);
  background: color-mix(in srgb, var(--surface-2) 95%, var(--accent) 5%);
  border-radius: 5px;
  padding: 0.62rem 0.68rem;
  display: flex;
  flex-direction: column;
  gap: 0.36rem;
}

.networking-quad-item__label {
  margin: 0;
  color: var(--muted-2);
  font-size: 0.58rem;
  letter-spacing: 0.22em;
  text-transform: uppercase;
}

:deep(.networking-quad-item__value) {
  margin: 0;
  color: var(--text);
  font-size: 0.8rem;
  line-height: 1.25;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.networking-quad-copy-btn) {
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  font-size: 0.8rem;
  line-height: 1.25;
  text-align: left;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  transform: translateX(0);
  transition:
    color 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

:deep(.networking-quad-copy-btn:hover),
:deep(.networking-quad-copy-btn:focus-visible) {
  color: var(--accent-strong);
  transform: translateX(2px);
  outline: none;
}

.networking-quad-item--check {
  gap: 0.55rem;
}

.networking-check-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  min-width: 0;
}

.networking-check-label {
  display: inline-flex;
  align-items: center;
  gap: 0.36rem;
  min-width: 0;
  color: var(--muted-2);
  font-size: 0.58rem;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  white-space: nowrap;
}

.networking-check-status {
  color: var(--text);
  font-size: 0.66rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  white-space: nowrap;
}

.networking-check-detail {
  margin: 0;
  color: var(--muted);
  font-size: 0.71rem;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.networking-routes-head {
  grid-template-columns: minmax(0, 0.65fr) minmax(0, 1.42fr) minmax(0, 0.55fr) minmax(0, 1.2fr);
  gap: 0.75rem;
  padding: 0 0.35rem;
  color: var(--muted-2);
  font-size: 0.62rem;
  font-weight: 600;
  letter-spacing: 0.23em;
  text-transform: uppercase;
}

.networking-route-row {
  display: grid;
  grid-template-columns: minmax(0, 0.65fr) minmax(0, 1.42fr) minmax(0, 0.55fr) minmax(0, 1.2fr);
  gap: 0.75rem;
  border: 1px solid color-mix(in srgb, var(--border) 86%, var(--accent) 14%);
  background: color-mix(in srgb, var(--surface-2) 92%, var(--accent) 8%);
  border-radius: 5px;
  padding: 0.65rem 0.8rem;
}

.networking-route-cell {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.22rem;
  justify-content: center;
}

.networking-route-mobile-label {
  display: none;
  color: var(--muted-2);
  font-size: 0.58rem;
  letter-spacing: 0.22em;
  text-transform: uppercase;
}

:deep(.networking-route-copy-btn) {
  display: inline-flex;
  align-items: center;
  gap: 0;
  max-width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  font-size: 0.75rem;
  line-height: 1.35;
  cursor: pointer;
  transform: translateX(0);
  transition:
    color 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

:deep(.networking-route-copy-btn:hover),
:deep(.networking-route-copy-btn:focus-visible) {
  color: var(--accent-strong);
  transform: translateX(2px);
  outline: none;
}

:deep(.networking-route-copy-btn__text) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.networking-preview-block {
  max-height: min(34rem, 60vh);
  overflow: auto;
  margin: 0;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-inset) 90%, var(--surface-2) 10%);
  color: color-mix(in srgb, var(--accent-ink) 68%, var(--text) 32%);
  border-radius: 5px;
  padding: 0.95rem;
  font-size: 0.72rem;
  line-height: 1.55;
}

@media (max-width: 840px) {
  .networking-quad-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .networking-scroll-region--compact,
  .networking-scroll-region--checks,
  .networking-scroll-region--routes,
  .networking-scroll-region--domains {
    max-height: none;
  }

  .networking-route-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.58rem;
    padding: 0.7rem;
  }

  .networking-route-mobile-label {
    display: block;
  }

  :deep(.networking-route-copy-btn) {
    justify-content: flex-start;
    width: 100%;
  }
}
</style>
