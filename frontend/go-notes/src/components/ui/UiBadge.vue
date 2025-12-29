<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ inheritAttrs: false })

type BadgeTone = 'neutral' | 'ok' | 'warn' | 'error'

const props = withDefaults(defineProps<{
  tone?: BadgeTone
}>(), {
  tone: 'neutral',
})

const toneClass = computed(() => {
  switch (props.tone) {
    case 'ok':
      return 'status-ok'
    case 'warn':
      return 'status-warn'
    case 'error':
      return 'status-error'
    default:
      return 'status-neutral'
  }
})
</script>

<template>
  <span class="badge" :class="toneClass" v-bind="$attrs">
    <slot />
  </span>
</template>
