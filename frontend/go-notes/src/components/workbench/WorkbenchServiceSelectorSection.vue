<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchServiceInventoryRow } from '@/components/workbench/projectDetailWorkbenchTypes'

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
    class="space-y-5 p-6"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Containers</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Container selector</h2>
      </div>
      <UiBadge :tone="serviceInventory.length > 0 ? 'ok' : 'neutral'">
        {{ serviceInventory.length }} tracked
      </UiBadge>
    </div>

    <UiState v-if="serviceInventory.length === 0">
      No Workbench service rows are stored for this snapshot yet.
    </UiState>
    <div v-else class="grid gap-4 md:grid-cols-2">
      <UiListRow
        v-for="service in serviceInventory"
        :key="service.serviceName"
        as="button"
        type="button"
        class="workbench-service-selector-card text-left"
        :aria-pressed="selectedServiceName === service.serviceName"
        :class="{ 'workbench-service-selector-card--active': selectedServiceName === service.serviceName }"
        @click="emit('select', service.serviceName)"
      >
        <div class="workbench-service-selector-card__main">
          <div class="workbench-service-selector-card__header">
            <h3 class="text-lg font-semibold text-[color:var(--text)]">
              {{ service.serviceName }}
            </h3>
            <p class="workbench-service-selector-card__origin text-xs uppercase tracking-[0.2em] text-[color:var(--muted)]">
              {{ service.originLabel }}
            </p>
          </div>
          <p class="workbench-service-selector-card__meta text-xs text-[color:var(--muted)]">
            {{
              service.image ||
              service.buildSource ||
              service.restartPolicy ||
              'Stored compose service'
            }}
          </p>
        </div>

        <div class="workbench-service-selector-card__stats text-xs text-[color:var(--muted)]">
          <div class="workbench-service-selector-card__stat-row">
            <span>Dependencies</span>
            <span class="text-[color:var(--text)]">{{ service.dependencies.length }}</span>
          </div>
          <div class="workbench-service-selector-card__stat-row">
            <span>Networks</span>
            <span class="text-[color:var(--text)]">{{ service.networkCount }}</span>
          </div>
          <div class="workbench-service-selector-card__stat-row">
            <span>Restart</span>
            <span class="text-[color:var(--text)] break-all text-right">{{ service.restartPolicy || '—' }}</span>
          </div>
        </div>
      </UiListRow>
    </div>
  </UiPanel>
</template>

<style scoped>
.workbench-service-selector-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 0.9rem;
  width: 100%;
  min-height: 12rem;
  border: 1px solid color-mix(in srgb, var(--border) 90%, transparent);
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease;
}

.workbench-service-selector-card:hover {
  border-color: color-mix(in srgb, var(--accent) 60%, var(--border));
}

.workbench-service-selector-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 75%, white);
  outline-offset: 2px;
}

.workbench-service-selector-card--active {
  border-color: color-mix(in srgb, var(--accent) 70%, var(--border));
  background: color-mix(in srgb, var(--accent) 10%, var(--surface-2));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 18%, transparent);
}

.workbench-service-selector-card__main {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  min-width: 0;
}

.workbench-service-selector-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  min-width: 0;
}

.workbench-service-selector-card__origin {
  flex-shrink: 0;
  text-align: right;
  line-height: 1.35;
}

.workbench-service-selector-card__meta {
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  min-height: 2.25em;
}

.workbench-service-selector-card__stats {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  margin-top: auto;
}

.workbench-service-selector-card__stat-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 1.15rem;
}

@media (max-width: 640px) {
  .workbench-service-selector-card {
    min-height: 11rem;
  }
}
</style>
