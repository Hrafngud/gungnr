<script setup lang="ts">
import { computed } from 'vue'
import UiListRow from '@/components/ui/UiListRow.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

const props = withDefaults(defineProps<{
  label: string
  value: string
  copyable?: boolean
  copied?: boolean
}>(), {
  copyable: true,
  copied: false,
})

const emit = defineEmits<{
  copy: []
}>()

const tooltipText = computed(() => (
  props.copied ? 'Copied!' : 'Copy to clipboard.'
))
</script>

<template>
  <UiListRow class="networking-readonly-row">
    <span class="networking-readonly-row__label">{{ label }}</span>

    <UiTooltip
      v-if="copyable"
      class="networking-readonly-row__value-wrap"
      :text="tooltipText"
    >
      <button
        type="button"
        class="networking-readonly-row__value-btn"
        @click="emit('copy')"
      >
        <span class="networking-readonly-row__value-text">{{ value }}</span>
      </button>
    </UiTooltip>

    <span v-else class="networking-readonly-row__value-static">{{ value }}</span>
  </UiListRow>
</template>

<style scoped>
.networking-readonly-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.networking-readonly-row__label {
  color: var(--muted);
  font-size: 0.72rem;
  letter-spacing: 0.01em;
}

.networking-readonly-row__value-wrap {
  min-width: 0;
  max-width: min(34rem, 100%);
}

.networking-readonly-row__value-btn {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 0;
  color: var(--text);
  background: transparent;
  border: 0;
  padding: 0;
  font-size: 0.76rem;
  line-height: 1.35;
  letter-spacing: 0.01em;
  cursor: pointer;
  transform: translateX(0);
  transition:
    color 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

.networking-readonly-row__value-btn:hover,
.networking-readonly-row__value-btn:focus-visible {
  color: var(--accent-strong);
  transform: translateX(2px);
  outline: none;
}

.networking-readonly-row__value-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.networking-readonly-row__value-static {
  color: var(--text);
  font-size: 0.76rem;
  line-height: 1.35;
  letter-spacing: 0.01em;
  max-width: min(34rem, 100%);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 800px) {
  .networking-readonly-row__value-wrap,
  .networking-readonly-row__value-static {
    width: 100%;
    max-width: none;
  }

  .networking-readonly-row__value-btn {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
