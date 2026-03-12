<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type {
  BadgeTone,
  WorkbenchComposeContextSummary,
  WorkbenchInlineFeedbackState,
  WorkbenchOptionalServiceCatalogRow,
  WorkbenchPendingOptionalServiceMutation,
} from '@/components/workbench/projectDetailWorkbenchTypes'

const props = defineProps<{
  isAdmin: boolean
  composePath: string
  fingerprintLabel: string
  currentComposeSummary: WorkbenchComposeContextSummary
  optionalServiceInventory: WorkbenchOptionalServiceCatalogRow[]
  catalogStatus: WorkbenchRequestStatus
  catalogErrorMessage: string
  pendingOptionalServiceMutation: WorkbenchPendingOptionalServiceMutation | null
  optionalServicePendingConfirmation: (entry: WorkbenchOptionalServiceCatalogRow) => boolean
  optionalServicePendingAction: (entry: WorkbenchOptionalServiceCatalogRow) => 'add' | 'remove'
  optionalServiceActionDisabled: (entry: WorkbenchOptionalServiceCatalogRow) => boolean
  queueOptionalServiceMutation: (entry: WorkbenchOptionalServiceCatalogRow) => void
  optionalServiceBusy: (entry: WorkbenchOptionalServiceCatalogRow) => boolean
  optionalServicePendingLabel: (entry: WorkbenchOptionalServiceCatalogRow) => string
  optionalServiceFeedback: (
    entry: WorkbenchOptionalServiceCatalogRow,
  ) => WorkbenchInlineFeedbackState | null
  confirmOptionalServiceMutation: (entry: WorkbenchOptionalServiceCatalogRow) => void
  cancelOptionalServiceMutation: (entryKey: string) => void
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
</script>

<template>
  <div class="flex flex-col gap-4">
    <UiPanel variant="soft" class=" p-4">
      <UiState v-if="catalogStatus === 'loading'" loading>
        Loading optional-service catalog...
      </UiState>
      <UiState v-else-if="catalogStatus === 'error'" tone="error">
        {{ catalogErrorMessage }}
      </UiState>
      <UiState v-else-if="optionalServiceInventory.length === 0">
        No optional-service catalog entries are available for this project yet.
      </UiState>
      <div v-else class="w-full h-[35vh] overflow-y-auto">
        <UiListRow
          v-for="entry in optionalServiceInventory"
          :key="entry.key"
          as="article"
          class="w-full"
        >
          <div class="flex flex-row justify-between gap-4 items-center w-full">
            <div class="min-w-0 flex flex-col gap-2">
              <p class="text-[11px] uppercase tracking-[0.18em] text-[color:var(--muted-2)]">
                {{ entry.category }}
              </p>
              <h4 class="mt-1 text-sm font-semibold text-[color:var(--text)]">
                {{ entry.displayName }}
              </h4>
              <p class="mt-1 text-xs text-[color:var(--muted)]">
                {{ entry.defaultServiceName }} · {{ entry.defaultContainerPortLabel }}
              </p>
            </div>
            <div class="w-full">
              <span :class="['workbench-compact-status', workbenchCompactToneClass(entry.availabilityTone)]">
                <span class="workbench-compact-status__dot" />
                {{ entry.availabilityLabel }}
              </span>
              <span :class="['workbench-compact-status', workbenchCompactToneClass(entry.currentStateTone)]">
                <span class="workbench-compact-status__dot" />
                {{ entry.currentStateLabel }}
              </span>
            </div>
              <UiButton
                v-if="!optionalServicePendingConfirmation(entry)"
                :variant="optionalServicePendingAction(entry) === 'remove' ? 'ghost' : 'primary'"
                size="sm"
                :disabled="optionalServiceActionDisabled(entry)"
                @click="queueOptionalServiceMutation(entry)"
                class="w-[10vw] cursor-pointer"
              >
                <span class="inline-flex items-center gap-2">
                  <UiInlineSpinner v-if="optionalServiceBusy(entry)" />
                  {{ optionalServicePendingLabel(entry) }}
                </span>
              </UiButton>
          </div>

          <p
            v-if="entry.notes[0] || (entry.legacyModuleType && entry.legacyMutationPath)"
            class="text-xs text-[color:var(--muted)]"
          >
            {{
              entry.notes[0] ||
              `${entry.legacyModuleType} stays visible on ${entry.legacyMutationPath}.`
            }}
          </p>

          <UiInlineFeedback
            v-if="optionalServiceFeedback(entry)"
            :tone="optionalServiceFeedback(entry)?.tone || 'neutral'"
          >
            {{ optionalServiceFeedback(entry)?.message }}
          </UiInlineFeedback>

          <div
            v-if="isAdmin && optionalServicePendingConfirmation(entry)"
            class="rounded-2xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3 text-xs text-[color:var(--muted)]"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <p class="font-medium text-[color:var(--text)]">
                {{
                  pendingOptionalServiceMutation?.action === 'remove'
                    ? `Remove ${pendingOptionalServiceMutation.serviceName}?`
                    : `Add ${entry.defaultServiceName}?`
                }}
              </p>
              <div class="flex flex-wrap gap-2">
                <UiButton
                  variant="primary"
                  size="sm"
                  :disabled="optionalServiceActionDisabled(entry)"
                  @click="confirmOptionalServiceMutation(entry)"
                >
                  <span class="inline-flex items-center gap-2">
                    <UiInlineSpinner v-if="optionalServiceBusy(entry)" />
                    Confirm
                  </span>
                </UiButton>
                <UiButton
                  variant="ghost"
                  size="sm"
                  :disabled="optionalServiceBusy(entry)"
                  @click="cancelOptionalServiceMutation(entry.key)"
                >
                  Cancel
                </UiButton>
              </div>
            </div>
          </div>

          <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
            Read only for non-admin users.
          </p>
        </UiListRow>
      </div>
    </UiPanel>
  </div>
</template>

<style scoped>
.workbench-shell-card {
  min-width: 0;
  border-radius: 1.25rem;
}

.workbench-catalog-list {
  display: grid;
  gap: 0.5rem;
  max-height: 28rem;
  overflow: auto;
  padding-right: 0.2rem;
}

.workbench-catalog-row {
  display: grid;
  gap: 0.6rem;
}

.workbench-catalog-row__main {
  display: grid;
  gap: 0.55rem;
}

.workbench-catalog-row__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.workbench-catalog-row__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
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

@media (min-width: 720px) {
  .workbench-catalog-row__main {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: start;
  }

  .workbench-catalog-row__actions {
    justify-content: flex-end;
  }
}

@media (max-width: 639px) {
  .workbench-shell-card {
    border-radius: 1rem;
  }

  .workbench-compact-status,
  .workbench-compact-metric {
    font-size: 0.7rem;
  }
}
</style>
