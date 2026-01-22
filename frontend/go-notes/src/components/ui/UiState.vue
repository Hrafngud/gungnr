<script setup lang="ts">
import { computed } from 'vue'
import UiHeroMark from '@/components/ui/UiHeroMark.vue'

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
    <UiHeroMark v-if="loading" class="state-spinner" />
    <slot />
  </div>
</template>
