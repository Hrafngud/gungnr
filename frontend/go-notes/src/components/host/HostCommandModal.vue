<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiState from '@/components/ui/UiState.vue'
import { useToastStore } from '@/stores/toasts'
import { jobActionLabel, jobStatusLabel, jobStatusTone } from '@/utils/jobStatus'
import type { JobDetail } from '@/types/jobs'

const props = withDefaults(defineProps<{
  modelValue: boolean
  command: string
  action?: string
  job?: JobDetail | null
  logs?: string[]
  error?: string | null
  expiresAt?: string | null
  polling?: boolean
}>(), {
  action: '',
  job: null,
  logs: () => [],
  error: null,
  expiresAt: null,
  polling: false,
})

const emit = defineEmits<{
  'update:modelValue': [boolean]
}>()

const toastStore = useToastStore()

const jobId = computed(() => props.job?.id ?? null)
const statusLabel = computed(() => jobStatusLabel(props.job?.status))
const statusTone = computed(() => jobStatusTone(props.job?.status))
const actionLabel = computed(() => jobActionLabel(props.action))

const logLines = computed(() => props.logs ?? [])
const trimmedLogs = computed(() => {
  if (logLines.value.length <= 200) return logLines.value
  return logLines.value.slice(logLines.value.length - 200)
})
const logText = computed(() => {
  if (logLines.value.length === 0) {
    return 'Waiting for host output...'
  }
  const prefix =
    logLines.value.length > trimmedLogs.value.length
      ? '... showing the latest 200 lines\n'
      : ''
  return `${prefix}${trimmedLogs.value.join('\n')}`
})

const expiresLabel = computed(() => {
  if (!props.expiresAt) return ''
  const date = new Date(props.expiresAt)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString()
})

const copyCommand = async () => {
  if (!props.command) return
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(props.command)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = props.command
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.focus()
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    toastStore.success('Host command copied to clipboard.', 'Copied')
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Clipboard copy failed.'
    toastStore.error(message, 'Copy failed')
  }
}

const closeModal = () => {
  emit('update:modelValue', false)
}
</script>

<template>
  <UiModal
    v-bind="$attrs"
    :model-value="modelValue"
    title="Run the host worker"
    description="Execute the command on the host to update the local cloudflared service and apply deploy changes."
    @update:model-value="closeModal"
  >
    <div class="space-y-5">
      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Host command
            </p>
            <p class="mt-1 text-xs text-[color:var(--muted)]">
              Run from the repo root on the host running Docker and the cloudflared service.
            </p>
          </div>
          <UiButton variant="ghost" size="xs" :disabled="!command" @click="copyCommand">
            Copy
          </UiButton>
        </div>
        <pre
          class="whitespace-pre-wrap break-words rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)] p-3 text-xs text-[color:var(--text)]"
        ><code>{{ command || 'Waiting for host command...' }}</code></pre>
        <p v-if="expiresLabel" class="text-xs text-[color:var(--muted)]">
          Token expires at {{ expiresLabel }}.
        </p>
      </UiPanel>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Host action
          </p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            {{ actionLabel }}
          </p>
        </div>
        <UiBadge :tone="statusTone">
          {{ statusLabel }}
        </UiBadge>
      </div>

      <UiState v-if="error" tone="error">
        {{ error }}
      </UiState>

      <UiPanel variant="soft" class="space-y-3 p-4">
        <div class="flex items-center justify-between">
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Worker logs
          </p>
          <span class="text-xs text-[color:var(--muted-2)]">
            {{ logLines.length }} lines
          </span>
        </div>
        <pre
          class="max-h-52 overflow-auto whitespace-pre-wrap break-words rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-inset)] p-3 text-xs text-[color:var(--text)]"
        ><code>{{ logText }}</code></pre>
        <p v-if="polling" class="text-xs text-[color:var(--muted)]">
          Polling status every few seconds until completion.
        </p>
      </UiPanel>
    </div>

    <template #footer>
      <UiButton variant="ghost" size="sm" @click="closeModal">
        Close
      </UiButton>
      <UiButton
        v-if="jobId"
        :as="RouterLink"
        :to="`/jobs/${jobId}`"
        variant="primary"
        size="sm"
      >
        Open full log
      </UiButton>
    </template>
  </UiModal>
</template>
