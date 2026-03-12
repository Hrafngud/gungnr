<script setup lang="ts">
import { computed } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelectTypingInput from '@/components/ui/UiSelectTypingInput.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchResourceField } from '@/types/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchResourceEditorField,
  WorkbenchResourceInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import { workbenchCompactToneClass } from '@/components/workbench/workbenchInspectorPresentation'

const cpuPresetValues = [
  '0.25',
  '0.50',
  '1',
  '2',
  '4',
  '8',
]

const memoryPresetValues = [
  '256M',
  '512M',
  '768M',
  '1G',
  '2G',
  '4G',
  '6G',
  '8G',
]

const resourcePresetValuesByField: Record<WorkbenchResourceField, string[]> = {
  limitCpus: cpuPresetValues,
  reservationCpus: cpuPresetValues,
  limitMemory: memoryPresetValues,
  reservationMemory: memoryPresetValues,
}

function resourcePresetValues(field: WorkbenchResourceField): string[] {
  return resourcePresetValuesByField[field] ?? []
}

function resourcePickerOptions(field: WorkbenchResourceField) {
  return resourcePresetValues(field).map((value) => ({
    key: value,
    value,
    label: value,
  }))
}

const props = defineProps<{
  isAdmin: boolean
  selectedServiceResource: WorkbenchResourceInventoryRow | null
  resourceEditorFields: WorkbenchResourceEditorField[]
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

const limitFields = computed(() => props.resourceEditorFields.filter((field) => field.section === 'limits'))
const reservationFields = computed(() => props.resourceEditorFields.filter((field) => field.section === 'reservations'))

function resourceFieldLabel(field: WorkbenchResourceField): string {
  return field.includes('Memory') ? 'RAM' : 'CPU'
}
</script>

<template>
  <div class="w-full space-y-3">
    <div class="flex mb-2">
      <h5 class="text-base font-semibold text-[color:var(--text)]">Resource Allocation</h5>
    </div>

    <UiState v-if="!selectedServiceResource">
      No Workbench resource row is stored for this service yet.
    </UiState>
    <div v-else class="space-y-3">
      <div class="space-y-2">
        <p class="text-base font-semibold text-[color:var(--text)]">Limits</p>
        <div class="grid gap-3 sm:grid-cols-2">
          <UiPanel
            v-for="field in limitFields"
            :key="`${selectedServiceResource.key}-${field.key}`"
            variant="raise"
            class="space-y-3 p-3"
          >
            <div class="flex flex-wrap items-start justify-between gap-2">
              <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource[field.key] ? 'ok' : 'neutral')]">
                <span class="workbench-compact-status__dot" />
                {{ selectedServiceResource[field.key] ? 'Stored' : 'Empty' }}
              </span>
              <UiButton
                v-if="isAdmin"
                variant="ghost"
                size="sm"
                :disabled="
                  !selectedServiceResource[field.key] ||
                  resourceActionDisabled(selectedServiceResource)
                "
                @click="clearResourceFields(selectedServiceResource, [field.key])"
              >
                Clear {{ resourceFieldLabel(field.key).toLowerCase() }}
              </UiButton>
            </div>

            <div class="space-y-2 text-xs text-[color:var(--muted)]">
              <div class="flex flex-row items-center justify-between">
                <h6 class="mt-1 text-sm font-semibold text-[color:var(--text)]">
                  {{ resourceFieldLabel(field.key) }}
                </h6>
                <div class="w-5/6">
                <UiSelectTypingInput
                  :model-value="resourceInputValue(selectedServiceResource, field.key)"
                  :options="resourcePickerOptions(field.key)"
                  status="ready"
                  input-type="text"
                  :placeholder="field.placeholder"
                  :disabled="!isAdmin || resourceActionDisabled(selectedServiceResource)"
                  :busy="resourceMutationBusy(selectedServiceResource)"
                  :can-request-options="false"
                  empty-message="No presets available."
                  toggle-aria-label="Toggle preset values"
                  @update:model-value="
                    setResourceInputValue(
                      selectedServiceResource.serviceName,
                      field.key,
                      $event,
                    )
                  "
                />
                </div>
              </div>
            </div>

          </UiPanel>
        </div>
      </div>

      <div class="space-y-2">
        <p class="text-base font-semibold text-[color:var(--text)]">Reservations</p>
        <div class="grid gap-3 sm:grid-cols-2">
          <UiPanel
            v-for="field in reservationFields"
            :key="`${selectedServiceResource.key}-${field.key}`"
            variant="raise"
            class="space-y-3 p-3"
          >
            <div class="flex flex-wrap items-start justify-between gap-2">
              <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource[field.key] ? 'ok' : 'neutral')]">
                <span class="workbench-compact-status__dot" />
                {{ selectedServiceResource[field.key] ? 'Stored' : 'Empty' }}
              </span>
              <UiButton
                v-if="isAdmin"
                variant="ghost"
                size="sm"
                :disabled="
                  !selectedServiceResource[field.key] ||
                  resourceActionDisabled(selectedServiceResource)
                "
                @click="clearResourceFields(selectedServiceResource, [field.key])"
              >
                Clear {{ resourceFieldLabel(field.key).toLowerCase() }}
              </UiButton>
            </div>

            <div class="space-y-2 text-xs text-[color:var(--muted)]">
              <div class="flex flex-row items-center justify-between">
                <h6 class="mt-1 text-sm font-semibold text-[color:var(--text)]">
                  {{ resourceFieldLabel(field.key) }}
                </h6>
                <div class="w-5/6">
                <UiSelectTypingInput
                  :model-value="resourceInputValue(selectedServiceResource, field.key)"
                  :options="resourcePickerOptions(field.key)"
                  status="ready"
                  input-type="text"
                  :placeholder="field.placeholder"
                  :disabled="!isAdmin || resourceActionDisabled(selectedServiceResource)"
                  :busy="resourceMutationBusy(selectedServiceResource)"
                  :can-request-options="false"
                  empty-message="No presets available."
                  toggle-aria-label="Toggle preset values"
                  @update:model-value="
                    setResourceInputValue(
                      selectedServiceResource.serviceName,
                      field.key,
                      $event,
                    )
                  "
                />
                </div>
              </div>
            </div>

          </UiPanel>
        </div>
      </div>

      <div v-if="isAdmin" class="flex flex-wrap gap-2">
        <UiButton
          variant="primary"
          size="sm"
          :disabled="resourceActionDisabled(selectedServiceResource)"
          @click="saveResource(selectedServiceResource)"
        >
          <span class="inline-flex items-center gap-2">
            <UiInlineSpinner
              v-if="resourceMutationBusy(selectedServiceResource)"
            />
            Save budgets
          </span>
        </UiButton>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="resourceActionDisabled(selectedServiceResource)"
          @click="resetResourceInputs(selectedServiceResource)"
        >
          Reset inputs
        </UiButton>
        <UiButton
          variant="ghost"
          size="sm"
          :disabled="
            resourceActionDisabled(selectedServiceResource) ||
            (!selectedServiceResource.hasLimits && !selectedServiceResource.hasReservations)
          "
          @click="
            clearResourceFields(selectedServiceResource, [
              'limitCpus',
              'limitMemory',
              'reservationCpus',
              'reservationMemory',
            ])
          "
        >
          Clear all
        </UiButton>
      </div>

      <UiInlineFeedback
        v-if="resourceMutationFeedback(selectedServiceResource)"
        :tone="resourceMutationFeedback(selectedServiceResource)?.tone || 'neutral'"
      >
        {{ resourceMutationFeedback(selectedServiceResource)?.message }}
      </UiInlineFeedback>
    </div>
  </div>
</template>
