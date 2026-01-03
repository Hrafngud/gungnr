<script setup lang="ts">
import { computed, ref } from 'vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import UiServiceIcon from '@/components/ui/UiServiceIcon.vue'
import { servicePresets } from '@/data/service-presets'

interface ServiceCard {
  id: string
  name: string
  description: string
  subdomain?: string
  port?: number
  image?: string
  containerPort?: number
  repoLabel: string
  repoUrl: string
  kind: 'custom' | 'preset'
  icon?: string
}

const props = defineProps<{
  selectedCardId: string | null
}>()

const emit = defineEmits<{
  (e: 'select-card', card: ServiceCard): void
}>()

const searchQuery = ref('')

const customServiceCard: ServiceCard = {
  id: 'custom',
  name: 'Custom service',
  description: 'Forward any local port through the host tunnel.',
  repoLabel: 'cloudflare/cloudflared',
  repoUrl: 'https://github.com/cloudflare/cloudflared',
  kind: 'custom',
  icon: 'custom',
}

const allServiceCards = computed<ServiceCard[]>(() => [
  customServiceCard,
  ...servicePresets.map((preset) => ({
    id: preset.id,
    name: preset.name,
    description: preset.description,
    subdomain: preset.subdomain,
    port: preset.port,
    image: preset.image,
    containerPort: preset.containerPort,
    repoLabel: preset.repoLabel,
    repoUrl: preset.repoUrl,
    kind: 'preset' as const,
    icon: preset.icon,
  })),
])

const serviceCards = computed<ServiceCard[]>(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return allServiceCards.value

  return allServiceCards.value.filter(card =>
    card.name.toLowerCase().includes(query) ||
    card.description.toLowerCase().includes(query)
  )
})
</script>

<template>
  <UiPanel as="article" variant="raise" class="space-y-6 p-6">
    <div>
      <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
        Services
      </p>
      <h3 class="mt-2 text-lg font-semibold text-[color:var(--text)]">
        Quick tunnel forwards
      </h3>
      <p class="mt-2 text-sm text-[color:var(--muted)]">
        Expose a running port through the host tunnel service instantly.
      </p>
    </div>
    <div class="relative">
      <UiInput
        v-model="searchQuery"
        type="text"
        placeholder="Search services..."
        class="w-full"
      />
      <svg
        class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[color:var(--muted-2)]"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.35-4.35" />
      </svg>
    </div>
    <div class="overflow-y-auto" style="max-height: 368px">
      <div class="grid gap-4 sm:grid-cols-2">
        <UiPanel
          v-for="card in serviceCards"
          :key="card.id"
          :variant="selectedCardId === card.id ? 'raise' : 'soft'"
          class="flex h-full flex-col gap-4 p-4 text-left transition"
          :class="selectedCardId === card.id ? 'border-[color:var(--accent)]' : ''"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded bg-[color:var(--bg-soft)]">
                <UiServiceIcon :type="card.icon" />
              </div>
              <div>
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  Service
                </p>
                <h4 class="mt-1 text-base font-semibold text-[color:var(--text)]">
                  {{ card.name }}
                </h4>
              </div>
            </div>
            <UiBadge :tone="selectedCardId === card.id ? 'ok' : 'neutral'">
              {{ selectedCardId === card.id ? 'Selected' : card.kind === 'custom' ? 'Custom' : 'Preset' }}
            </UiBadge>
          </div>
          <p class="text-xs text-[color:var(--muted)]">
            {{ card.description }}
          </p>
          <div class="flex flex-wrap items-center gap-2 text-xs text-[color:var(--muted)]">
            <span v-if="card.subdomain">subdomain: {{ card.subdomain }}</span>
            <span v-if="card.port">port: {{ card.port }}</span>
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
              :href="card.repoUrl"
              target="_blank"
              rel="noreferrer"
              class="text-[color:var(--text)] transition hover:text-[color:var(--accent-ink)]"
            >
              {{ card.repoLabel }}
            </a>
          </div>
          <UiButton
            type="button"
            variant="ghost"
            size="sm"
            @click="emit('select-card', card)"
          >
            {{ selectedCardId === card.id ? 'Hide form' : 'Select service' }}
          </UiButton>
        </UiPanel>
      </div>
      <p
        v-if="serviceCards.length === 0"
        class="py-8 text-center text-sm text-[color:var(--muted)]"
      >
        No services found matching "{{ searchQuery }}"
      </p>
    </div>
  </UiPanel>
</template>
