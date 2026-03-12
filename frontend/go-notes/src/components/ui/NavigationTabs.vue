<script setup lang="ts">
type NavigationTab = {
  id: string
  label: string
  description?: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  tabs: readonly NavigationTab[]
  disabled?: boolean
  ariaLabel?: string
}>(), {
  disabled: false,
  ariaLabel: 'Navigation tabs',
})

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

function selectTab(tabId: string) {
  if (props.disabled || props.modelValue === tabId) return
  emit('update:modelValue', tabId)
}
</script>

<template>
  <nav
    class="navigation-tabs"
    :class="{ 'navigation-tabs--disabled': disabled }"
    :aria-label="ariaLabel"
  >
    <div class="navigation-tabs__list">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="navigation-tabs__tab"
        :class="{ 'navigation-tabs__tab--active': modelValue === tab.id }"
        :aria-pressed="modelValue === tab.id ? 'true' : 'false'"
        :disabled="disabled"
        :title="tab.description || tab.label"
        @click="selectTab(tab.id)"
      >
        <span class="navigation-tabs__label">{{ tab.label }}</span>
      </button>
    </div>
  </nav>
</template>

<style scoped>
.navigation-tabs {
  border-bottom: 1px solid var(--border);
}

.navigation-tabs__list {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.28rem;
}

.navigation-tabs__tab {
  position: relative;
  flex: 1 1 9rem;
  min-width: 0;
  border: 1px solid transparent;
  border-bottom: none;
  border-radius: 0.55rem 0.55rem 0 0;
  background: transparent;
  color: var(--muted);
  padding: 0.62rem 0.95rem;
  font-size: 0.95rem;
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: 0.01em;
  transition:
    color 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.navigation-tabs__tab:hover:not(:disabled) {
  color: var(--accent-ink);
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
}

.navigation-tabs__tab:focus-visible {
  outline: none;
  border-color: color-mix(in srgb, var(--accent) 64%, var(--border));
  color: var(--text);
}

.navigation-tabs__tab--active {
  border-color: var(--border);
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
  color: var(--accent-ink);
}

.navigation-tabs__tab--active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 1px;
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
}

.navigation-tabs__tab:disabled {
  cursor: not-allowed;
}

.navigation-tabs__label {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.navigation-tabs--disabled .navigation-tabs__tab {
  opacity: 0.64;
}

@media (max-width: 640px) {
  .navigation-tabs__tab {
    flex-basis: calc(50% - 0.14rem);
    padding: 0.58rem 0.72rem;
    font-size: 0.86rem;
  }
}
</style>
