<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import NavIcon from '@/components/NavIcon.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import UiToggle from '@/components/ui/UiToggle.vue'
import { apiErrorMessage } from '@/services/api'
import { projectsApi } from '@/services/projects'
import { useToastStore } from '@/stores/toasts'
import type { ProjectArchiveOptions, ProjectArchivePlan } from '@/types/projects'

const props = defineProps<{
  projectName: string
  projectDisplayName: string
  isAdmin: boolean
}>()

const emit = defineEmits<{
  queued: []
}>()

const toastStore = useToastStore()

const archivePlan = ref<ProjectArchivePlan | null>(null)
const archivePlanLoading = ref(false)
const archivePlanError = ref<string | null>(null)
const archiveExecuting = ref(false)
const archiveExecuteError = ref<string | null>(null)
const archiveExecutedWithWarnings = ref(false)
const archiveConfirmInput = ref('')
const archiveOptions = ref<ProjectArchiveOptions>({
  removeContainers: true,
  removeVolumes: false,
  removeIngress: true,
  removeDns: true,
})

const archiveConfirmationPhrase = computed(() => {
  const normalized = (props.projectDisplayName || props.projectName || '').toLowerCase().trim()
  if (!normalized) return 'ARCHIVE PROJECT'
  return `ARCHIVE ${normalized}`
})

const canSubmitArchive = computed(() => {
  if (!props.isAdmin || archiveExecuting.value) return false
  if (archiveOptions.value.removeVolumes && !archiveOptions.value.removeContainers) return false
  return archiveConfirmInput.value.trim() === archiveConfirmationPhrase.value
})

function applyArchiveDefaults(plan: ProjectArchivePlan) {
  archiveOptions.value = {
    removeContainers: plan.defaults.removeContainers,
    removeVolumes: plan.defaults.removeVolumes,
    removeIngress: plan.defaults.removeIngress,
    removeDns: plan.defaults.removeDns,
  }
}

async function loadArchivePlan() {
  if (!props.projectName) {
    archivePlan.value = null
    archivePlanError.value = 'Invalid project name.'
    return
  }
  if (!props.isAdmin) {
    archivePlan.value = null
    archivePlanError.value = null
    return
  }

  archivePlanLoading.value = true
  archivePlanError.value = null
  try {
    const { data } = await projectsApi.getArchivePlan(props.projectName)
    archivePlan.value = data.plan
    applyArchiveDefaults(data.plan)
  } catch (err) {
    archivePlan.value = null
    archivePlanError.value = apiErrorMessage(err)
  } finally {
    archivePlanLoading.value = false
  }
}

async function queueArchive() {
  if (!props.projectName) return
  if (!props.isAdmin) {
    toastStore.error('Admin access required.', 'Archive blocked')
    return
  }
  if (!canSubmitArchive.value) return

  archiveExecuteError.value = null
  archiveExecuting.value = true
  try {
    const payload = {
      removeContainers: archiveOptions.value.removeContainers,
      removeVolumes: archiveOptions.value.removeVolumes,
      removeIngress: archiveOptions.value.removeIngress,
      removeDns: archiveOptions.value.removeDns,
    }
    const { data } = await projectsApi.archiveProject(props.projectName, payload)
    archivePlan.value = data.plan
    archiveExecutedWithWarnings.value = (data.plan.warnings?.length ?? 0) > 0
    archiveConfirmInput.value = ''

    if (archiveExecutedWithWarnings.value) {
      toastStore.warn(
        `Archive queued (job #${data.job.id}) with ${data.plan.warnings.length} warning(s) in plan preview.`,
        'Archive queued',
      )
    } else {
      toastStore.success(`Archive queued (job #${data.job.id}).`, 'Project cleanup')
    }

    emit('queued')
  } catch (err) {
    const message = apiErrorMessage(err)
    archiveExecuteError.value = message
    toastStore.error(message, 'Archive queue failed')
  } finally {
    archiveExecuting.value = false
  }
}

watch(
  () => archiveOptions.value.removeContainers,
  (enabled) => {
    if (enabled) return
    archiveOptions.value.removeVolumes = false
  },
)

watch(
  () => [props.projectName, props.isAdmin],
  () => {
    archivePlan.value = null
    archivePlanError.value = null
    archiveExecuteError.value = null
    archiveExecuting.value = false
    archiveExecutedWithWarnings.value = false
    archiveConfirmInput.value = ''
    void loadArchivePlan()
  },
  { immediate: true },
)

</script>

