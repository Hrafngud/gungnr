<script setup lang="ts">
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'

defineProps<{
  show: boolean
  message?: string
}>()
</script>

<template>
  <Transition name="page-fade">
    <div v-if="show" class="page-overlay" role="status" aria-live="polite">
      <div class="page-overlay-card">
        <UiInlineSpinner />
        <p class="text-sm font-semibold text-[color:var(--text)]">
          {{ message || 'Loading...' }}
        </p>
        <p class="text-xs text-[color:var(--muted)]">
          Preparing the latest host data.
        </p>
      </div>
    </div>
  </Transition>
</template>
