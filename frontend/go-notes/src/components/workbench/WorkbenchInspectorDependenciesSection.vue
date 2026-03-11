<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiState from '@/components/ui/UiState.vue'
import WorkbenchDependencyGraph from '@/components/workbench/WorkbenchDependencyGraph.vue'
import type { WorkbenchTopologyInventoryRow } from '@/components/workbench/projectDetailWorkbenchTypes'

defineProps<{
  selectedServiceTopology: WorkbenchTopologyInventoryRow | null
}>()
</script>

<template>
  <div class="workbench-inspector-section">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Dependencies</p>
        <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">Graph</h4>
      </div>
      <UiBadge :tone="selectedServiceTopology ? 'ok' : 'neutral'">
        {{
          selectedServiceTopology &&
          (selectedServiceTopology.dependsOn.length > 0 ||
            selectedServiceTopology.dependedBy.length > 0)
            ? 'Connected'
            : 'Isolated'
        }}
      </UiBadge>
    </div>

    <UiState v-if="!selectedServiceTopology">
      No Workbench topology rows are stored for this service yet.
    </UiState>
    <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
      <WorkbenchDependencyGraph
        :service-name="selectedServiceTopology.serviceName"
        :depends-on="selectedServiceTopology.dependsOn"
        :depended-by="selectedServiceTopology.dependedBy"
      />
      <div class="grid gap-2 sm:grid-cols-2">
        <div class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
          <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Networks</p>
          <p class="mt-2 text-[color:var(--text)]">
            {{
              selectedServiceTopology.networkNames.length > 0
                ? selectedServiceTopology.networkNames.join(', ')
                : 'None'
            }}
          </p>
        </div>
        <div class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
          <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Legacy metadata
          </p>
          <p class="mt-2 text-[color:var(--text)]">
            {{
              selectedServiceTopology.moduleTypes.length > 0
                ? selectedServiceTopology.moduleTypes.join(', ')
                : 'None'
            }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

