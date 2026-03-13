<script setup lang="ts">
import type { IconName } from '@/components/NavIcon.vue'
import NavIcon from '@/components/NavIcon.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

type NavigationTab = {
  id: string
  label: string
  icon?: IconName
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
      <UiTooltip
        v-for="tab in tabs"
        :key="tab.id"
        as="span"
        placement="auto"
        class="navigation-tabs__tooltip-host"
        :text="tab.description || tab.label"
      >
        <button
          type="button"
          class="navigation-tabs__tab"
          :class="{ 'navigation-tabs__tab--active': modelValue === tab.id }"
          :aria-pressed="modelValue === tab.id ? 'true' : 'false'"
          :disabled="disabled"
          @click="selectTab(tab.id)"
        >
          <span class="navigation-tabs__content">
            <NavIcon
              v-if="tab.icon"
              :name="tab.icon"
              class="navigation-tabs__icon"
            />
            <span class="navigation-tabs__label">{{ tab.label }}</span>
          </span>
        </button>
      </UiTooltip>
    </div>
  </nav>
</template>

<style scoped>
.navigation-tabs {
  --navigation-tab-width: clamp(14rem, 21vw, 18rem);
  position: relative;
}

.navigation-tabs::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: var(--border);
  pointer-events: none;
}

.navigation-tabs__list {
  display: flex;
  flex-wrap: nowrap;
  align-items: flex-end;
  position: relative;
  z-index: 1;
  gap: 0.32rem;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  scrollbar-width: thin;
}

:deep(.navigation-tabs__tooltip-host) {
  flex: 0 0 var(--navigation-tab-width);
  min-width: var(--navigation-tab-width);
  display: flex;
}

.navigation-tabs__tab {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1 1 auto;
  min-width: var(--navigation-tab-width);
  width: var(--navigation-tab-width);
  border: 1px solid transparent;
  border-bottom: 1px solid transparent;
  border-radius: 0.55rem 0.55rem 0 0;
  background: transparent;
  color: var(--muted);
  margin-bottom: 0;
  padding: 0.52rem 1.5rem;
  font-size: 0.92rem;
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: 0.01em;
  z-index: 1;
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
  border-bottom-color: var(--surface);
  background: var(--surface);
  color: var(--accent-ink);
  margin-bottom: -1px;
  z-index: 3;
}

.navigation-tabs__tab:disabled {
  cursor: not-allowed;
}

.navigation-tabs__label {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.navigation-tabs__content {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.46rem;
  min-width: 0;
  width: 100%;
}

.navigation-tabs__icon {
  width: 0.9rem;
  height: 0.9rem;
  flex: none;
  opacity: 0.88;
}

.navigation-tabs--disabled .navigation-tabs__tab {
  opacity: 0.64;
}

@media (max-width: 640px) {
  .navigation-tabs {
    --navigation-tab-width: 11rem;
  }

  .navigation-tabs__tab {
    padding: 0.5rem 0.86rem;
    font-size: 0.86rem;
  }
}
</style>
