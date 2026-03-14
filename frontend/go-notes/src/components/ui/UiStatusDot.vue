<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ inheritAttrs: false })

type StatusDotTone = 'neutral' | 'ok' | 'warn' | 'error'
type StatusDotSize = 'xs' | 'sm' | 'md'
type StatusDotPalette = 'fixed' | 'semantic'

const props = withDefaults(defineProps<{
  tone?: StatusDotTone
  size?: StatusDotSize
  palette?: StatusDotPalette
}>(), {
  tone: 'neutral',
  size: 'sm',
  palette: 'fixed',
})

const toneClass = computed(() => `ui-status-dot--${props.tone}`)
const sizeClass = computed(() => `ui-status-dot--${props.size}`)
const paletteClass = computed(() => `ui-status-dot--${props.palette}`)
</script>

<template>
  <span class="ui-status-dot" :class="[toneClass, sizeClass, paletteClass]" v-bind="$attrs" />
</template>

<style scoped>
.ui-status-dot {
  display: inline-flex;
  border-radius: 9999px;
  flex: 0 0 auto;
}

.ui-status-dot--xs {
  width: 0.375rem;
  height: 0.375rem;
}

.ui-status-dot--sm {
  width: 0.48rem;
  height: 0.48rem;
}

.ui-status-dot--md {
  width: 0.5rem;
  height: 0.5rem;
}

.ui-status-dot--fixed.ui-status-dot--ok {
  background: #22c55e;
}

.ui-status-dot--fixed.ui-status-dot--warn {
  background: #f59e0b;
}

.ui-status-dot--fixed.ui-status-dot--error {
  background: #ef4444;
}

.ui-status-dot--fixed.ui-status-dot--neutral {
  background: color-mix(in oklab, var(--muted) 75%, transparent);
}

.ui-status-dot--semantic.ui-status-dot--ok {
  background: var(--success);
}

.ui-status-dot--semantic.ui-status-dot--warn {
  background: var(--warning);
}

.ui-status-dot--semantic.ui-status-dot--error {
  background: var(--danger);
}

.ui-status-dot--semantic.ui-status-dot--neutral {
  background: var(--muted-2);
}
</style>
