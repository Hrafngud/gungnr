<script setup lang="ts">
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import WorkbenchCatalogControlsPanel from '@/components/workbench/WorkbenchCatalogControlsPanel.vue'
import type { WorkbenchRequestStatus } from '@/stores/workbench'
import type {
  BadgeTone,
  WorkbenchComposeContextSummary,
  WorkbenchComposeIssueInventoryRow,
  WorkbenchInlineFeedbackState,
  WorkbenchOptionalServiceCatalogRow,
  WorkbenchPendingOptionalServiceMutation,
} from '@/components/workbench/projectDetailWorkbenchTypes'

type WorkbenchRemediationAction = 'refresh' | 'import' | 'refresh_backups'

interface WorkbenchStructuredErrorDetails {
  revision?: number
  expectedRevision?: number
  sourceFingerprint?: string
  currentSourceFingerprint?: string
}

interface WorkbenchRemediationState {
  tone: BadgeTone
  sourceLabel: string
  title: string
  message: string
  primaryAction?: WorkbenchRemediationAction
  secondaryAction?: WorkbenchRemediationAction
  details?: WorkbenchStructuredErrorDetails
}

defineProps<{
  isAdmin: boolean
  composeActionBusy: boolean
  composeImportRequired: boolean
  importStatus: WorkbenchRequestStatus
  resolveStatus: WorkbenchRequestStatus
  portMutationStatus: WorkbenchRequestStatus
  resourceMutationStatus: WorkbenchRequestStatus
  optionalServiceMutationStatus: WorkbenchRequestStatus
  previewStatus: WorkbenchRequestStatus
  applyStatus: WorkbenchRequestStatus
  previewLabel: string
  applyLabel: string
  hasPreviewCompose: boolean
  applyActionDisabled: boolean
  composeIssueInventory: WorkbenchComposeIssueInventoryRow[]
  previewFeedback: WorkbenchInlineFeedbackState | null
  applyFeedback: WorkbenchInlineFeedbackState | null
  composeRemediationState: WorkbenchRemediationState | null
  remediationActionDisabled: (action: WorkbenchRemediationAction) => boolean
  remediationActionLabel: (action: WorkbenchRemediationAction) => string
  runRemediationAction: (action?: WorkbenchRemediationAction) => void | Promise<void>
  previewCompose: () => void | Promise<void>
  copyPreviewCompose: () => void | Promise<void>
  applyCompose: () => void | Promise<void>
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
</script>

<template>
  <UiPanel
    variant="soft"
    class="space-y-4 p-4 text-sm text-[color:var(--muted)]"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">Add Services</h3>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <div v-if="isAdmin" class="flex flex-wrap gap-2">
          <UiButton
            variant="ghost"
            size="sm"
            class="w-full justify-center sm:w-auto"
            :disabled="
              composeActionBusy ||
              composeImportRequired ||
              importStatus === 'loading' ||
              resolveStatus === 'loading' ||
              portMutationStatus === 'loading' ||
              resourceMutationStatus === 'loading' ||
              optionalServiceMutationStatus === 'loading'
            "
            @click="previewCompose"
          >
            <span class="inline-flex items-center gap-2">
              <UiInlineSpinner v-if="previewStatus === 'loading'" />
              {{ previewLabel }}
            </span>
          </UiButton>
          <UiButton
            variant="ghost"
            size="sm"
            class="w-full justify-center sm:w-auto"
            :disabled="!hasPreviewCompose || composeActionBusy"
            @click="copyPreviewCompose"
          >
            Copy preview
          </UiButton>
          <UiButton
            variant="primary"
            size="sm"
            class="w-full justify-center sm:w-auto"
            :disabled="applyActionDisabled"
            @click="applyCompose"
          >
            <span class="inline-flex items-center gap-2">
              <UiInlineSpinner v-if="applyStatus === 'loading'" />
              {{ applyLabel }}
            </span>
          </UiButton>
        </div>
        <p v-else class="text-xs text-[color:var(--muted)]">
          Read-only access: admin permissions are required to generate compose preview output and apply stored Workbench changes.
        </p>
        <UiBadge :tone="composeIssueInventory.length > 0 ? 'warn' : 'ok'">
          {{ composeIssueInventory.length }} blockers
        </UiBadge>
      </div>
    </div>

    <UiInlineFeedback
      v-if="previewFeedback"
      :tone="previewFeedback.tone"
    >
      {{ previewFeedback.message }}
    </UiInlineFeedback>
    <UiInlineFeedback
      v-if="applyFeedback"
      :tone="applyFeedback.tone"
    >
      {{ applyFeedback.message }}
    </UiInlineFeedback>

    <UiPanel
      v-if="composeRemediationState"
      variant="soft"
      class="space-y-3 border border-[color:var(--line)] p-3"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            {{ composeRemediationState.sourceLabel }}
          </p>
          <h4 class="mt-1 font-semibold text-[color:var(--text)]">
            {{ composeRemediationState.title }}
          </h4>
        </div>
        <UiBadge :tone="composeRemediationState.tone">
          Needs attention
        </UiBadge>
      </div>
      <p>{{ composeRemediationState.message }}</p>

      <div
        v-if="
          composeRemediationState.details?.expectedRevision != null ||
          composeRemediationState.details?.revision != null ||
          composeRemediationState.details?.sourceFingerprint ||
          composeRemediationState.details?.currentSourceFingerprint
        "
        class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4"
      >
        <UiPanel
          v-if="composeRemediationState.details?.expectedRevision != null"
          variant="raise"
          class="space-y-1 p-3"
        >
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Expected revision
          </p>
          <p class="text-sm font-semibold text-[color:var(--text)]">
            {{ composeRemediationState.details?.expectedRevision }}
          </p>
        </UiPanel>
        <UiPanel
          v-if="composeRemediationState.details?.revision != null"
          variant="raise"
          class="space-y-1 p-3"
        >
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Current revision
          </p>
          <p class="text-sm font-semibold text-[color:var(--text)]">
            {{ composeRemediationState.details?.revision }}
          </p>
        </UiPanel>
        <UiPanel
          v-if="composeRemediationState.details?.sourceFingerprint"
          variant="raise"
          class="space-y-1 p-3"
        >
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Stored fingerprint
          </p>
          <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
            {{ composeRemediationState.details?.sourceFingerprint }}
          </p>
        </UiPanel>
        <UiPanel
          v-if="composeRemediationState.details?.currentSourceFingerprint"
          variant="raise"
          class="space-y-1 p-3"
        >
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            On-disk fingerprint
          </p>
          <p class="font-mono text-[11px] text-[color:var(--text)] break-all">
            {{ composeRemediationState.details?.currentSourceFingerprint }}
          </p>
        </UiPanel>
      </div>

      <div
        v-if="isAdmin && (composeRemediationState.primaryAction || composeRemediationState.secondaryAction)"
        class="flex flex-wrap gap-2"
      >
        <UiButton
          v-if="composeRemediationState.primaryAction"
          variant="primary"
          size="sm"
          :disabled="remediationActionDisabled(composeRemediationState.primaryAction)"
          @click="runRemediationAction(composeRemediationState.primaryAction)"
        >
          {{ remediationActionLabel(composeRemediationState.primaryAction) }}
        </UiButton>
        <UiButton
          v-if="composeRemediationState.secondaryAction"
          variant="ghost"
          size="sm"
          :disabled="remediationActionDisabled(composeRemediationState.secondaryAction)"
          @click="runRemediationAction(composeRemediationState.secondaryAction)"
        >
          {{ remediationActionLabel(composeRemediationState.secondaryAction) }}
        </UiButton>
      </div>
    </UiPanel>

    <WorkbenchCatalogControlsPanel
      :is-admin="isAdmin"
      :compose-path="composePath"
      :fingerprint-label="fingerprintLabel"
      :current-compose-summary="currentComposeSummary"
      :optional-service-inventory="optionalServiceInventory"
      :catalog-status="catalogStatus"
      :catalog-error-message="catalogErrorMessage"
      :pending-optional-service-mutation="pendingOptionalServiceMutation"
      :optional-service-pending-confirmation="optionalServicePendingConfirmation"
      :optional-service-pending-action="optionalServicePendingAction"
      :optional-service-action-disabled="optionalServiceActionDisabled"
      :queue-optional-service-mutation="queueOptionalServiceMutation"
      :optional-service-busy="optionalServiceBusy"
      :optional-service-pending-label="optionalServicePendingLabel"
      :optional-service-feedback="optionalServiceFeedback"
      :confirm-optional-service-mutation="confirmOptionalServiceMutation"
      :cancel-optional-service-mutation="cancelOptionalServiceMutation"
    />
  </UiPanel>
</template>
