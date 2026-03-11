<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchServiceInventoryRow } from '@/components/workbench/projectDetailWorkbenchTypes'
import { workbenchCompactToneClass } from '@/components/workbench/workbenchInspectorPresentation'

defineProps<{
  serviceInventory: WorkbenchServiceInventoryRow[]
  selectedServiceName: string
}>()

const emit = defineEmits<{
  select: [serviceName: string]
}>()
</script>

<template>
  <UiPanel
    variant="soft"
    class="workbench-shell-card workbench-shell-card--left space-y-4 p-4"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers</p>
        <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Container selector</h3>
      </div>
      <UiBadge :tone="serviceInventory.length > 0 ? 'ok' : 'neutral'">
        {{ serviceInventory.length }} tracked
      </UiBadge>
    </div>

    <UiState v-if="serviceInventory.length === 0">
      No Workbench service rows are stored for this snapshot yet.
    </UiState>
    <div v-else class="workbench-service-selector-list">
      <button
        v-for="service in serviceInventory"
        :key="service.serviceName"
        type="button"
        class="workbench-service-selector"
        :aria-pressed="selectedServiceName === service.serviceName"
        :class="{ 'workbench-service-selector--active': selectedServiceName === service.serviceName }"
        @click="emit('select', service.serviceName)"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="workbench-service-selector__eyebrow">Service</p>
            <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">{{ service.serviceName }}</h4>
            <p class="mt-2 text-xs text-[color:var(--muted)]">
              {{
                service.image ||
                service.buildSource ||
                service.restartPolicy ||
                'Stored compose service'
              }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <UiBadge :tone="selectedServiceName === service.serviceName ? 'ok' : 'neutral'">
              {{ selectedServiceName === service.serviceName ? 'Selected' : 'Select' }}
            </UiBadge>
            <span :class="['workbench-compact-status', workbenchCompactToneClass(service.originTone)]">
              <span class="workbench-compact-status__dot" />
              {{ service.originLabel }}
            </span>
          </div>
        </div>

        <div class="flex flex-wrap gap-2 text-[11px]">
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">{{ service.portCount }}</span>
            <span>ports</span>
          </span>
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">{{ service.dependencies.length }}</span>
            <span>deps</span>
          </span>
          <span class="workbench-compact-metric">
            <span class="workbench-compact-metric__value">{{ service.networkCount }}</span>
            <span>networks</span>
          </span>
          <span
            v-if="service.restartPolicy"
            class="workbench-service-chip"
          >
            restart {{ service.restartPolicy }}
          </span>
        </div>
      </button>
    </div>
  </UiPanel>
</template>

