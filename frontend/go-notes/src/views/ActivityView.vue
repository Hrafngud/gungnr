<script setup lang="ts">
import { onMounted } from 'vue'
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
  <section class="space-y-8">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
          Activity
        </p>
        <h1 class="mt-2 text-3xl font-semibold text-neutral-900">
          Audit timeline
        </h1>
        <p class="mt-2 text-sm text-neutral-600">
          Track who triggered deploy workflows and settings updates.
        </p>
      </div>
      <button
        type="button"
        class="inline-flex items-center justify-center rounded-2xl border border-black/10 bg-white/80 px-4 py-2 text-sm font-semibold text-neutral-700 transition hover:-translate-y-0.5"
        @click="auditStore.fetchLogs"
      >
        Refresh
      </button>
    </div>

    <div
      v-if="auditStore.error"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ auditStore.error }}
    </div>

    <div
      v-if="auditStore.loading"
      class="rounded-[28px] border border-dashed border-black/10 bg-white/70 p-6 text-sm text-neutral-500"
    >
      Loading audit entries from the panel API...
    </div>

    <div
      v-else-if="auditStore.logs.length === 0"
      class="rounded-[28px] border border-black/10 bg-white/80 p-6 text-sm text-neutral-600"
    >
      <p class="text-lg font-semibold text-neutral-900">No activity yet</p>
      <p class="mt-2">
        Once someone deploys a template or updates settings, the audit trail
        will appear here.
      </p>
    </div>

    <div v-else class="space-y-4">
      <article
        v-for="entry in auditStore.logs"
        :key="entry.id"
        class="rounded-[24px] border border-black/10 bg-white/90 p-5 shadow-[0_20px_50px_-40px_rgba(0,0,0,0.55)]"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-neutral-500">
              {{ entry.action }}
            </p>
            <h2 class="mt-1 text-lg font-semibold text-neutral-900">
              {{ entry.target || 'System action' }}
            </h2>
          </div>
          <div class="text-right text-xs text-neutral-500">
            <p>{{ new Date(entry.createdAt).toLocaleString() }}</p>
            <p class="mt-1 font-semibold text-neutral-700">
              {{ entry.userLogin || 'System' }}
            </p>
          </div>
        </div>

        <div
          v-if="entry.metadata"
          class="mt-4 rounded-2xl border border-black/10 bg-neutral-50 px-4 py-3 text-xs text-neutral-700"
        >
          <pre class="whitespace-pre-wrap font-mono">{{ formatMetadata(entry.metadata) }}</pre>
        </div>
      </article>
    </div>
  </section>
</template>
