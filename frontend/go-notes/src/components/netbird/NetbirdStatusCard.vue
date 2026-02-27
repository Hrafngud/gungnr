<script setup lang="ts">
import { computed, onMounted } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetbirdStore } from '@/stores/netbird'
import type { NetBirdMode } from '@/types/netbird'

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'
type FeedbackTone = 'neutral' | 'ok' | 'warn' | 'error'

const authStore = useAuthStore()
const netbirdStore = useNetbirdStore()

const isAdmin = computed(() => authStore.isAdmin)
const status = computed(() => netbirdStore.status.data)
const statusLoading = computed(() => netbirdStore.status.loading)
const statusError = computed(() => netbirdStore.status.error)
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

const boolTone = (value: boolean): BadgeTone => (value ? 'ok' : 'error')
const boolLabel = (value: boolean) => (value ? 'Yes' : 'No')

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

onMounted(() => {
  if (!status.value) {
    void netbirdStore.loadStatus()
  }
})
</script>

<template>
  <UiPanel as="article" class="space-y-4 p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          NetBird
        </p>
        <h2 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          Status
        </h2>
        <p class="mt-1 text-xs text-[color:var(--muted)]">
          Connectivity, peer identity, policy sync, and managed ACL visibility.
        </p>
      </div>
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

    <div v-else-if="status" class="grid gap-4 lg:grid-cols-2">
      <UiPanel variant="soft" class="space-y-3 p-3">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Connectivity
        </p>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Client installed</span>
          <UiBadge :tone="boolTone(status.clientInstalled)">
            {{ boolLabel(status.clientInstalled) }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Daemon running</span>
          <UiBadge :tone="boolTone(status.daemonRunning)">
            {{ boolLabel(status.daemonRunning) }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Connected</span>
          <UiBadge :tone="boolTone(status.connected)">
            {{ boolLabel(status.connected) }}
          </UiBadge>
        </UiListRow>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-3">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Peer
        </p>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Peer name</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.peerName || 'n/a' }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Peer ID</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.peerId || 'n/a' }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">wg0 IP</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.wg0Ip || 'n/a' }}</span>
        </UiListRow>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-3">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Mode and Sync
        </p>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Effective mode</span>
          <UiBadge tone="neutral">
            {{ modeLabel(status.currentMode) }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Configured mode</span>
          <UiBadge :tone="status.modeDrift ? 'warn' : 'neutral'">
            {{ modeLabel(status.configuredMode) }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Mode source</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.modeSource || 'n/a' }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Last policy sync</span>
          <span class="text-xs text-[color:var(--text)]">{{ formatDate(status.lastPolicySyncAt) }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Sync status</span>
          <UiBadge :tone="syncTone(status.lastPolicySyncStatus)">
            {{ status.lastPolicySyncStatus || 'unknown' }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Warning count</span>
          <UiBadge :tone="warningTone">
            {{ status.lastPolicySyncWarnings }}
          </UiBadge>
        </UiListRow>
      </UiPanel>

      <UiPanel variant="soft" class="space-y-3 p-3">
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Managed Summary
        </p>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Managed groups</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.managedGroups }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Managed policies</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.managedPolicies }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Count source</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.managedCountSource }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">API reachable</span>
          <UiBadge :tone="status.apiReachable ? 'ok' : 'warn'">
            {{ status.apiReachable ? 'Reachable' : 'Unavailable' }}
          </UiBadge>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Reachability source</span>
          <span class="text-xs text-[color:var(--text)]">{{ status.apiReachability.source || 'n/a' }}</span>
        </UiListRow>
        <UiListRow class="flex items-center justify-between gap-3">
          <span class="text-xs text-[color:var(--muted)]">Reachability checked</span>
          <span class="text-xs text-[color:var(--text)]">
            {{ formatDate(status.apiReachability.checkedAt) }}
          </span>
        </UiListRow>
        <UiListRow v-if="status.apiReachability.message" class="flex flex-wrap gap-2 text-xs text-[color:var(--muted)]">
          {{ status.apiReachability.message }}
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
</template>
