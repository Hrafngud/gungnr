<script setup lang="ts">
import NavigationTabs from '@/components/ui/NavigationTabs.vue'
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

function onSelectTab(tabId: string) {
  selectTab(tabId as ProjectDetailSectionTab)
}
</script>
<template>
  <NavigationTabs
    :model-value="modelValue"
    :tabs="PROJECT_DETAIL_SECTION_TABS"
    :disabled="disabled"
    aria-label="Project section navigation"
    @update:model-value="onSelectTab"
  />
</template>
