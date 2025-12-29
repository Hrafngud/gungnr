<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  description?: string
  variant?: 'modal' | 'sheet-right' | 'sheet-bottom'
  closeOnBackdrop?: boolean
}>(), {
  variant: 'modal',
  closeOnBackdrop: true,
})

const emit = defineEmits<{
  'update:modelValue': [boolean]
}>()

const close = () => {
  emit('update:modelValue', false)
}

const onBackdropClick = () => {
  if (props.closeOnBackdrop) {
    close()
  }
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    close()
  }
}

const panelClass = computed(() => ({
  'modal-panel': true,
  'modal-sheet-right': props.variant === 'sheet-right',
  'modal-sheet-bottom': props.variant === 'sheet-bottom',
}))

const lockBody = (lock: boolean) => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = lock ? 'hidden' : ''
}

watch(
  () => props.modelValue,
  (open) => {
    if (typeof window !== 'undefined') {
      if (open) {
        window.addEventListener('keydown', onKeydown)
      } else {
        window.removeEventListener('keydown', onKeydown)
      }
    }
    lockBody(open)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', onKeydown)
  }
  lockBody(false)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div
        v-if="modelValue"
        class="modal-overlay"
        role="presentation"
        @click="onBackdropClick"
      >
        <div
          :class="panelClass"
          role="dialog"
          aria-modal="true"
          @click.stop
          v-bind="$attrs"
        >
          <header v-if="title || $slots.header || description" class="modal-header">
            <slot name="header">
              <div class="space-y-2">
                <h2 v-if="title" class="text-lg font-semibold text-[color:var(--text)]">
                  {{ title }}
                </h2>
                <p v-if="description" class="text-xs text-[color:var(--muted)]">
                  {{ description }}
                </p>
              </div>
            </slot>
          </header>
          <div class="modal-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="modal-footer">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
