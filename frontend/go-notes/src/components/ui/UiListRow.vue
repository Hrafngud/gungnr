<script setup lang="ts">
import type { Component } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  as?: string | Component
}>(), {
  as: 'div',
})
</script>

<template>
  <component :is="as" class="list-row" v-bind="$attrs">
    <slot />
  </component>
</template>
