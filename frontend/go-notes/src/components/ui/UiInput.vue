<script setup lang="ts">
defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: string | number
  type?: string
  disabled?: boolean
}>(), {
  type: 'text',
})

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

const onInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <input
    class="input"
    :type="type"
    :value="modelValue ?? ''"
    :disabled="disabled"
    @input="onInput"
    v-bind="$attrs"
  />
</template>
