<script setup lang="ts">
defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: boolean
  disabled?: boolean
}>(), {
  modelValue: false,
})

const emit = defineEmits<{
  'update:modelValue': [boolean]
}>()

const toggle = () => {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>

<template>
  <button
    type="button"
    role="switch"
    class="toggle"
    :class="modelValue ? 'toggle-on' : 'toggle-off'"
    :aria-checked="modelValue"
    :disabled="disabled"
    @click="toggle"
    v-bind="$attrs"
  >
    <span class="toggle-track">
      <span class="toggle-thumb" />
    </span>
    <span v-if="$slots.default" class="toggle-label">
      <slot />
    </span>
  </button>
</template>
