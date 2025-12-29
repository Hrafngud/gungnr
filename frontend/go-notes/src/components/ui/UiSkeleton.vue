<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ inheritAttrs: false })

type SkeletonVariant = 'line' | 'block'

const props = withDefaults(defineProps<{
  variant?: SkeletonVariant
}>(), {
  variant: 'line',
})

const variantClass = computed(() => (props.variant === 'block' ? 'skeleton-block' : 'skeleton-line'))
</script>

<template>
  <div class="skeleton" :class="variantClass" v-bind="$attrs" />
</template>
