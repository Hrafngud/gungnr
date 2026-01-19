<script setup lang="ts">
defineProps<{
  show: boolean
  message?: string
}>()
</script>

<template>
  <Transition name="page-fade">
    <div v-if="show" class="page-overlay" role="status" aria-live="polite">
      <div class="page-overlay-gif" aria-hidden="true" />
      <span class="sr-only">{{ message || 'Loading...' }}</span>
    </div>
  </Transition>
</template>
