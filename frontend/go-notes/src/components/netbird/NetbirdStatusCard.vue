<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCopyableValue from '@/components/ui/UiCopyableValue.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiStatusDot from '@/components/ui/UiStatusDot.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetbirdStore } from '@/stores/netbird'
import { useToastStore } from '@/stores/toasts'
import { isCopyValueAllowed, writeTextToClipboard } from '@/utils/clipboard'
import type { NetBirdMode } from '@/types/netbird'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'
type FeedbackTone = 'neutral' | 'ok' | 'warn' | 'error'

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()
const toastStore = useToastStore()
const copiedPeerFieldKey = ref<string | null>(null)
const showWarningsModal = ref(false)

let copiedPeerFieldTimer: ReturnType<typeof setTimeout> | null = null

const isAdmin = computed(() => authStore.isAdmin)
const status = computed(() => netbirdStore.status.data)
const statusLoading = computed(() => netbirdStore.status.loading)
const statusError = computed(() => netbirdStore.status.error)
const aclGraph = computed(() => netbirdStore.aclGraph.data)
const aclGraphLoading = computed(() => netbirdStore.aclGraph.loading)
const aclGraphError = computed(() => netbirdStore.aclGraph.error)
const aclWarnings = computed(() => aclGraph.value?.notes ?? [])
const reapplySubmitting = computed(() => netbirdStore.policyReapply.submitting)
const reapplyError = computed(() => netbirdStore.policyReapply.error)
const reapplySummary = computed(() => netbirdStore.policyReapply.summary)

const modeLabel = (mode: NetBirdMode) => {
  if (mode === 'mode_a') return 'Mode A'
  if (mode === 'mode_b') return 'Mode B'
  return 'Legacy'
}

const formatDate = (value?: string) => {
  if (!value) return 'n/a'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'n/a'
  return date.toLocaleString()
}

const syncTone = (value: string): BadgeTone => {
  const normalized = value.trim().toLowerCase()
  if (!normalized) return 'neutral'
  if (normalized.includes('success') || normalized.includes('complete') || normalized === 'ok') {
    return 'ok'
  }
  if (normalized.includes('warn')) return 'warn'
  if (normalized.includes('fail') || normalized.includes('error')) return 'error'
  return 'neutral'
}

const boolStatusDotTone = (value: boolean): BadgeTone => (value ? 'ok' : 'error')

const boolStatusLabel = (value: boolean) => (value ? 'Yes' : 'No')

const warningTone = computed<BadgeTone>(() => {
  if (!status.value) return 'neutral'
  return status.value.lastPolicySyncWarnings > 0 ? 'warn' : 'ok'
})

const reapplySummaryTone = computed<FeedbackTone>(() => {
  if (!reapplySummary.value) return 'neutral'
  return reapplySummary.value.warnings.length > 0 ? 'warn' : 'ok'
})

const reapplySummaryMessage = computed(() => {
  if (!reapplySummary.value) return ''
  return [
    `Policies reapplied for ${modeLabel(reapplySummary.value.currentMode)}.`,
    `Groups: ${reapplySummary.value.groupResultCounts.updated} updated, ${reapplySummary.value.groupResultCounts.created} created, ${reapplySummary.value.groupResultCounts.deleted} deleted.`,
    `Policies: ${reapplySummary.value.policyResultCounts.updated} updated, ${reapplySummary.value.policyResultCounts.created} created, ${reapplySummary.value.policyResultCounts.deleted} deleted.`,
    reapplySummary.value.warnings.length > 0
      ? `${reapplySummary.value.warnings.length} warning(s) reported.`
      : 'No warnings reported.',
  ].join(' ')
})

const refreshStatus = async () => {
  await netbirdStore.loadStatus()
}

const triggerReapply = async () => {
  if (!isAdmin.value) return
  await netbirdStore.reapplyPolicies()
  await netbirdStore.loadStatus()
}

const openWarningsModal = () => {
  showWarningsModal.value = true
  if (!aclGraph.value && !aclGraphLoading.value) {
    void netbirdStore.loadAclGraph()
  }
}

