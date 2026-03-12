<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

defineOptions({ inheritAttrs: false })

type PanelVariant = 'default' | 'soft' | 'raise' | 'projects'

const props = withDefaults(defineProps<{
  as?: string | Component
  variant?: PanelVariant
}>(), {
  as: 'div',
  variant: 'default',
})

const variantClass = computed(() => {
  switch (props.variant) {
    case 'soft':
      return 'panel-soft'
    case 'raise':
      return 'panel-raise'
    case 'projects':
      return 'panel-projects'
    default:
      return 'panel'
  }
})
</script>

<template>
  <component :is="as" :class="variantClass" v-bind="$attrs">
    <slot />
  </component>
</template>
