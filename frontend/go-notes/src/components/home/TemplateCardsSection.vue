<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import type { GitHubCatalog } from '@/types/github'

type TemplateCardId = 'create' | 'existing'

interface TemplateCard {
  id: TemplateCardId
  title: string
  description: string
  actionLabel: string
}

const props = defineProps<{
  catalog: GitHubCatalog | null
  catalogError: string | null
  selectedCard: TemplateCardId | null
}>()

const emit = defineEmits<{
  (e: 'select-card', id: TemplateCardId): void
}>()

const templateCards: TemplateCard[] = [
  {
    id: 'create',
    title: 'Create from template',
    description: 'Create a new GitHub repo and deploy with auto-configured ports.',
    actionLabel: 'Configure template',
  },
  {
    id: 'existing',
    title: 'Forward localhost service',
    description: 'Expose any running localhost port via Cloudflare tunnel.',
    actionLabel: 'Configure forward',
  },
]

const resolvedTemplates = computed(() => {
  const list = props.catalog?.templates?.filter(
    (template) => template.owner && template.repo,
  )
  if (list && list.length > 0) return list
  if (props.catalog?.template?.configured) {
    const { owner, repo } = props.catalog.template
    if (owner && repo) {
      return [props.catalog.template]
    }
  }
  return []
})

const templateRepoLabel = computed(() => {
  if (props.catalogError) return 'Template source unavailable'
  if (resolvedTemplates.value.length === 0) return 'Template source not configured'
  if (resolvedTemplates.value.length === 1) {
    const template = resolvedTemplates.value[0]
    if (template) {
      return `${template.owner}/${template.repo}`
    }
    return 'Template source not configured'
  }
  return `${resolvedTemplates.value.length} template sources`
})

const templateRepoUrl = computed(() => {
  if (resolvedTemplates.value.length !== 1) return ''
  const template = resolvedTemplates.value[0]
  if (!template) return ''
  return `https://github.com/${template.owner}/${template.repo}`
})
</script>

<template>
  <UiPanel as="article" variant="raise" class="space-y-6 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
          Templates
        </p>
        <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
          GitHub-backed stacks
        </h3>
        <p class="mt-2 text-sm text-[color:var(--muted)]">
          Create new repos from templates or deploy existing folders.
        </p>
      </div>
      <UiButton :as="RouterLink" to="/github" variant="ghost" size="sm">
        GitHub settings
      </UiButton>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <UiPanel
        v-for="card in templateCards"
        :key="card.id"
        :variant="selectedCard === card.id ? 'raise' : 'soft'"
        class="flex h-full flex-col gap-4 p-4 text-left transition"
        :class="selectedCard === card.id ? 'border-[color:var(--accent)]' : ''"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
              Template
            </p>
            <h4 class="mt-2 text-base font-semibold text-[color:var(--text)]">
              {{ card.title }}
            </h4>
            <p class="mt-2 text-xs text-[color:var(--muted)]">
              {{ card.description }}
            </p>
          </div>
          <UiBadge :tone="selectedCard === card.id ? 'ok' : 'neutral'">
            {{ selectedCard === card.id ? 'Selected' : 'Ready' }}
          </UiBadge>
        </div>
        <div class="flex items-center gap-2 text-xs text-[color:var(--muted)]">
          <svg
            class="h-3.5 w-3.5 text-[color:var(--muted-2)]"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M10 13a5 5 0 0 1 0-7l2-2a5 5 0 0 1 7 7l-2 2" />
            <path d="M14 11a5 5 0 0 1 0 7l-2 2a5 5 0 0 1-7-7l2-2" />
          </svg>
          <a
            v-if="templateRepoUrl"
            :href="templateRepoUrl"
            target="_blank"
            rel="noreferrer"
            class="text-[color:var(--text)] transition hover:text-[color:var(--accent-ink)]"
          >
            {{ templateRepoLabel }}
          </a>
          <span v-else>{{ templateRepoLabel }}</span>
        </div>
        <UiButton
          type="button"
          variant="ghost"
          size="sm"
          @click="emit('select-card', card.id)"
        >
          {{ card.actionLabel }}
        </UiButton>
      </UiPanel>
    </div>
  </UiPanel>
</template>
