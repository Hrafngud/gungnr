<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import NavIcon from '@/components/NavIcon.vue'
import type { BadgeTone } from '@/components/workbench/projectDetailWorkbenchTypes'
import type { ProjectContainer } from '@/types/projects'

const props = defineProps<{
  containers: ProjectContainer[]
  projectStatus: string
  isAdmin: boolean
  stackRestarting: boolean
  stackRestartError: string | null
}>()

const emit = defineEmits<{
  restartStack: []
}>()

function containerTone(container: ProjectContainer): BadgeTone {
  const normalized = container.status.trim().toLowerCase()
  if (normalized.startsWith('up') || normalized.includes('running')) return 'ok'
  if (normalized.startsWith('exited') || normalized.includes('dead')) return 'error'
  if (normalized.includes('paused') || normalized.includes('restarting')) return 'warn'
  return 'neutral'
}

function projectStatusTone(status: string): BadgeTone {
  const normalized = status.trim().toLowerCase()
  if (!normalized) return 'neutral'
  if (normalized === 'running' || normalized === 'up' || normalized.includes('running')) return 'ok'
  if (normalized.includes('failed') || normalized.includes('error')) return 'error'
  if (normalized.includes('pending') || normalized.includes('building')) return 'warn'
  return 'neutral'
}
</script>

<template>
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">
          Runtime units ({{ containers.length }})
        </h2>
      </div>
      <div class="rounded-xl border border-[color:var(--line)] bg-[color:var(--panel)] p-3">
        <div class="flex flex-wrap items-center gap-2">
          <UiButton
            variant="ghost"
            size="sm"
            :disabled="stackRestarting || !isAdmin"
            @click="emit('restartStack')"
          >
            <span class="inline-flex items-center gap-2">
              <NavIcon name="restart" class="h-3.5 w-3.5" />
              <UiInlineSpinner v-if="stackRestarting" />
              {{ stackRestarting ? 'Restarting stack...' : 'Restart stack' }}
            </span>
          </UiButton>
          <UiBadge :tone="projectStatusTone(projectStatus)">
            {{ projectStatus || 'unknown' }}
          </UiBadge>
        </div>
      </div>
    </div>
    <UiInlineFeedback v-if="props.stackRestartError" tone="error">
      {{ props.stackRestartError }}
    </UiInlineFeedback>
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
