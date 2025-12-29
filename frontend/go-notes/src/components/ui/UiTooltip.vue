<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  as?: string | Component
  text: string
  position?: 'top' | 'bottom'
}>(), {
  as: 'span',
  position: 'top',
})

const positionClass = computed(() => (
  props.position === 'bottom' ? 'tooltip-bottom' : 'tooltip-top'
))
</script>

<template>
  <component :is="as" class="tooltip" :class="positionClass" v-bind="$attrs">
    <slot />
    <span class="tooltip-content" role="tooltip">
      {{ text }}
    </span>
  </component>
</template>
