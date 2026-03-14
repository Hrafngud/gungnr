<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

defineOptions({ inheritAttrs: false })

type ListRowVariant = 'default' | 'card'

const props = withDefaults(defineProps<{
  as?: string | Component
  variant?: ListRowVariant
}>(), {
  as: 'div',
  variant: 'default',
})

const variantClass = computed(() => {
  if (props.variant === 'card') return 'list-row-card'
  return 'list-row'
})
</script>

<template>
  <component :is="as" :class="variantClass" v-bind="$attrs">
    <slot />
  </component>
</template>
