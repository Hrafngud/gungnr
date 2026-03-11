<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiState from '@/components/ui/UiState.vue'
import WorkbenchInspectorDependenciesSection from '@/components/workbench/WorkbenchInspectorDependenciesSection.vue'
import WorkbenchInspectorOverviewSection from '@/components/workbench/WorkbenchInspectorOverviewSection.vue'
import WorkbenchInspectorServiceControlsSection from '@/components/workbench/WorkbenchInspectorServiceControlsSection.vue'
import WorkbenchServiceSelectorSection from '@/components/workbench/WorkbenchServiceSelectorSection.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type { WorkbenchResourceField } from '@/types/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchPortInventoryRow,
  WorkbenchResourceEditorField,
  WorkbenchResourceInventoryRow,
  WorkbenchServiceInventoryRow,
  WorkbenchTopologyInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import type { WorkbenchPortSuggestion } from '@/components/workbench/workbenchInspectorPresentation'

const props = defineProps<{
  isAdmin: boolean
  optionalServiceMutationStatus: WorkbenchRequestStatus
  previewStatus: WorkbenchRequestStatus
  applyStatus: WorkbenchRequestStatus
  restoreStatus: WorkbenchRequestStatus
  resolveStatus: WorkbenchRequestStatus
  serviceInventory: WorkbenchServiceInventoryRow[]
  topologyInventory: WorkbenchTopologyInventoryRow[]
  portInventory: WorkbenchPortInventoryRow[]
  resourceInventory: WorkbenchResourceInventoryRow[]
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

const selectedServiceName = ref('')
const selectedService = computed<WorkbenchServiceInventoryRow | null>(() => {
  const inventory = props.serviceInventory
  if (inventory.length === 0) return null

  const selectedName = selectedServiceName.value.trim().toLowerCase()
  if (!selectedName) return inventory[0] ?? null

  return (
    inventory.find((service) => service.serviceName.trim().toLowerCase() === selectedName) ??
    inventory[0] ??
    null
  )
})

const selectedServiceTopology = computed<WorkbenchTopologyInventoryRow | null>(() => {
  const serviceName = selectedService.value?.serviceName
  if (!serviceName) return null
  return props.topologyInventory.find((row) => row.serviceName === serviceName) ?? null
})

const selectedServicePorts = computed<WorkbenchPortInventoryRow[]>(() => {
  const serviceName = selectedService.value?.serviceName
  if (!serviceName) return []
  return props.portInventory.filter((port) => port.serviceName === serviceName)
})

const selectedServiceResource = computed<WorkbenchResourceInventoryRow | null>(() => {
  const serviceName = selectedService.value?.serviceName
  if (!serviceName) return null
  return props.resourceInventory.find((resource) => resource.serviceName === serviceName) ?? null
})

watch(
  () => props.serviceInventory,
  (services) => {
    const selectedName = selectedServiceName.value.trim().toLowerCase()
    if (services.length === 0) {
      selectedServiceName.value = ''
      return
    }

    if (
      selectedName &&
      services.some((service) => service.serviceName.trim().toLowerCase() === selectedName)
    ) {
      return
    }

    selectedServiceName.value = services[0]?.serviceName ?? ''
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex flex-col gap-4">
    <WorkbenchServiceSelectorSection
      :service-inventory="serviceInventory"
      :selected-service-name="selectedService?.serviceName ?? ''"
      @select="selectedServiceName = $event"
    />

    <div
      class="bg-transparent"
    >
      <div class="flex flex-wrap items-start justify-between gap-3 bg--bg p-4 rounded">
        <div>
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Selected service</p>
          <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Inspector</h3>
        </div>
        <UiBadge :tone="selectedService ? 'ok' : 'neutral'">
          {{ selectedService ? selectedService.serviceName : 'No selection' }}
        </UiBadge>
      </div>

      <UiState v-if="!selectedService">
        Select a stored service to inspect its metadata, relationships, ports, and resources in one place.
      </UiState>
      <template v-else>
        <WorkbenchInspectorOverviewSection
          :selected-service="selectedService"
          :selected-service-topology="selectedServiceTopology"
          :selected-service-ports="selectedServicePorts"
        />
        <WorkbenchInspectorDependenciesSection :selected-service-topology="selectedServiceTopology" />
        <WorkbenchInspectorServiceControlsSection
          :is-admin="isAdmin"
          :optional-service-mutation-status="optionalServiceMutationStatus"
          :preview-status="previewStatus"
          :apply-status="applyStatus"
          :restore-status="restoreStatus"
          :resolve-status="resolveStatus"
          :selected-service-ports="selectedServicePorts"
          :selected-service-resource="selectedServiceResource"
          :resource-editor-fields="resourceEditorFields"
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
          :resource-input-value="resourceInputValue"
          :set-resource-input-value="setResourceInputValue"
          :resource-action-disabled="resourceActionDisabled"
          :clear-resource-fields="clearResourceFields"
          :save-resource="saveResource"
          :resource-mutation-busy="resourceMutationBusy"
          :reset-resource-inputs="resetResourceInputs"
          :resource-mutation-feedback="resourceMutationFeedback"
        />
      </template>
    </div>
  </div>
</template>
