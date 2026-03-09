<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import UiPanel from '@/components/ui/UiPanel.vue'
import { PROJECT_DETAIL_SECTION_TABS, type ProjectDetailSectionTab } from '@/composables/projectDetail/useProjectDetailTabs'

const props = withDefaults(defineProps<{
  modelValue: ProjectDetailSectionTab
  disabled?: boolean
}>(), {
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [ProjectDetailSectionTab]
}>()

function selectTab(tab: ProjectDetailSectionTab) {
  if (props.disabled || props.modelValue === tab) return
  emit('update:modelValue', tab)
}
</script>

<template>
  <UiPanel class="project-section-tabs p-2">
    <div class="project-section-tabs__rail">
      <UiButton
        v-for="tab in PROJECT_DETAIL_SECTION_TABS"
        :key="tab.id"
        :variant="modelValue === tab.id ? 'primary' : 'ghost'"
        size="sm"
        class="project-section-tabs__button"
        :disabled="disabled"
        @click="selectTab(tab.id)"
      >
        <span class="project-section-tabs__label">{{ tab.label }}</span>
      </UiButton>
    </div>
  </UiPanel>
</template>

<style scoped>
.project-section-tabs__rail {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
  scrollbar-width: thin;
}

.project-section-tabs__button {
  white-space: nowrap;
}

.project-section-tabs__label {
  letter-spacing: 0.01em;
}
</style>
