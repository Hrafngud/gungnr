<script setup lang="ts">
import UiHeroMark from '@/components/ui/UiHeroMark.vue'

defineProps<{
  show: boolean
  message?: string
}>()
</script>

<template>
  <Transition name="page-fade">
    <div v-if="show" class="page-overlay" role="status" aria-live="polite">
      <UiHeroMark />
      <span class="sr-only">{{ message || 'Loading...' }}</span>
    </div>
  </Transition>
</template>
