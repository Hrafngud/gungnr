<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { BadgeTone } from '@/components/workbench/projectDetailWorkbenchTypes'
import type { ProjectContainer } from '@/types/projects'

defineProps<{
  containers: ProjectContainer[]
}>()

function containerTone(container: ProjectContainer): BadgeTone {
  const normalized = container.status.trim().toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  if (normalized.includes('paused') || normalized.includes('restarting')) return 'warn'
  return 'neutral'
}
</script>

<template>
  <UiPanel class="space-y-5 p-6">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
      <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Runtime units ({{ containers.length }})</h2>
    </div>
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
      </UiListRow>
    </div>
  </UiPanel>
</template>