<template>
  <UiPanel class="space-y-5 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Archive cleanup</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Plan and execute</h2>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Cleanup is asynchronous and always queued as a project job.
        </p>
      </div>
      <UiButton variant="ghost" size="sm" :disabled="archivePlanLoading" @click="loadArchivePlan">
        <span class="inline-flex items-center gap-2">
          <NavIcon name="refresh" class="h-3.5 w-3.5" />
          <UiInlineSpinner v-if="archivePlanLoading" />
          Refresh plan
        </span>
      </UiButton>
    </div>

    <UiState v-if="!isAdmin">
      Read-only access: admin permissions are required to preview and execute archive cleanup.
    </UiState>
    <UiState v-else-if="archivePlanLoading" loading>Building archive cleanup plan...</UiState>
    <UiState v-else-if="archivePlanError" tone="error">{{ archivePlanError }}</UiState>

    <template v-else-if="archivePlan">
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers</p>
          <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.containers.length }}</p>
        </UiPanel>
        <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
          <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.hostnames.length }}</p>
        </UiPanel>
        <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress rules</p>
          <p class="text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.ingressRules.length }}</p>
        </UiPanel>
        <UiPanel variant="soft" class="space-y-1 p-3 text-sm text-[color:var(--muted)]">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS records</p>
          <p class="text-lg font-semibold text-[color:var(--text)]">
            {{ archivePlan.dnsRecords.filter((record) => record.deleteEligible).length }}/{{ archivePlan.dnsRecords.length }}
          </p>
        </UiPanel>
      </div>

      <UiInlineFeedback v-if="archivePlan.warnings.length > 0" tone="warn">
        {{ archivePlan.warnings.length }} warning(s): {{ archivePlan.warnings.join(' | ') }}
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecutedWithWarnings" tone="warn">
        Last archive request was queued with warnings in the plan preview. Review job logs after completion.
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecuteError" tone="error">
        {{ archiveExecuteError }}
      </UiInlineFeedback>

      <div class="grid gap-4 xl:grid-cols-2">
        <UiPanel variant="soft" class="space-y-3 p-4">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS targets</p>
          <UiState v-if="archivePlan.dnsRecords.length === 0">No DNS records matched.</UiState>
          <ul v-else class="space-y-2 text-xs text-[color:var(--muted)]">
            <li
              v-for="record in archivePlan.dnsRecords"
              :key="`${record.zoneId}-${record.id}-${record.name}`"
              class="space-y-1"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-mono text-[color:var(--text)] break-all">{{ record.name }}</span>
                <UiBadge :tone="record.deleteEligible ? 'ok' : 'warn'">
                  {{ record.deleteEligible ? 'deletable' : 'skip' }}
                </UiBadge>
              </div>
              <p class="font-mono text-[11px] text-[color:var(--muted-2)] break-all">
                {{ record.type }} → {{ record.content }}
              </p>
              <p v-if="record.skipReason" class="text-[11px] text-[color:var(--muted)]">
                {{ record.skipReason }}
              </p>
            </li>
          </ul>
        </UiPanel>
      </div>

      <UiPanel variant="raise" class="space-y-4 p-5">
        <div>
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Execution</p>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Confirmation phrase: <span class="font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
          </p>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <UiToggle v-model="archiveOptions.removeContainers" :disabled="!isAdmin">
            Remove project containers
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeVolumes" :disabled="!isAdmin || !archiveOptions.removeContainers">
            Remove container volumes
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeIngress" :disabled="!isAdmin">
            Remove ingress rules
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeDns" :disabled="!isAdmin">
            Remove DNS records
          </UiToggle>
        </div>

        <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
          Confirmation phrase
        </label>
        <UiInput
          v-model="archiveConfirmInput"
          :disabled="!isAdmin || archiveExecuting"
          autocomplete="off"
          spellcheck="false"
          placeholder="Type the phrase exactly"
        />

        <div class="flex flex-wrap items-center gap-3">
          <UiButton
            variant="danger"
            size="sm"
            :disabled="!canSubmitArchive"
            @click="queueArchive"
          >
            <span class="inline-flex items-center gap-2">
              <UiInlineSpinner v-if="archiveExecuting" />
              {{ archiveExecuting ? 'Queueing archive...' : 'Queue archive job' }}
            </span>
          </UiButton>
          <p v-if="!isAdmin" class="text-xs text-[color:var(--muted)]">
            Read-only access: admin permissions are required to queue archive cleanup.
          </p>
        </div>
      </UiPanel>
    </template>
  </UiPanel>
</template>
