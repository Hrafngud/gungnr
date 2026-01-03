<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

defineOptions({ inheritAttrs: false })

type ButtonVariant = 'default' | 'primary' | 'ghost' | 'chip' | 'danger'
type ButtonSize = 'md' | 'sm' | 'xs' | 'chip'

const props = withDefaults(defineProps<{
  as?: string | Component
  variant?: ButtonVariant
  size?: ButtonSize
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
}>(), {
  as: 'button',
  variant: 'default',
  size: 'md',
  type: 'button',
})

const isNativeButton = computed(() => typeof props.as === 'string' && props.as === 'button')

const baseClass = computed(() => (props.variant === 'chip' ? 'chip' : 'btn'))

const variantClass = computed(() => {
  switch (props.variant) {
    case 'primary':
      return 'btn-primary'
    case 'danger':
      return 'btn-danger'
    case 'ghost':
      return 'btn-ghost'
    default:
      return ''
  }
})

const sizeClass = computed(() => {
  switch (props.size) {
    case 'xs':
      return 'px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.2em]'
    case 'sm':
      return 'px-3 py-2 text-xs font-semibold'
    case 'chip':
      return 'px-3 py-1 text-xs font-semibold'
    default:
      return 'px-4 py-2 text-xs font-semibold'
  }
})
</script>

<template>
  <component
    :is="as"
    :type="isNativeButton ? type : undefined"
    :disabled="isNativeButton ? disabled : undefined"
    :aria-disabled="!isNativeButton && disabled ? 'true' : undefined"
    :tabindex="!isNativeButton && disabled ? -1 : undefined"
    :class="[baseClass, variantClass, sizeClass, disabled ? 'opacity-60 cursor-not-allowed' : '']"
    v-bind="$attrs"
  >
    <slot />
  </component>
</template>
