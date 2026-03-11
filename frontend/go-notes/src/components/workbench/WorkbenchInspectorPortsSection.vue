<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type {
  WorkbenchInlineFeedbackState,
  WorkbenchPortInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'
import type { WorkbenchPortSuggestion } from '@/components/workbench/workbenchInspectorPresentation'
import {
  workbenchCompactToneClass,
  workbenchGuidanceToneClass,
} from '@/components/workbench/workbenchInspectorPresentation'

defineProps<{
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
  loadPortSuggestions: (port: WorkbenchPortInventoryRow) => void
  portMutationFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
  portSuggestionFeedback: (port: WorkbenchPortInventoryRow) => WorkbenchInlineFeedbackState | null
}>()
</script>

<template>
  <div class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/30 p-3">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ports</p>
        <h5 class="mt-1 text-base font-semibold text-[color:var(--text)]">Mappings</h5>
      </div>
      <div class="flex flex-wrap gap-2">
        <span class="workbench-compact-metric">
          <span class="workbench-compact-metric__value">
            {{ selectedServicePorts.filter((port) => port.allocationStatus === 'assigned').length }}
          </span>
          <span>assigned</span>
        </span>
        <span class="workbench-compact-metric">
          <span class="workbench-compact-metric__value">
            {{ selectedServicePorts.filter((port) => port.allocationStatus === 'unresolved').length }}
          </span>
          <span>unresolved</span>
        </span>
      </div>
    </div>

    <UiState v-if="selectedServicePorts.length === 0">
      No Workbench port rows are stored for this service yet.
    </UiState>
    <div v-else class="space-y-3">
      <UiListRow
        v-for="port in selectedServicePorts"
        :key="port.key"
        as="article"
        class="space-y-3"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h6 class="text-base font-semibold text-[color:var(--text)]">
              {{ port.containerPort }}/{{ port.protocol }}
            </h6>
            <p class="mt-1 font-mono text-[11px] text-[color:var(--muted-2)]">
              {{ port.mappingLabel }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2 text-[11px]">
            <span :class="['workbench-compact-status', workbenchCompactToneClass(port.assignmentStrategyTone)]">
              <span class="workbench-compact-status__dot" />
              {{ port.assignmentStrategyLabel }}
            </span>
            <span :class="['workbench-compact-status', workbenchCompactToneClass(port.allocationStatusTone)]">
              <span class="workbench-compact-status__dot" />
              {{ port.allocationStatusLabel }}
            </span>
          </div>
        </div>

        <div class="grid gap-3 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
          <div class="grid gap-2 text-xs text-[color:var(--muted)]">
            <div class="flex flex-wrap items-start justify-between gap-2">
              <span>Requested</span>
              <span class="font-mono text-[color:var(--text)]">{{ port.requestedHostPort || 'Auto' }}</span>
            </div>
            <div class="flex flex-wrap items-start justify-between gap-2">
              <span>Effective</span>
              <span class="font-mono text-[color:var(--text)]">{{ port.effectiveHostPortLabel }}</span>
            </div>
          </div>

          <div class="space-y-3 rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
            <div class="workbench-port-editor">
              <label class="space-y-2">
                <span class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                  Host port
                </span>
                <UiInput
                  :model-value="portInputValue(port)"
                  type="number"
                  min="1"
                  max="65535"
                  step="1"
                  placeholder="8080"
                  :disabled="
                    !isAdmin ||
                    portMutationBusy(port) ||
                    optionalServiceMutationStatus === 'loading' ||
                    previewStatus === 'loading' ||
                    applyStatus === 'loading' ||
                    restoreStatus === 'loading'
                  "
                  @update:model-value="setPortInputValue(port.key, $event)"
                />
              </label>
              <div v-if="isAdmin" class="workbench-port-editor__actions">
                <UiButton
                  variant="primary"
                  size="sm"
                  :disabled="
                    portMutationBusy(port) ||
                    resolveStatus === 'loading' ||
                    optionalServiceMutationStatus === 'loading' ||
                    previewStatus === 'loading' ||
                    applyStatus === 'loading' ||
                    restoreStatus === 'loading'
                  "
                  @click="setManualPort(port)"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="portMutationBusy(port)" />
                    Set manual
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="
                    portMutationBusy(port) ||
                    resolveStatus === 'loading' ||
                    optionalServiceMutationStatus === 'loading' ||
                    previewStatus === 'loading' ||
                    applyStatus === 'loading' ||
                    restoreStatus === 'loading'
                  "
                  @click="resetPortToAuto(port)"
                >
                  Reset
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="
                    portSuggestionStatus(port) === 'loading' ||
                    portMutationBusy(port) ||
                    optionalServiceMutationStatus === 'loading' ||
                    previewStatus === 'loading' ||
                    applyStatus === 'loading' ||
                    restoreStatus === 'loading'
                  "
                  @click="loadPortSuggestions(port)"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="portSuggestionStatus(port) === 'loading'" />
                    Suggestions
                  </span>
                </UiButton>
              </div>
            </div>

            <UiInlineFeedback
              v-if="portMutationFeedback(port)"
              :tone="portMutationFeedback(port)?.tone || 'neutral'"
            >
              {{ portMutationFeedback(port)?.message }}
            </UiInlineFeedback>
            <UiInlineFeedback
              v-if="portSuggestionFeedback(port)"
              :tone="portSuggestionFeedback(port)?.tone || 'neutral'"
            >
              {{ portSuggestionFeedback(port)?.message }}
            </UiInlineFeedback>

            <div
              v-if="portSuggestionResultByKey[port.key]?.suggestions?.length"
              class="flex flex-wrap gap-2"
            >
              <UiButton
                v-for="suggestion in portSuggestionResultByKey[port.key]?.suggestions || []"
                :key="`${port.key}-suggestion-${suggestion.rank}-${suggestion.hostPort}`"
                variant="ghost"
                size="sm"
                :disabled="
                  portMutationBusy(port) ||
                  optionalServiceMutationStatus === 'loading' ||
                  previewStatus === 'loading' ||
                  applyStatus === 'loading' ||
                  restoreStatus === 'loading'
                "
                @click="setPortInputValue(port.key, String(suggestion.hostPort))"
              >
                #{{ suggestion.rank }} · {{ suggestion.hostPort }}
              </UiButton>
            </div>

            <p
              class="text-xs"
              :class="workbenchGuidanceToneClass(port.allocationStatus)"
            >
              {{ port.guidance }}
            </p>
          </div>
        </div>
      </UiListRow>
    </div>
  </div>
</template>

