<script setup lang="ts">
import { computed } from 'vue'
import UiStatusDot from '@/components/ui/UiStatusDot.vue'

defineOptions({ inheritAttrs: false })

type StatusDotTone = 'neutral' | 'ok' | 'warn' | 'error'

const props = withDefaults(defineProps<{
  modelValue: boolean
  label: string
  statusTone?: StatusDotTone
  statusLabel?: string
  disabled?: boolean
}>(), {
  modelValue: false,
  statusTone: 'neutral',
  statusLabel: '',
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [boolean]
}>()

const toggle = () => {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}

const buttonClass = computed(() =>
  props.modelValue ? 'ui-status-toggle-button--selected' : 'ui-status-toggle-button--idle',
)
</script>

<template>
  <button
    type="button"
    class="ui-status-toggle-button"
    :class="buttonClass"
    :aria-pressed="modelValue ? 'true' : 'false'"
    :disabled="disabled"
    @click="toggle"
    v-bind="$attrs"
  >
    <span class="ui-status-toggle-button__label" :title="label">{{ label }}</span>
    <span class="ui-status-toggle-button__meta">
      <UiStatusDot :tone="statusTone" size="sm" />
      <span v-if="statusLabel" class="ui-status-toggle-button__status">{{ statusLabel }}</span>
    </span>
  </button>
</template>

<style scoped>
.ui-status-toggle-button {
  width: 100%;
  min-width: 0;
  min-height: 2.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.55rem;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  padding: 0.5rem 0.65rem;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.ui-status-toggle-button:hover:not(:disabled) {
  border-color: var(--border-soft);
  background: var(--surface-2);
}

.ui-status-toggle-button:focus-visible {
  outline: none;
  border-color: var(--accent);
}

.ui-status-toggle-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.ui-status-toggle-button--idle {
  border-color: var(--border);
  background: var(--surface);
}

.ui-status-toggle-button--selected {
  border-color: color-mix(in oklab, var(--accent) 68%, var(--border));
  background: color-mix(in oklab, var(--accent) 14%, var(--surface-2));
}

.ui-status-toggle-button__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.82rem;
  text-align: left;
}

.ui-status-toggle-button__meta {
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  flex: 0 0 auto;
}

.ui-status-toggle-button__status {
  font-size: 0.61rem;
  line-height: 1;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted-2);
}
</style>
