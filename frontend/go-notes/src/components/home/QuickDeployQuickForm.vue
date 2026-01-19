<script setup lang="ts">
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

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

type SelectOption = {
  value: string | number
  label: string
  disabled?: boolean
}

defineProps<{
  open: boolean
  title: string
  isAuthenticated: boolean
  state: QueueState
  subdomain: string
  domain: string
  port: string
  image: string
  containerPort: string
  domainOptions: SelectOption[]
  showGuidance: (payload: GuidancePayload) => void
  clearGuidance: () => void
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'update:subdomain': [string]
  'update:domain': [string]
  'update:port': [string]
  'update:image': [string]
  'update:containerPort': [string]
  submit: []
}>()
</script>

<template>
  <UiFormSidePanel
    :model-value="open"
    :title="title"
    @update:model-value="emit('update:open', $event)"
  >
    <form class="space-y-5" @submit.prevent="emit('submit')">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Expose a running port through the host tunnel service instantly.
        </p>
      </div>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Subdomain
        </span>
        <UiInput
          :model-value="subdomain"
          type="text"
          placeholder="preview"
          :disabled="state.loading"
          @update:model-value="emit('update:subdomain', $event)"
          @focus="showGuidance({
            title: 'Subdomain',
            description: 'Set the hostname Cloudflare should route to this service.',
          })"
          @blur="clearGuidance()"
        />
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Domain
        </span>
        <UiSelect
          :model-value="domain"
          :options="domainOptions"
          placeholder="Select a domain"
          :disabled="state.loading"
          @update:model-value="emit('update:domain', String($event))"
          @focusin="showGuidance({
            title: 'Domain',
            description: 'Pick which configured domain should receive this hostname.',
          })"
          @focusout="clearGuidance()"
        />
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Local port
        </span>
        <UiInput
          :model-value="port"
          type="text"
          placeholder="5173"
          :disabled="state.loading"
          @update:model-value="emit('update:port', $event)"
          @focus="showGuidance({
            title: 'Local port',
            description: 'Port already exposed on this host that you want to publish.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Host port exposed by Docker on this machine.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Container image (optional)
        </span>
        <UiInput
          :model-value="image"
          type="text"
          placeholder="excalidraw/excalidraw:latest"
          :disabled="state.loading"
          @update:model-value="emit('update:image', $event)"
          @focus="showGuidance({
            title: 'Container image',
            description: 'Optional image to run if you want Gungnr to launch the service.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Leave blank to use the default image.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Container port (optional)
        </span>
        <UiInput
          :model-value="containerPort"
          type="text"
          placeholder="80"
          :disabled="state.loading"
          @update:model-value="emit('update:containerPort', $event)"
          @focus="showGuidance({
            title: 'Container port',
            description: 'Port inside the container that the host port maps to.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Port inside the container (default 80). Host port maps to this.
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
          {{ state.loading ? 'Queueing...' : 'Forward service' }}
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
