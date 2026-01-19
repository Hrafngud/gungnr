<script setup lang="ts">
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInput from '@/components/ui/UiInput.vue'

type QueueState = {
  loading: boolean
  error: string | null
  success: string | null
  jobId: number | null
}

type GuidancePayload = {
  title: string
  description: string
}

defineProps<{
  open: boolean
  isAuthenticated: boolean
  state: QueueState
  name: string
  subdomain: string
  port: string
  showGuidance: (payload: GuidancePayload) => void
  clearGuidance: () => void
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'update:name': [string]
  'update:subdomain': [string]
  'update:port': [string]
  submit: []
}>()
</script>

<template>
  <UiFormSidePanel
    :model-value="open"
    title="Forward localhost service"
    @update:model-value="emit('update:open', $event)"
  >
    <form class="space-y-5" @submit.prevent="emit('submit')">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Forward any running localhost service (Docker or not) through the host tunnel for web access.
        </p>
      </div>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Service name <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          :model-value="name"
          type="text"
          placeholder="my-service"
          required
          :disabled="state.loading"
          @update:model-value="emit('update:name', $event)"
          @focus="showGuidance({
            title: 'Service name',
            description: 'Used for tracking this forwarded service in jobs and activity.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Internal identifier for tracking this forwarded service.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Subdomain <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          :model-value="subdomain"
          type="text"
          placeholder="my-service"
          required
          :disabled="state.loading"
          @update:model-value="emit('update:subdomain', $event)"
          @focus="showGuidance({
            title: 'Subdomain',
            description: 'The public hostname to route through the host tunnel.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Subdomain for web access through the host tunnel.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Running at (localhost port) <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          :model-value="port"
          type="text"
          placeholder="3000"
          required
          :disabled="state.loading"
          @update:model-value="emit('update:port', $event)"
          @focus="showGuidance({
            title: 'Localhost port',
            description: 'Enter the port your service is already listening on (for example 3000).',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          The localhost port where your service is currently running.
        </p>
      </label>
      <UiInlineFeedback v-if="state.error" tone="error">
        {{ state.error }}
      </UiInlineFeedback>
      <UiInlineFeedback v-if="state.success" tone="ok">
        {{ state.success }}
      </UiInlineFeedback>
      <div class="flex flex-wrap items-center gap-3">
        <UiButton
          type="submit"
          variant="primary"
          :disabled="state.loading || !isAuthenticated"
        >
          {{ state.loading ? 'Queueing...' : 'Queue forward job' }}
        </UiButton>
        <UiButton
          v-if="state.jobId"
          :as="RouterLink"
          :to="`/jobs/${state.jobId}`"
          variant="ghost"
        >
          View job log
        </UiButton>
      </div>
    </form>
  </UiFormSidePanel>
</template>
