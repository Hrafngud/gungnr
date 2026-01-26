<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ inheritAttrs: false })

type StateTone = 'neutral' | 'ok' | 'warn' | 'error'

const props = withDefaults(defineProps<{
  tone?: StateTone
  loading?: boolean
}>(), {
  tone: 'neutral',
  loading: false,
})

const toneClass = computed(() => `state-${props.tone}`)
</script>

<template>
  <div class="state" :class="toneClass" v-bind="$attrs">
    <span v-if="loading" class="spinner-mark state-spinner" aria-hidden="true" />
    <slot />
  </div>
</template>
