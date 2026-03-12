<script setup lang="ts">
import UiPanel from '@/components/ui/UiPanel.vue'
import type {
  WorkbenchPortInventoryRow,
  WorkbenchServiceInventoryRow,
  WorkbenchTopologyInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
defineProps<{
  selectedService: WorkbenchServiceInventoryRow
  selectedServiceTopology: WorkbenchTopologyInventoryRow | null
  selectedServicePorts: WorkbenchPortInventoryRow[]
}>()
</script>

<template>
  <div class="workbench-inspector-section p-6">
    <div class="flex flex-row items-center justify-between">
      <div class="flex flex-row items-center justify-between w-full">
        <span class="text-sm"> Service name:</span>
        <h4 class="mt-1 text-xl font-semibold text-[color:var(--text)]">
          {{ selectedService.serviceName }}
        </h4>
      </div>
    </div>

    <div class="flex flex-row justify-between gap-2">
      <UiPanel variant="raise" class="h-24 w-full p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Image</p>
        <p class="text-sm text-[color:var(--text)] w-full">
          {{ selectedService.image || 'Not declared' }}
        </p>
      </UiPanel>
      <UiPanel variant="raise" class="w-full p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Build source</p>
        <p class="text-sm text-[color:var(--text)] break-all">
          {{ selectedService.buildSource || 'Not declared' }}
        </p>
      </UiPanel>
      <UiPanel variant="raise" class="w-full p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Restart policy</p>
        <p class="text-sm text-[color:var(--text)]">
          {{ selectedService.restartPolicy || 'Default compose behavior' }}
        </p>
      </UiPanel>
    </div>
  </div>
</template>

