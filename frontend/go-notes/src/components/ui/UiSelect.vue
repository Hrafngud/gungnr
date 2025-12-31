<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

type SelectOption = {
  value: string | number
  label: string
  disabled?: boolean
}

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: string | number
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
}>(), {
  options: () => [],
  placeholder: 'Select an option',
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [string | number]
}>()

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const highlightedIndex = ref(-1)

const selectedOption = computed(() =>
  props.options.find((option) => option.value === props.modelValue),
)

const displayLabel = computed(() =>
  selectedOption.value?.label ?? props.placeholder,
)

const isPlaceholder = computed(() => !selectedOption.value)

const findNextEnabledIndex = (start: number, direction: 1 | -1) => {
  if (props.options.length === 0) return -1
  let index = start
  for (let step = 0; step < props.options.length; step += 1) {
    index = (index + direction + props.options.length) % props.options.length
    if (!props.options[index]?.disabled) {
      return index
    }
  }
  return -1
}

const openMenu = () => {
  if (props.disabled) return
  open.value = true
  const selectedIndex = props.options.findIndex(
    (option) => option.value === props.modelValue && !option.disabled,
  )
  if (selectedIndex >= 0) {
    highlightedIndex.value = selectedIndex
  } else {
    highlightedIndex.value = findNextEnabledIndex(-1, 1)
  }
}

const closeMenu = () => {
  open.value = false
  highlightedIndex.value = -1
}

const toggleMenu = () => {
  if (open.value) {
    closeMenu()
  } else {
    openMenu()
  }
}

const selectOption = (option: SelectOption) => {
  if (option.disabled) return
  emit('update:modelValue', option.value)
  closeMenu()
}

const onKeydown = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (!open.value && ['Enter', ' ', 'ArrowDown'].includes(event.key)) {
    event.preventDefault()
    openMenu()
    return
  }

  if (!open.value) return

  if (event.key === 'Escape') {
    event.preventDefault()
    closeMenu()
    return
  }

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    highlightedIndex.value = findNextEnabledIndex(highlightedIndex.value, 1)
    return
  }

  if (event.key === 'ArrowUp') {
    event.preventDefault()
    highlightedIndex.value = findNextEnabledIndex(highlightedIndex.value, -1)
    return
  }

  if (event.key === 'Enter') {
    event.preventDefault()
    const option = props.options[highlightedIndex.value]
    if (option) {
      selectOption(option)
    }
  }
}

const onClickOutside = (event: MouseEvent) => {
  const target = event.target as Node | null
  if (!rootRef.value || !target) return
  if (!rootRef.value.contains(target)) {
    closeMenu()
  }
}

onMounted(() => {
  document.addEventListener('mousedown', onClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onClickOutside)
})
</script>

<template>
  <div ref="rootRef" class="select-root" v-bind="$attrs">
    <button
      type="button"
      class="input select-trigger"
      :disabled="disabled"
      aria-haspopup="listbox"
      :aria-expanded="open ? 'true' : 'false'"
      @click="toggleMenu"
      @keydown="onKeydown"
    >
      <span class="select-label" :class="isPlaceholder ? 'select-placeholder' : ''">
        {{ displayLabel }}
      </span>
      <svg
        class="select-caret"
        viewBox="0 0 20 20"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <polyline points="6 8 10 12 14 8" />
      </svg>
    </button>

    <Transition name="select-fade">
      <div v-if="open" class="select-menu" role="listbox">
        <p v-if="options.length === 0" class="select-empty">
          No options available
        </p>
        <button
          v-for="(option, index) in options"
          :key="option.value"
          type="button"
          role="option"
          class="select-option"
          :class="{
            'select-option-active': index === highlightedIndex,
            'select-option-selected': option.value === modelValue,
          }"
          :disabled="option.disabled"
          :aria-selected="option.value === modelValue ? 'true' : 'false'"
          @mouseenter="highlightedIndex = index"
          @click="selectOption(option)"
        >
          {{ option.label }}
        </button>
      </div>
    </Transition>
  </div>
</template>
