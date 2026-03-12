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

const deletableDnsRecordsCount = computed(() =>
  archivePlan.value?.dnsRecords.filter((record) => record.deleteEligible).length ?? 0,
)

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
  <UiPanel variant="projects" class="space-y-5 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">Project lifecycle</p>
        <h2 class="mt-2 text-xl font-semibold text-[color:var(--text)]">Archive</h2>
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
      <UiPanel variant="raise" class="space-y-4 p-5">
        <div>
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Archive Project</p>
          <p class="mt-2 text-sm text-[color:var(--muted)]">
            Confirmation phrase: <span class="font-mono text-[color:var(--text)]">{{ archiveConfirmationPhrase }}</span>
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <UiToggle v-model="archiveOptions.removeContainers" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove project containers
          </UiToggle>
          <UiToggle
            v-model="archiveOptions.removeVolumes"
            :disabled="!isAdmin || !archiveOptions.removeContainers"
            class="min-w-[240px] flex-1"
          >
            Remove container volumes
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeIngress" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove ingress rules
          </UiToggle>
          <UiToggle v-model="archiveOptions.removeDns" :disabled="!isAdmin" class="min-w-[240px] flex-1">
            Remove DNS records
          </UiToggle>
        </div>

        <div class="flex flex-wrap items-end gap-3">
          <div class="min-w-[280px] flex-1">
            <label class="block text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
              Confirmation phrase
            </label>
            <UiInput
              v-model="archiveConfirmInput"
              :disabled="!isAdmin || archiveExecuting"
              autocomplete="off"
              spellcheck="false"
              placeholder="Type the phrase exactly"
              class="mt-2"
            />
          </div>
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

      <UiInlineFeedback v-if="archivePlan.warnings.length > 0" tone="warn">
        {{ archivePlan.warnings.length }} warning(s): {{ archivePlan.warnings.join(' | ') }}
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecutedWithWarnings" tone="warn">
        Last archive request was queued with warnings in the plan preview. Review job logs after completion.
      </UiInlineFeedback>
      <UiInlineFeedback v-if="archiveExecuteError" tone="error">
        {{ archiveExecuteError }}
      </UiInlineFeedback>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <UiPanel variant="soft" class="space-y-3 p-4">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Plan summary</p>
          <div class="flex flex-wrap gap-3">
            <div class="min-w-[140px] flex-1 rounded-xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Containers</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.containers.length }}</p>
            </div>
            <div class="min-w-[140px] flex-1 rounded-xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Hostnames</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.hostnames.length }}</p>
            </div>
            <div class="min-w-[140px] flex-1 rounded-xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">Ingress rules</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">{{ archivePlan.ingressRules.length }}</p>
            </div>
            <div class="min-w-[140px] flex-1 rounded-xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3">
              <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS records</p>
              <p class="mt-1 text-lg font-semibold text-[color:var(--text)]">
                {{ deletableDnsRecordsCount }}/{{ archivePlan.dnsRecords.length }}
              </p>
            </div>
          </div>
        </UiPanel>

        <UiPanel variant="soft" class="space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">DNS targets</p>
            <p class="text-xs text-[color:var(--muted)]">
              {{ deletableDnsRecordsCount }} deletable / {{ archivePlan.dnsRecords.length }} total
            </p>
          </div>
          <UiState v-if="archivePlan.dnsRecords.length === 0">No DNS records matched.</UiState>
          <div v-else class="overflow-x-auto pb-1">
            <ul class="flex min-w-max items-stretch gap-2 text-xs text-[color:var(--muted)]">
              <li
                v-for="record in archivePlan.dnsRecords"
                :key="`${record.zoneId}-${record.id}-${record.name}`"
                class="w-[320px] space-y-1 rounded-xl border border-[color:var(--line)]/70 bg-[color:var(--panel)]/35 p-3"
              >
                <div class="flex items-center justify-between gap-2">
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
          </div>
        </UiPanel>
      </div>
    </template>
  </UiPanel>
</template>
