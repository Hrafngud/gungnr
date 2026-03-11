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
  <UiPanel class="flex items-center p-2">
    <div class="flex flex-row justify-between gap-2 w-full project-section-tabs">
      <UiButton
        v-for="tab in PROJECT_DETAIL_SECTION_TABS"
        :key="tab.id"
        :variant="modelValue === tab.id ? 'primary' : 'ghost'"
        size="sm"
        class="w-full"
        :disabled="disabled"
        @click="selectTab(tab.id)"
      >
        <span class="text-lg">{{ tab.label }}</span>
      </UiButton>
    </div>
  </UiPanel>
</template>

