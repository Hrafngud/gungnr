<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UiInlineSpinner from '@/components/ui/UiInlineSpinner.vue'
import UiInput from '@/components/ui/UiInput.vue'

type UiSelectTypingStatus = 'idle' | 'loading' | 'ready' | 'error'

interface UiSelectTypingOption {
  key: string
  value: string
  label: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options?: UiSelectTypingOption[]
    status?: UiSelectTypingStatus
    disabled?: boolean
    busy?: boolean
    placeholder?: string
    inputType?: string
    min?: string | number
    max?: string | number
    step?: string | number
    showAction?: boolean
    actionLabel?: string
    toggleAriaLabel?: string
    canRequestOptions?: boolean
    loadingText?: string
    emptyMessage?: string
  }>(),
  {
    modelValue: '',
    options: () => [],
    status: 'idle',
    disabled: false,
    busy: false,
    placeholder: '',
    inputType: 'text',
    min: undefined,
    max: undefined,
    step: undefined,
    showAction: false,
    actionLabel: 'Reset',
    toggleAriaLabel: 'Toggle options',
    canRequestOptions: true,
    loadingText: 'Loading options...',
    emptyMessage: 'No options available.',
  },
)

const emit = defineEmits<{
  'update:modelValue': [string]
  'request-options': []
  'commit': []
  'action': []
}>()

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const focusStartValue = ref(props.modelValue)

const loadingOptions = computed(() => props.status === 'loading')
const controlsDisabled = computed(() => props.disabled || props.busy)
const shouldRequestOptions = computed(
  () =>
    props.canRequestOptions &&
    !controlsDisabled.value &&
    (props.status === 'idle' || props.status === 'error'),
)

watch(
  () => props.modelValue,
  (value) => {
    if (!open.value) {
      focusStartValue.value = value
    }
  },
)

watch(controlsDisabled, (value) => {
  if (value) {
    open.value = false
  }
})

const requestOptionsIfNeeded = () => {
  if (shouldRequestOptions.value) {
    emit('request-options')
  }
}

const openMenu = () => {
  if (controlsDisabled.value) return
  open.value = true
  requestOptionsIfNeeded()
}

const closeMenu = () => {
  open.value = false
}

const commitIfChanged = (force = false) => {
  if (controlsDisabled.value) return

  const nextValue = props.modelValue.trim()
  if (!nextValue) return

  if (!force && nextValue === focusStartValue.value.trim()) {
    return
  }

  emit('commit')
  focusStartValue.value = nextValue
}

const onFocus = () => {
  focusStartValue.value = props.modelValue
  openMenu()
}

const onInputClick = () => {
  openMenu()
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    closeMenu()
    return
  }

  if (event.key === 'ArrowDown') {
    openMenu()
    return
  }

  if (event.key === 'Enter') {
    event.preventDefault()
    commitIfChanged(true)
    closeMenu()
  }
}

const onFocusOut = (event: FocusEvent) => {
  const relatedTarget = event.relatedTarget as Node | null
  if (relatedTarget && rootRef.value?.contains(relatedTarget)) {
    return
  }

  closeMenu()
  commitIfChanged(false)
}

const selectOptionValue = (value: string) => {
  if (controlsDisabled.value) return

  emit('update:modelValue', value)
  emit('commit')
  focusStartValue.value = value
  closeMenu()
}

const triggerAction = () => {
  if (controlsDisabled.value || !props.showAction) return

  emit('action')
  focusStartValue.value = ''
  closeMenu()
}
</script>

<template>
  <div
    ref="rootRef"
    class="relative w-full"
    @focusout="onFocusOut"
  >
    <div class="relative">
      <UiInput
        :model-value="modelValue"
        :type="inputType"
        :min="min"
        :max="max"
        :step="step"
        :placeholder="placeholder"
        :disabled="controlsDisabled"
        class="pr-10"
        @update:model-value="emit('update:modelValue', $event)"
        @focus="onFocus"
        @click="onInputClick"
        @keydown="onKeydown"
      />
      <button
        type="button"
        class="absolute right-2 top-1/2 inline-flex -translate-y-1/2 cursor-pointer items-center justify-center rounded p-1 text-[color:var(--muted)] transition hover:text-[color:var(--text)] disabled:cursor-not-allowed disabled:opacity-60"
        :disabled="controlsDisabled"
        :aria-label="toggleAriaLabel"
        @click="open ? closeMenu() : openMenu()"
      >
        <UiInlineSpinner v-if="loadingOptions || busy" />
        <svg
          v-else
          viewBox="0 0 20 20"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          class="h-3.5 w-3.5"
        >
          <polyline points="6 8 10 12 14 8" />
        </svg>
      </button>
    </div>

    <div
      v-if="open"
      class="absolute z-40 mt-2 w-full rounded-lg border border-[color:var(--border)] bg-[color:var(--surface-2)] p-1 shadow-sm"
    >
      <slot
        v-if="showAction"
        name="action"
        :disabled="controlsDisabled"
        :trigger-action="triggerAction"
      >
        <button
          type="button"
          class="w-full cursor-pointer rounded px-2 py-2 text-left text-xs text-[color:var(--text)] transition hover:bg-[color:var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="controlsDisabled"
          @mousedown.prevent
          @click="triggerAction"
        >
          {{ actionLabel }}
        </button>
      </slot>

      <div
        v-if="loadingOptions"
        class="inline-flex w-full items-center gap-2 px-2 py-2 text-xs text-[color:var(--muted)]"
      >
        <UiInlineSpinner />
        {{ loadingText }}
      </div>

      <template v-else>
        <button
          v-for="option in options"
          :key="option.key"
          type="button"
          class="w-full cursor-pointer rounded px-2 py-2 text-left text-xs text-[color:var(--text)] transition hover:bg-[color:var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="controlsDisabled"
          @mousedown.prevent
          @click="selectOptionValue(option.value)"
        >
          <slot
            name="option"
            :option="option"
          >
            {{ option.label }}
          </slot>
        </button>

        <p
          v-if="options.length === 0"
          class="px-2 py-2 text-xs text-[color:var(--muted)]"
        >
          {{ emptyMessage }}
        </p>
      </template>
    </div>
  </div>
</template>
