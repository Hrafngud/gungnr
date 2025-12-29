<script setup lang="ts">
defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: string | number
  disabled?: boolean
}>(), {})

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

const onChange = (event: Event) => {
  const target = event.target as HTMLSelectElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <select
    class="input select"
    :value="modelValue ?? ''"
    :disabled="disabled"
    @change="onChange"
    v-bind="$attrs"
  >
    <slot />
  </select>
</template>
