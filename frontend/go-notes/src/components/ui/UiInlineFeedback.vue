<script setup lang="ts">
import UiState from '@/components/ui/UiState.vue'

defineOptions({ inheritAttrs: false })

type FeedbackTone = 'neutral' | 'ok' | 'warn' | 'error'

withDefaults(defineProps<{
  tone?: FeedbackTone
}>(), {
  tone: 'neutral',
})
</script>

<template>
  <UiState :tone="tone" class="text-xs" v-bind="$attrs">
    <slot />
  </UiState>
</template>
