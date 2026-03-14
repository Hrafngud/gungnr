<script setup lang="ts">
import { computed } from 'vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  value: string
  copyable?: boolean
  copied?: boolean
  tooltipText?: string
  copiedTooltipText?: string
  buttonClass?: string
  valueClass?: string
  staticClass?: string
}>(), {
  copyable: true,
  copied: false,
  tooltipText: 'Copy to clipboard.',
  copiedTooltipText: 'Copied!',
  buttonClass: '',
  valueClass: '',
  staticClass: '',
})

const emit = defineEmits<{
  copy: []
}>()

const resolvedTooltipText = computed(() => (
  props.copied ? props.copiedTooltipText : props.tooltipText
))
</script>

<template>
  <UiTooltip
    v-if="copyable"
    :text="resolvedTooltipText"
    v-bind="$attrs"
  >
    <button
      type="button"
      :class="['ui-copyable-value__button', buttonClass]"
      :title="value"
      @click="emit('copy')"
    >
      <span :class="['ui-copyable-value__text', valueClass]">{{ value }}</span>
    </button>
  </UiTooltip>

  <span
    v-else
    :class="['ui-copyable-value__static', staticClass]"
    :title="value"
    v-bind="$attrs"
  >
    {{ value }}
  </span>
</template>

<style scoped>
.ui-copyable-value__button {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 0;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transform: translateX(0);
  transition:
    color 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

.ui-copyable-value__button:hover,
.ui-copyable-value__button:focus-visible {
  color: var(--accent-strong);
  transform: translateX(2px);
  outline: none;
}

.ui-copyable-value__text,
.ui-copyable-value__static {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
