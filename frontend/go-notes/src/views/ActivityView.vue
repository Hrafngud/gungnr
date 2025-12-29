<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useAuditStore } from '@/stores/audit'

const auditStore = useAuditStore()

onMounted(() => {
  if (!auditStore.initialized) {
    auditStore.fetchLogs()
  }
})

const formatMetadata = (raw: string) => {
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}
</script>

<template>
  <section class="page space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Activity
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-[color:var(--text)]">
          Audit timeline
        </h1>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Track who triggered deploy workflows and settings updates.
        </p>
      </div>
      <UiButton
        variant="ghost"
        size="sm"
        :disabled="auditStore.loading"
        @click="auditStore.fetchLogs"
      >
        <span class="flex items-center gap-2">
          <UiInlineSpinner v-if="auditStore.loading" />
          Refresh
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
          Confirm each deploy or settings change shows up here, then cross-check the Jobs log.
        </p>
      </div>
      <UiButton :as="RouterLink" to="/jobs" variant="ghost" size="sm">
        Open jobs
      </UiButton>
    </UiPanel>

    <UiState v-if="auditStore.error" tone="error">
      {{ auditStore.error }}
    </UiState>

    <UiState v-else-if="auditStore.loading" loading>
      Loading audit entries from the panel API...
    </UiState>

    <UiState v-else-if="auditStore.logs.length === 0">
      <p class="text-lg font-semibold text-[color:var(--text)]">No activity yet</p>
      <p class="mt-2">
        Once someone deploys a template or updates settings, the audit trail
        will appear here.
      </p>
    </UiState>

    <div v-else class="space-y-4">
      <UiListRow
        v-for="entry in auditStore.logs"
        :key="entry.id"
        as="article"
        class="space-y-4"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              {{ entry.action }}
            </p>
            <h2 class="mt-1 text-lg font-semibold text-[color:var(--text)]">
              {{ entry.target || 'System action' }}
            </h2>
          </div>
          <div class="text-right text-xs text-[color:var(--muted)]">
            <p>{{ new Date(entry.createdAt).toLocaleString() }}</p>
            <p class="mt-1 font-semibold text-[color:var(--text)]">
              {{ entry.userLogin || 'System' }}
            </p>
            <UiBadge tone="neutral" class="mt-2 inline-flex">
              Audit
            </UiBadge>
          </div>
        </div>

        <UiPanel
          v-if="entry.metadata"
          variant="soft"
          class="p-4 text-xs text-[color:var(--muted)]"
        >
          <pre class="whitespace-pre-wrap font-mono">{{ formatMetadata(entry.metadata) }}</pre>
        </UiPanel>
      </UiListRow>
    </div>
  </section>
</template>
