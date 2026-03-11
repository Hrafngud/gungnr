<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchResourceField } from '@/types/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchResourceEditorField,
  WorkbenchResourceInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import { workbenchCompactToneClass } from '@/components/workbench/workbenchInspectorPresentation'

defineProps<{
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
</script>

<template>
  <div class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/30 p-3">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Resources</p>
        <h5 class="mt-1 text-base font-semibold text-[color:var(--text)]">Budgets</h5>
      </div>
      <div class="flex flex-wrap gap-2 text-[11px]">
        <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource?.tracked ? 'ok' : 'neutral')]">
          <span class="workbench-compact-status__dot" />
          {{ selectedServiceResource?.tracked ? 'Tracked' : 'No row' }}
        </span>
        <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource?.hasLimits ? 'ok' : 'neutral')]">
          <span class="workbench-compact-status__dot" />
          {{ selectedServiceResource?.hasLimits ? 'Limits set' : 'Limits empty' }}
        </span>
        <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource?.hasReservations ? 'ok' : 'neutral')]">
          <span class="workbench-compact-status__dot" />
          {{ selectedServiceResource?.hasReservations ? 'Reservations set' : 'Reservations empty' }}
        </span>
      </div>
    </div>

    <UiState v-if="!selectedServiceResource">
      No Workbench resource row is stored for this service yet.
    </UiState>
    <div v-else class="space-y-3">
      <div class="grid gap-3 sm:grid-cols-2">
        <UiPanel
          v-for="field in resourceEditorFields"
          :key="`${selectedServiceResource.key}-${field.key}`"
          variant="raise"
          class="space-y-3 p-3"
        >
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div>
              <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                {{ field.section === 'limits' ? 'Limits' : 'Reservations' }}
              </p>
              <h6 class="mt-1 text-sm font-semibold text-[color:var(--text)]">
                {{ field.label }}
              </h6>
            </div>
            <span :class="['workbench-compact-status', workbenchCompactToneClass(selectedServiceResource[field.key] ? 'ok' : 'neutral')]">
              <span class="workbench-compact-status__dot" />
              {{ selectedServiceResource[field.key] ? 'Stored' : 'Empty' }}
            </span>
          </div>

          <div class="space-y-2 text-xs text-[color:var(--muted)]">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span>Current</span>
              <span class="font-mono text-[color:var(--text)]">
                {{ selectedServiceResource[field.key] || 'Not declared' }}
              </span>
            </div>

            <label class="space-y-1">
              <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                New value
              </span>
              <UiInput
                :model-value="resourceInputValue(selectedServiceResource, field.key)"
                type="text"
                :placeholder="field.placeholder"
                :disabled="!isAdmin || resourceActionDisabled(selectedServiceResource)"
                @update:model-value="
                  setResourceInputValue(
                    selectedServiceResource.serviceName,
                    field.key,
                    $event,
                  )
                "
              />
            </label>
          </div>

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
            Clear {{ field.label.toLowerCase() }}
          </UiButton>
        </UiPanel>
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

