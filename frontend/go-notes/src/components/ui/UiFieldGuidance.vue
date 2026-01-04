<script setup lang="ts">
export interface GuidanceLink {
  label: string
  href: string
}

export interface GuidanceContent {
  title: string
  description: string
  links?: GuidanceLink[]
}

defineProps<{
  modelValue: boolean
  content: GuidanceContent | null
}>()
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="opacity-0 -translate-x-8"
      enter-to-class="opacity-100 translate-x-0"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="opacity-100 translate-x-0"
      leave-to-class="opacity-0 -translate-x-8"
    >
      <div
        v-if="modelValue && content"
        class="fixed left-[5%] top-1/2 z-[70] w-[300px] -translate-y-1/2 space-y-4 bg-[color:var(--surface)] p-6 shadow-2xl"
        role="complementary"
        aria-live="polite"
      >
        <div class="space-y-2">
          <h3 class="text-2xl font-semibold leading-tight text-[color:var(--text)]">
            {{ content.title }}
          </h3>
          <p class="text-base leading-relaxed text-[color:var(--muted)]">
            {{ content.description }}
          </p>
        </div>

        <div v-if="content.links?.length" class="space-y-2">
          <p class="text-xs uppercase tracking-[0.2em] text-[color:var(--muted-2)]">
            Quick links
          </p>
          <div class="space-y-2">
            <a
              v-for="link in content.links"
              :key="link.href"
              :href="link.href"
              target="_blank"
              rel="noreferrer"
              class="flex items-center justify-between gap-2 rounded border border-[color:var(--border)] bg-[color:var(--bg-soft)] px-3 py-2 text-sm text-[color:var(--text)] transition hover:border-[color:var(--accent)] hover:text-[color:var(--accent-ink)]"
            >
              <span>{{ link.label }}</span>
              <svg
                class="h-4 w-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <path d="M15 3h6v6" />
                <path d="M10 14L21 3" />
              </svg>
            </a>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
