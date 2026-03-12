<script setup lang="ts">
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiSelectTypingInput from '@/components/ui/UiSelectTypingInput.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchPortInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import type { WorkbenchPortSuggestion } from '@/components/workbench/workbenchInspectorPresentation'
import { workbenchGuidanceToneClass } from '@/components/workbench/workbenchInspectorPresentation'

const props = defineProps<{
  isAdmin: boolean
  optionalServiceMutationStatus: WorkbenchRequestStatus
  previewStatus: WorkbenchRequestStatus
  applyStatus: WorkbenchRequestStatus
  restoreStatus: WorkbenchRequestStatus
  resolveStatus: WorkbenchRequestStatus
  selectedServicePorts: WorkbenchPortInventoryRow[]
  portSuggestionResultByKey: Record<string, { suggestions?: WorkbenchPortSuggestion[] } | null | undefined>
  portInputValue: (port: WorkbenchPortInventoryRow) => string
  setPortInputValue: (key: string, value: string) => void
  portMutationBusy: (port: WorkbenchPortInventoryRow) => boolean
  setManualPort: (port: WorkbenchPortInventoryRow) => void
  resetPortToAuto: (port: WorkbenchPortInventoryRow) => void
  portSuggestionStatus: (port: WorkbenchPortInventoryRow) => WorkbenchRequestStatus
  loadPortSuggestions: (
    port: WorkbenchPortInventoryRow,
    options?: { silent?: boolean },
  ) => void | Promise<void>
  portMutationFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
  portSuggestionFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
}>()

function portEditorDisabled(port: WorkbenchPortInventoryRow): boolean {
  return (
    !props.isAdmin ||
    props.portMutationBusy(port) ||
    props.resolveStatus === 'loading' ||
    props.optionalServiceMutationStatus === 'loading' ||
    props.previewStatus === 'loading' ||
    props.applyStatus === 'loading' ||
    props.restoreStatus === 'loading'
  )
}

function portPickerOptions(port: WorkbenchPortInventoryRow) {
  const suggestions = props.portSuggestionResultByKey[port.key]?.suggestions ?? []
  return suggestions.map((suggestion) => ({
    key: `${suggestion.rank}-${suggestion.hostPort}`,
    value: String(suggestion.hostPort),
    label: `#${suggestion.rank} · ${suggestion.hostPort}`,
  }))
}
</script>

<template>
  <div class="w-full mb-2">
    <div class="flex mb-2">
      <h5 class="text-base font-semibold text-[color:var(--text)]">Container Port</h5>
    </div>

    <UiState v-if="props.selectedServicePorts.length === 0">
      No Workbench port rows are stored for this service yet.
    </UiState>

    <div v-else class="flex w-full flex-col gap-2">
      <div
        v-for="port in props.selectedServicePorts"
        :key="port.key"
        class="flex flex-col gap-2"
      >
        <UiListRow as="article">
            <div class="flex flex-col items-start w-full">
              <div class="flex w-full flex-row items-center gap-1">
                <span class="text-[11px] min-w-1/6 uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                  Host port
                </span>
                <UiSelectTypingInput
                  :model-value="props.portInputValue(port)"
                  :options="portPickerOptions(port)"
                  :status="props.portSuggestionStatus(port)"
                  :disabled="portEditorDisabled(port)"
                  :busy="props.portMutationBusy(port)"
                  input-type="number"
                  placeholder="8080"
                  :min="1"
                  :max="65535"
                  :step="1"
                  :show-action="props.isAdmin"
                  action-label="Auto allocation"
                  toggle-aria-label="Toggle port suggestions"
                  @update:model-value="props.setPortInputValue(port.key, $event)"
                  @request-options="props.loadPortSuggestions(port, { silent: true })"
                  @commit="props.setManualPort(port)"
                  @action="props.resetPortToAuto(port)"
                />
            </div>

            <div class="flex flex-col w-full gap-2">
              <UiInlineFeedback
                v-if="props.portMutationFeedback(port)"
                :tone="props.portMutationFeedback(port)?.tone || 'neutral'"
              >
                {{ props.portMutationFeedback(port)?.message }}
              </UiInlineFeedback>
              <UiInlineFeedback
                v-if="props.portSuggestionFeedback(port)"
                :tone="props.portSuggestionFeedback(port)?.tone || 'neutral'"
              >
                {{ props.portSuggestionFeedback(port)?.message }}
              </UiInlineFeedback>
            </div>
          </div>
        </UiListRow>

        <p
          class="px-1 text-xs"
          :class="workbenchGuidanceToneClass(port.allocationStatus)"
        >
          {{ port.guidance }}
        </p>
      </div>
    </div>
  </div>
</template>