const copyPeerValue = async (
  payload: string | null | undefined,
  label: string,
  fieldKey: string,
) => {
  const value = payload?.trim() ?? ''
  if (!isCopyValueAllowed(value)) {
    toastStore.warn(`${label} is not available on this host.`, 'Copy value')
    return
  }

  try {
    await writeTextToClipboard(value)
    copiedPeerFieldKey.value = fieldKey

    if (copiedPeerFieldTimer) clearTimeout(copiedPeerFieldTimer)
    copiedPeerFieldTimer = setTimeout(() => {
      copiedPeerFieldKey.value = null
      copiedPeerFieldTimer = null
    }, 1500)

    toastStore.success(`${label} copied to clipboard.`, 'Copy value')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

onMounted(() => {
  if (!status.value) {
    void netbirdStore.loadStatus()
  }
})

onBeforeUnmount(() => {
  if (!copiedPeerFieldTimer) return
  clearTimeout(copiedPeerFieldTimer)
  copiedPeerFieldTimer = null
})
</script>

<template>
  <UiPanel as="article" class="space-y-4 p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
        <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          Status
        </h2>
        <p class="mt-1 text-xs text-[color:var(--muted)]">
          Connectivity, peer identity, policy sync, and managed ACL visibility.
        </p>      
    </div>

    <UiState v-if="statusError" tone="error">
      {{ statusError }}
    </UiState>

    <UiState v-if="!isAdmin" tone="warn">
      Read-only access: admin permissions are required to reapply policies.
    </UiState>

    <UiPanel v-if="statusLoading && !status" variant="soft" class="space-y-3 p-4">
      <UiSkeleton class="h-3 w-36" />
      <UiSkeleton class="h-3 w-full" />
      <UiSkeleton class="h-3 w-2/3" />
      <UiSkeleton class="h-3 w-3/4" />
    </UiPanel>

    <div v-else-if="status" class="flex flex-col gap-2 w-full">

      <div class="flex flex-row items-center justify-between">
      <UiPanel variant="soft" class="flex flex-row justify-between">
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Client installed</span>
          <span class="inline-flex items-center" role="status" :aria-label="boolStatusLabel(status.clientInstalled)">
            <UiStatusDot :tone="boolStatusDotTone(status.clientInstalled)" />
            <span class="sr-only">{{ boolStatusLabel(status.clientInstalled) }}</span>
          </span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Daemon running</span>
          <span class="inline-flex items-center" role="status" :aria-label="boolStatusLabel(status.daemonRunning)">
            <UiStatusDot :tone="boolStatusDotTone(status.daemonRunning)" />
            <span class="sr-only">{{ boolStatusLabel(status.daemonRunning) }}</span>
          </span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Connected</span>
          <span class="inline-flex items-center" role="status" :aria-label="boolStatusLabel(status.connected)">
            <UiStatusDot :tone="boolStatusDotTone(status.connected)" />
            <span class="sr-only">{{ boolStatusLabel(status.connected) }}</span>
          </span>
        </UiListRow>
      </UiPanel>
      <div class="flex flex-wrap items-center gap-2">
        <UiButton variant="ghost" size="sm" :disabled="statusLoading || reapplySubmitting" @click="refreshStatus">
          <span class="flex items-center gap-2">
            <NavIcon name="refresh" class="h-3.5 w-3.5" />
            <UiInlineSpinner v-if="statusLoading" />
            Refresh
          </span>
        </UiButton>
        <UiButton
          v-if="isAdmin"
          variant="primary"
          size="sm"
          :disabled="reapplySubmitting"
          @click="triggerReapply"
        >
          <span class="flex items-center gap-2">
            <UiInlineSpinner v-if="reapplySubmitting" />
            {{ reapplySubmitting ? 'Reapplying...' : 'Reapply policies' }}
          </span>
        </UiButton>
      </div>
      <UiPanel variant="soft" class="flex flex-row justify-between">
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Peer name</span>
          <UiCopyableValue
            :value="status.peerName || 'n/a'"
            :copyable="isCopyValueAllowed(status.peerName || '')"
            :copied="copiedPeerFieldKey === 'peer-name'"
            button-class="text-xs text-[color:var(--text)]"
            static-class="text-xs text-[color:var(--text)]"
            @copy="copyPeerValue(status.peerName, 'Peer name', 'peer-name')"
          />
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Peer ID</span>
          <UiCopyableValue
            :value="status.peerId || 'n/a'"
            :copyable="isCopyValueAllowed(status.peerId || '')"
            :copied="copiedPeerFieldKey === 'peer-id'"
            button-class="text-xs text-[color:var(--text)]"
            static-class="text-xs text-[color:var(--text)]"
            @copy="copyPeerValue(status.peerId, 'Peer ID', 'peer-id')"
          />
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">wg0 IP</span>
          <UiCopyableValue
            :value="status.wg0Ip || 'n/a'"
            :copyable="isCopyValueAllowed(status.wg0Ip || '')"
            :copied="copiedPeerFieldKey === 'wg0-ip'"
            button-class="text-xs text-[color:var(--text)]"
            static-class="text-xs text-[color:var(--text)]"
            @copy="copyPeerValue(status.wg0Ip, 'wg0 IP', 'wg0-ip')"
          />
        </UiListRow>
      </UiPanel>
      </div>

      <UiPanel variant="soft" class="flex flex-wrap items-stretch gap-2 p-2">
        <UiListRow variant="card" class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">Netbird mode</span>
          <UiBadge tone="neutral">
            {{ modeLabel(status.currentMode) }}
          </UiBadge>
        </UiListRow>
        <UiListRow
          v-if="status.currentMode === 'mode_b' || status.configuredMode === 'mode_b'"
          variant="card"
          class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center"
        >
          <span class="text-xs text-[color:var(--muted)]">Project networks</span>
          <div class="text-xs text-[color:var(--text)]">
            {{ status.effectiveModeBProjectIds?.length ?? 0 }}
          </div>
        </UiListRow>
        <UiListRow variant="card" class="flex min-w-[210px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">Last policy sync</span>
          <span class="text-xs text-[color:var(--text)]">{{ formatDate(status.lastPolicySyncAt) }}</span>
        </UiListRow>
        <UiListRow variant="card" class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">Sync status</span>
          <UiBadge :tone="syncTone(status.lastPolicySyncStatus)">
            {{ status.lastPolicySyncStatus || 'unknown' }}
          </UiBadge>
        </UiListRow>
        <UiListRow
          as="button"
          type="button"
          variant="card"
          class="flex min-w-[130px] flex-1 cursor-pointer flex-col items-center justify-between gap-1 text-center transition hover:border-[color:var(--accent)] hover:shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--accent)]"
          @click="openWarningsModal"
        >
          <span class="text-xs text-[color:var(--muted)]">Warnings</span>
          <UiBadge :tone="warningTone">
            {{ status.lastPolicySyncWarnings }}
          </UiBadge>
        </UiListRow>
        <UiListRow variant="card" class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">Groups</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.managedGroups }}</span>
        </UiListRow>
        <UiListRow variant="card" class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">Policies</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.managedPolicies }}</span>
        </UiListRow>
        <UiListRow variant="card" class="flex min-w-[130px] flex-1 flex-col items-center justify-between gap-1 text-center">
          <span class="text-xs text-[color:var(--muted)]">API Status</span>
          <UiBadge :tone="status.apiReachable ? 'ok' : 'warn'">
            {{ status.apiReachable ? 'Reachable' : 'Unavailable' }}
          </UiBadge>
        </UiListRow>
      </UiPanel>
    </div>

    <UiState v-if="status && status.modeDrift" tone="warn">
      Runtime mode was restored from the latest successful apply and differs from configured mode.
    </UiState>

    <UiState v-else-if="!status" tone="neutral">
      NetBird status is not available yet.
    </UiState>

    <UiState v-if="reapplyError" tone="error">
      {{ reapplyError }}
    </UiState>
    <UiState v-if="reapplySummary" :tone="reapplySummaryTone">
      {{ reapplySummaryMessage }}
    </UiState>
  </UiPanel>

  <UiModal
    v-model="showWarningsModal"
    title="Notes and Warnings"
    :description="aclWarnings.length > 0 ? `${aclWarnings.length} warning(s) reported` : 'No warnings reported'"
  >
    <UiState v-if="aclGraphLoading && !aclGraph" loading>
      Loading warning details...
    </UiState>
    <UiState v-else-if="aclGraphError" tone="error">
      {{ aclGraphError }}
    </UiState>
    <UiState v-else-if="aclWarnings.length === 0" tone="ok">
      No backend notes or warnings were reported.
    </UiState>
    <ul v-else class="space-y-2">
      <li
        v-for="(note, index) in aclWarnings"
        :key="`netbird-status-note-${index}`"
        class="rounded border border-[color:var(--border)] bg-[color:var(--surface)] px-3 py-2 text-sm text-[color:var(--muted)]"
      >
        {{ note }}
      </li>
    </ul>
  </UiModal>
</template>
