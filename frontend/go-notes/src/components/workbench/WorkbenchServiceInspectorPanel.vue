<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import WorkbenchDependencyGraph from '@/components/workbench/WorkbenchDependencyGraph.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type { WorkbenchResourceField } from '@/types/workbench'
import type {
  BadgeTone,
  WorkbenchInlineFeedbackState,
  WorkbenchPortInventoryRow,
  WorkbenchResourceEditorField,
  WorkbenchResourceInventoryRow,
  WorkbenchServiceInventoryRow,
  WorkbenchTopologyInventoryRow,
} from '@/components/workbench/projectDetailWorkbenchTypes'

interface WorkbenchPortSuggestion {
  rank: number
  hostPort: number
}

const props = defineProps<{
  isAdmin: boolean
  optionalServiceMutationStatus: WorkbenchRequestStatus
  previewStatus: WorkbenchRequestStatus
  applyStatus: WorkbenchRequestStatus
  restoreStatus: WorkbenchRequestStatus
  resolveStatus: WorkbenchRequestStatus
  serviceInventory: WorkbenchServiceInventoryRow[]
  selectedService: WorkbenchServiceInventoryRow | null
  selectedServiceTopology: WorkbenchTopologyInventoryRow | null
  selectedServicePorts: WorkbenchPortInventoryRow[]
  selectedServiceResource: WorkbenchResourceInventoryRow | null
  resourceEditorFields: WorkbenchResourceEditorField[]
  portSuggestionResultByKey: Record<string, { suggestions?: WorkbenchPortSuggestion[] } | null | undefined>
  selectService: (serviceName: string) => void
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

function workbenchCompactToneClass(tone: BadgeTone): string {
  switch (tone) {
    case 'ok':
      return 'workbench-compact-status-ok'
    case 'warn':
      return 'workbench-compact-status-warn'
    case 'error':
      return 'workbench-compact-status-error'
    default:
      return 'workbench-compact-status-neutral'
  }
}

function workbenchGuidanceToneClass(status: string): string {
  if (status === 'unavailable') return 'text-[color:var(--danger)]'
  if (status === 'conflict') return 'text-[color:var(--warn)]'
  return 'text-[color:var(--muted)]'
}
</script>

<template>
  <div class="workbench-shell-grid grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
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
          :aria-pressed="selectedService?.serviceName === service.serviceName"
          :class="{
            'workbench-service-selector--active': selectedService?.serviceName === service.serviceName,
          }"
          @click="selectService(service.serviceName)"
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
              <UiBadge
                :tone="selectedService?.serviceName === service.serviceName ? 'ok' : 'neutral'"
              >
                {{ selectedService?.serviceName === service.serviceName ? 'Selected' : 'Select' }}
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

    <UiPanel
      variant="soft"
      class="workbench-shell-card workbench-shell-card--right space-y-4 p-4"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
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

        <div class="workbench-inspector-section">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Dependencies</p>
              <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">Graph</h4>
            </div>
            <UiBadge :tone="selectedServiceTopology ? 'ok' : 'neutral'">
              {{
                selectedServiceTopology &&
                (selectedServiceTopology.dependsOn.length > 0 ||
                  selectedServiceTopology.dependedBy.length > 0)
                  ? 'Connected'
                  : 'Isolated'
              }}
            </UiBadge>
          </div>

          <UiState v-if="!selectedServiceTopology">
            No Workbench topology rows are stored for this service yet.
          </UiState>
          <div v-else class="space-y-3 text-xs text-[color:var(--muted)]">
            <WorkbenchDependencyGraph
              :service-name="selectedServiceTopology.serviceName"
              :depends-on="selectedServiceTopology.dependsOn"
              :depended-by="selectedServiceTopology.dependedBy"
            />
            <div class="grid gap-2 sm:grid-cols-2">
              <div class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
                <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Networks</p>
                <p class="mt-2 text-[color:var(--text)]">
                  {{
                    selectedServiceTopology.networkNames.length > 0
                      ? selectedServiceTopology.networkNames.join(', ')
                      : 'None'
                  }}
                </p>
              </div>
              <div class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
                <p class="text-[11px] uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
                  Legacy metadata
                </p>
                <p class="mt-2 text-[color:var(--text)]">
                  {{
                    selectedServiceTopology.moduleTypes.length > 0
                      ? selectedServiceTopology.moduleTypes.join(', ')
                      : 'None'
                  }}
                </p>
              </div>
            </div>
          </div>
        </div>

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
          </div>
        </div>
      </template>
    </UiPanel>
  </div>
