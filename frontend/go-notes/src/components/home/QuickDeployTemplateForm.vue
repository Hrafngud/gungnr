<script setup lang="ts">
import { RouterLink } from 'vue-router'
import UiButton from '@/components/ui/UiButton.vue'
import UiFormSidePanel from '@/components/ui/UiFormSidePanel.vue'
import UiInlineFeedback from '@/components/ui/UiInlineFeedback.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import NavIcon from '@/components/NavIcon.vue'

type QueueState = {
  loading: boolean
  error: string | null
  success: string | null
  jobId: number | null
}

type SelectOption = {
  value: string | number
  label: string
  disabled?: boolean
}

type GuidancePayload = {
  title: string
  description: string
}

const props = defineProps<{
  open: boolean
  isAuthenticated: boolean
  templateOptions: SelectOption[]
  templateRepoLabel: string
  templateEmptyStateMessage: string
  templateCreateBlocked: boolean
  templateSelectionDisabled: boolean
  state: QueueState
  templateRef: string
  name: string
  subdomain: string
  domain: string
  domainOptions: SelectOption[]
  showGuidance: (payload: GuidancePayload) => void
  clearGuidance: () => void
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'update:templateRef': [string]
  'update:name': [string]
  'update:subdomain': [string]
  'update:domain': [string]
  submit: []
}>()
</script>

<template>
  <UiFormSidePanel
    :model-value="open"
    title="Create from template"
    @update:model-value="emit('update:open', $event)"
  >
    <form class="space-y-5" @submit.prevent="emit('submit')">
      <div class="space-y-2">
        <p class="text-xs text-[color:var(--muted)]">
          Create a new GitHub repo from your template and deploy it locally with automatic port configuration.
        </p>
        <p class="text-xs text-[color:var(--muted)]">
          Template source: {{ templateRepoLabel }}
        </p>
      </div>
      <UiPanel
        v-if="templateOptions.length === 0"
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-3 p-3 text-xs text-[color:var(--muted)]"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Template sources
          </p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            {{ templateEmptyStateMessage }}
          </p>
        </div>
        <UiButton
          :as="RouterLink"
          :to="isAuthenticated ? '/host-settings' : '/login'"
          variant="ghost"
          size="sm"
        >
          <span class="flex items-center gap-2">
            <NavIcon v-if="!isAuthenticated" name="login" class="h-3.5 w-3.5" />
            {{ isAuthenticated ? 'Template settings' : 'Sign in' }}
          </span>
        </UiButton>
      </UiPanel>
      <UiPanel
        v-else-if="templateCreateBlocked"
        variant="soft"
        class="flex flex-wrap items-center justify-between gap-3 p-3 text-xs text-[color:var(--muted)]"
      >
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
            Template creation
          </p>
          <p class="mt-1 text-sm text-[color:var(--muted)]">
            GitHub App credentials are required to create new repos from templates.
          </p>
        </div>
        <UiButton
          :as="RouterLink"
          to="/host-settings"
          variant="ghost"
          size="sm"
        >
          Template settings
        </UiButton>
      </UiPanel>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Template repo <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiSelect
          :model-value="templateRef"
          :options="templateOptions"
          placeholder="Select a template"
          :disabled="templateSelectionDisabled"
          @update:model-value="emit('update:templateRef', String($event))"
          @focusin="showGuidance({
            title: 'Template source',
            description: 'Choose the GitHub template repo Gungnr should clone and deploy.',
          })"
          @focusout="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Pick the repository that seeds the new project.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Project name <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          :model-value="name"
          type="text"
          placeholder="my-project"
          required
          :disabled="templateSelectionDisabled"
          @update:model-value="emit('update:name', $event)"
          @focus="showGuidance({
            title: 'Project name',
            description: 'Used for the GitHub repo and the local folder. Keep it short and DNS-safe.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Name for the GitHub repo and local folder.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Subdomain <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiInput
          :model-value="subdomain"
          type="text"
          placeholder="my-project"
          required
          :disabled="templateSelectionDisabled"
          @update:model-value="emit('update:subdomain', $event)"
          @focus="showGuidance({
            title: 'Subdomain',
            description: 'Becomes the hostname prepended to your base domain.',
          })"
          @blur="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Subdomain for web access through the host tunnel.
        </p>
      </label>
      <label class="grid gap-2 text-sm">
        <span class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Domain <span class="text-[color:var(--danger)]">*</span>
        </span>
        <UiSelect
          :model-value="domain"
          :options="domainOptions"
          placeholder="Select a domain"
          :disabled="templateSelectionDisabled"
          @update:model-value="emit('update:domain', String($event))"
          @focusin="showGuidance({
            title: 'Domain',
            description: 'Choose which configured domain should receive this subdomain.',
          })"
          @focusout="clearGuidance()"
        />
        <p class="text-xs text-[color:var(--muted)]">
          Defaults to the base domain configured during bootstrap.
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
          :disabled="templateSelectionDisabled"
        >
          {{ state.loading ? 'Queueing...' : 'Queue template job' }}
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
