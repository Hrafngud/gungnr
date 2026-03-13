<script setup lang="ts">
import UiState from '@/components/ui/UiState.vue'
import WorkbenchDependencyGraph from '@/components/workbench/WorkbenchDependencyGraph.vue'
import type { WorkbenchTopologyInventoryRow } from '@/components/workbench/projectDetailWorkbenchTypes'

defineProps<{
  selectedServiceTopology: WorkbenchTopologyInventoryRow | null
}>()
</script>

<template>
  <div class="flex w-full flex-col items-start border-b-2 border-zinc-700 mb-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Dependencies</p>
        <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">Graph</h4>
      </div>
    </div>

    <UiState v-if="!selectedServiceTopology">
      No Workbench topology rows are stored for this service yet.
    </UiState>
    <div v-else class="h-full w-full p-3 items-center gap-3">
      <WorkbenchDependencyGraph
        :service-name="selectedServiceTopology.serviceName"
        :depends-on="selectedServiceTopology.dependsOn"
        :depended-by="selectedServiceTopology.dependedBy"
      />
    </div>
  </div>
</template>