</template>

<style scoped>
.workbench-shell-grid {
  grid-column: 1 / -1;
}

.workbench-shell-card {
  min-width: 0;
  border-radius: 1.25rem;
}

.workbench-service-selector {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: 100%;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  border-radius: 1rem;
  padding: 0.875rem;
  background: color-mix(in srgb, var(--panel) 70%, transparent);
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    transform 160ms ease;
}

.workbench-service-selector:hover {
  border-color: color-mix(in srgb, var(--accent) 60%, var(--line));
  background: color-mix(in srgb, var(--panel) 82%, transparent);
}

.workbench-service-selector:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 75%, white);
  outline-offset: 2px;
}

.workbench-service-selector--active {
  border-color: color-mix(in srgb, var(--accent) 75%, var(--line));
  background: color-mix(in srgb, var(--accent) 12%, var(--panel));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 24%, transparent);
}

.workbench-service-selector__eyebrow {
  font-size: 0.68rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--muted-2);
}

.workbench-compact-status {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  border-radius: 999px;
  padding: 0.24rem 0.58rem;
  background: color-mix(in srgb, var(--panel) 70%, transparent);
  font-size: 0.74rem;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
}

.workbench-compact-status__dot {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.92;
}

.workbench-compact-status-neutral {
  color: var(--muted);
}

.workbench-compact-status-ok {
  color: var(--ok);
}

.workbench-compact-status-warn {
  color: var(--warn);
}

.workbench-compact-status-error {
  color: var(--danger);
}

.workbench-compact-metric {
  display: inline-flex;
  align-items: baseline;
  gap: 0.45rem;
  padding: 0.28rem 0.62rem;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  background: color-mix(in srgb, var(--panel) 66%, transparent);
  font-size: 0.72rem;
  color: var(--muted);
  white-space: nowrap;
}

.workbench-compact-metric__value {
  color: var(--text);
  font-size: 0.88rem;
  font-weight: 600;
}

.workbench-inspector-hero {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  padding: 0.875rem;
  border-radius: 1rem;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  background: color-mix(in srgb, var(--panel) 76%, transparent);
}

.workbench-inspector-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding-top: 0.15rem;
}

.workbench-inspector-section--editing {
  padding: 0.875rem;
  border-radius: 1.25rem;
  border: 1px solid color-mix(in srgb, var(--line) 80%, transparent);
  background: color-mix(in srgb, var(--panel) 62%, transparent);
}

.workbench-service-chip {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--line) 82%, transparent);
  background: color-mix(in srgb, var(--panel) 72%, transparent);
  padding: 0.26rem 0.62rem;
  font-size: 0.72rem;
  color: var(--text);
  white-space: nowrap;
}

.workbench-port-editor {
  display: grid;
  gap: 0.75rem;
}

.workbench-port-editor__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

@media (min-width: 900px) {
  .workbench-service-selector-list {
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  }
}

@media (min-width: 720px) {
  .workbench-port-editor {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
  }

  .workbench-port-editor__actions {
    justify-content: flex-end;
  }
}

@media (max-width: 639px) {
  .workbench-shell-card {
    border-radius: 1rem;
  }

  .workbench-service-selector {
    padding: 0.8rem;
  }

  .workbench-inspector-section--editing,
  .workbench-inspector-hero {
    padding: 0.8rem;
  }

  .workbench-compact-status,
  .workbench-compact-metric,
  .workbench-service-chip {
    font-size: 0.7rem;
  }
}
</style>
