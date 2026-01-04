<script setup lang="ts">
import { ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  eyebrow?: string
}>(), {
  eyebrow: 'Form',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const isOpen = ref(props.modelValue)

watch(() => props.modelValue, (value) => {
  isOpen.value = value
})

const close = () => {
  isOpen.value = false
  emit('update:modelValue', false)
}

const onOverlayClick = (event: MouseEvent) => {
  if (event.target === event.currentTarget) {
    close()
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div
        v-if="isOpen"
        class="modal-overlay"
        @click="onOverlayClick"
      >
        <Transition name="panel-slide-right">
          <div
            v-if="isOpen"
            class="form-side-panel"
            @click.stop
          >
            <div class="form-side-panel-header">
              <div v-if="title">
                <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--muted-2)]">
                  {{ props.eyebrow }}
                </p>
                <h2 class="mt-1 text-lg font-semibold text-[color:var(--text)]">
                  {{ title }}
                </h2>
              </div>
              <button
                type="button"
                class="btn btn-ghost p-2"
                aria-label="Close form panel"
                @click="close"
              >
                <svg
                  class="h-5 w-5"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>
            <div class="form-side-panel-body">
              <slot />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
