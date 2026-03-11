<script setup lang="ts">
import UiPanel from '@/components/ui/UiPanel.vue'
import type {
  WorkbenchPortInventoryRow,
  WorkbenchServiceInventoryRow,
  WorkbenchTopologyInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import { workbenchCompactToneClass } from '@/components/workbench/workbenchInspectorPresentation'

defineProps<{
  selectedService: WorkbenchServiceInventoryRow
  selectedServiceTopology: WorkbenchTopologyInventoryRow | null
  selectedServicePorts: WorkbenchPortInventoryRow[]
}>()
</script>

<template>
  <div class="workbench-inspector-section">
    <div class="workbench-inspector-hero">
      <div class="min-w-0">
        <p class="workbench-service-selector__eyebrow">Service</p>
        <h4 class="mt-1 text-xl font-semibold text-[color:var(--text)]">
          {{ selectedService.serviceName }}
        </h4>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          {{
            selectedService.image ||
            selectedService.buildSource ||
            'Compose-defined source with no explicit image or build path'
          }}
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedService.originTone)]">
          <span class="workbench-compact-status__dot" />
          {{ selectedService.originLabel }}
        </span>
        <span
          v-if="selectedService.managedEntryKeys.length > 0"
          class="workbench-service-chip"
        >
          {{ selectedService.managedEntryKeys.length }} managed
        </span>
        <span
          v-if="selectedService.legacyModuleTypes.length > 0"
          class="workbench-service-chip"
        >
          {{ selectedService.legacyModuleTypes.length }} legacy
        </span>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <UiPanel variant="raise" class="space-y-1 p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Image</p>
        <p class="text-sm text-[color:var(--text)] break-all">
          {{ selectedService.image || 'Not declared' }}
        </p>
      </UiPanel>
      <UiPanel variant="raise" class="space-y-1 p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Build source</p>
        <p class="text-sm text-[color:var(--text)] break-all">
          {{ selectedService.buildSource || 'Not declared' }}
        </p>
      </UiPanel>
      <UiPanel variant="raise" class="space-y-1 p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Restart policy</p>
        <p class="text-sm text-[color:var(--text)]">
          {{ selectedService.restartPolicy || 'Default compose behavior' }}
        </p>
      </UiPanel>
      <UiPanel variant="raise" class="space-y-1 p-3">
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Inventory</p>
        <div class="flex flex-wrap gap-2">
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">{{ selectedServicePorts.length }}</span>
            <span>ports</span>
          </span>
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">
              {{ selectedServiceTopology?.networkNames.length ?? selectedService.networkCount }}
            </span>
            <span>networks</span>
          </span>
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">
              {{
                (selectedServiceTopology?.dependsOn.length ?? 0) +
                (selectedServiceTopology?.dependedBy.length ?? 0)
              }}
            </span>
            <span>links</span>
          </span>
        </div>
      </UiPanel>
    </div>
  </div>
</template>

