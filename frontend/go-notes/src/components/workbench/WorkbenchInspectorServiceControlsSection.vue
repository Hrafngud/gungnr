<script setup lang="ts">
import UiState from '@/components/ui/UiState.vue'
import WorkbenchInspectorPortsSection from '@/components/workbench/WorkbenchInspectorPortsSection.vue'
import WorkbenchInspectorResourcesSection from '@/components/workbench/WorkbenchInspectorResourcesSection.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type { WorkbenchResourceField } from '@/types/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchPortInventoryRow,
  WorkbenchResourceEditorField,
  WorkbenchResourceInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import type { WorkbenchPortSuggestion } from '@/components/workbench/workbenchInspectorPresentation'

defineProps<{
  isAdmin: boolean
  optionalServiceMutationStatus: WorkbenchRequestStatus
  previewStatus: WorkbenchRequestStatus
  applyStatus: WorkbenchRequestStatus
  restoreStatus: WorkbenchRequestStatus
  resolveStatus: WorkbenchRequestStatus
  selectedServicePorts: WorkbenchPortInventoryRow[]
  selectedServiceResource: WorkbenchResourceInventoryRow | null
  resourceEditorFields: WorkbenchResourceEditorField[]
  portSuggestionResultByKey: Record<string, { suggestions?: WorkbenchPortSuggestion[] } | null | undefined>
  portInputValue: (port: WorkbenchPortInventoryRow) => string
  setPortInputValue: (key: string, value: string) => void
  portMutationBusy: (port: WorkbenchPortInventoryRow) => boolean
  setManualPort: (port: WorkbenchPortInventoryRow) => void
  resetPortToAuto: (port: WorkbenchPortInventoryRow) => void
  portSuggestionStatus: (port: WorkbenchPortInventoryRow) => WorkbenchRequestStatus
  loadPortSuggestions: (port: WorkbenchPortInventoryRow) => void
  portMutationFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
  portSuggestionFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
  resourceInputValue: (
    resource: WorkbenchResourceInventoryRow,
    field: WorkbenchResourceField,
  ) => string
  setResourceInputValue: (
    serviceName: string,
    field: WorkbenchResourceField,
    value: string,
  ) => void
  resourceActionDisabled: (resource: WorkbenchResourceInventoryRow) => boolean
  clearResourceFields: (
    resource: WorkbenchResourceInventoryRow,
    fields: WorkbenchResourceField[],
  ) => void
  saveResource: (resource: WorkbenchResourceInventoryRow) => void
  resourceMutationBusy: (resource: WorkbenchResourceInventoryRow) => boolean
  resetResourceInputs: (resource: WorkbenchResourceInventoryRow) => void
  resourceMutationFeedback: (
    resource: WorkbenchResourceInventoryRow,
  ) => WorkbenchInlineFeedbackState | null
}>()
</script>

<template>
  <div class="workbench-inspector-section workbench-inspector-section--editing">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Service controls</p>
        <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">Ports and budgets</h4>
      </div>
      <div class="flex flex-wrap gap-2">
        <span class="workbench-compact-metric">
          <span class="workbench-compact-metric__value">{{ selectedServicePorts.length }}</span>
          <span>ports</span>
        </span>
        <span class="workbench-compact-metric">
          <span class="workbench-compact-metric__value">
            {{ selectedServiceResource?.hasLimits ? 1 : 0 }}
          </span>
          <span>limits</span>
        </span>
        <span class="workbench-compact-metric">
          <span class="workbench-compact-metric__value">
            {{ selectedServiceResource?.hasReservations ? 1 : 0 }}
          </span>
          <span>reservations</span>
        </span>
      </div>
    </div>

    <p class="text-xs text-[color:var(--muted)]">
      Current values and supported edits stay together here. Host-port allocation still prefers the compose-declared host port before the next free fallback.
    </p>

    <UiState v-if="!isAdmin" tone="neutral">
      Read-only visibility: admin permissions are required to change host ports or resource budgets.
    </UiState>

    <div class="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]">
      <WorkbenchInspectorPortsSection
        :is-admin="isAdmin"
        :optional-service-mutation-status="optionalServiceMutationStatus"
        :preview-status="previewStatus"
        :apply-status="applyStatus"
        :restore-status="restoreStatus"
        :resolve-status="resolveStatus"
        :selected-service-ports="selectedServicePorts"
        :port-suggestion-result-by-key="portSuggestionResultByKey"
        :port-input-value="portInputValue"
        :set-port-input-value="setPortInputValue"
        :port-mutation-busy="portMutationBusy"
        :set-manual-port="setManualPort"
        :reset-port-to-auto="resetPortToAuto"
        :port-suggestion-status="portSuggestionStatus"
        :load-port-suggestions="loadPortSuggestions"
        :port-mutation-feedback="portMutationFeedback"
        :port-suggestion-feedback="portSuggestionFeedback"
      />
      <WorkbenchInspectorResourcesSection
        :is-admin="isAdmin"
        :selected-service-resource="selectedServiceResource"
        :resource-editor-fields="resourceEditorFields"
        :resource-input-value="resourceInputValue"
        :set-resource-input-value="setResourceInputValue"
        :resource-action-disabled="resourceActionDisabled"
        :clear-resource-fields="clearResourceFields"
        :save-resource="saveResource"
        :resource-mutation-busy="resourceMutationBusy"
        :reset-resource-inputs="resetResourceInputs"
        :resource-mutation-feedback="resourceMutationFeedback"
      />
    </div>
  </div>
</template>
