<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import type { OnboardingStep } from '@/types/onboarding'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: boolean
  steps: OnboardingStep[]
  stepIndex: number
  closeOnBackdrop?: boolean
}>(), {
  closeOnBackdrop: true,
})

const emit = defineEmits<{
  'update:modelValue': [boolean]
  'update:stepIndex': [number]
  finish: []
  skip: []
}>()

const activeStep = computed(() => props.steps[props.stepIndex] ?? null)
const targetRect = ref<DOMRect | null>(null)
const viewport = ref({ width: 0, height: 0 })

const hasPrev = computed(() => props.stepIndex > 0)
const hasNext = computed(() => props.stepIndex < props.steps.length - 1)

const updateViewport = () => {
  if (typeof window === 'undefined') return
  viewport.value = { width: window.innerWidth, height: window.innerHeight }
}

const updateTarget = () => {
  if (typeof document === 'undefined') return
  const selector = activeStep.value?.target
  if (!selector) {
    targetRect.value = null
    return
  }
  const element = document.querySelector(selector)
  if (!element) {
    targetRect.value = null
    return
  }
  targetRect.value = element.getBoundingClientRect()
}

const scheduleUpdate = () => {
  if (!props.modelValue) return
  nextTick(() => {
    updateViewport()
    updateTarget()
  })
}

const close = () => {
  emit('update:modelValue', false)
}

const skip = () => {
  emit('skip')
  close()
}

const finish = () => {
  emit('finish')
  close()
}

const nextStep = () => {
  if (hasNext.value) {
    emit('update:stepIndex', props.stepIndex + 1)
  } else {
    finish()
  }
}

const prevStep = () => {
  if (hasPrev.value) {
    emit('update:stepIndex', props.stepIndex - 1)
  }
}

const onBackdropClick = () => {
  if (props.closeOnBackdrop) {
    skip()
  }
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    skip()
  }
}

const lockBody = (lock: boolean) => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = lock ? 'hidden' : ''
}

const highlightStyle = computed(() => {
  if (!targetRect.value) return {}
  const pad = 10
  const top = Math.max(8, targetRect.value.top - pad)
  const left = Math.max(8, targetRect.value.left - pad)
  const width = Math.min(
    targetRect.value.width + pad * 2,
    viewport.value.width - 16,
  )
  const height = Math.min(
    targetRect.value.height + pad * 2,
    viewport.value.height - 16,
  )
  return {
    top: `${top}px`,
    left: `${left}px`,
    width: `${width}px`,
    height: `${height}px`,
  }
})

const cardStyle = computed(() => {
  const width = Math.min(340, Math.max(260, viewport.value.width - 32))
  if (!targetRect.value) {
    return {
      width: `${width}px`,
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
    }
  }

  const gutter = 16
  const estimatedHeight = 230
  const rect = targetRect.value
  const preferBelow = rect.bottom + estimatedHeight + gutter < viewport.value.height
  const top = preferBelow
    ? rect.bottom + gutter
    : Math.max(gutter, rect.top - estimatedHeight - gutter)
  const left = Math.min(
    Math.max(gutter, rect.left + rect.width / 2 - width / 2),
    viewport.value.width - width - gutter,
  )

  return {
    width: `${width}px`,
    top: `${top}px`,
    left: `${left}px`,
  }
})

watch(
  () => props.modelValue,
  (open) => {
    if (typeof window !== 'undefined') {
      if (open) {
        window.addEventListener('keydown', onKeydown)
        window.addEventListener('resize', scheduleUpdate)
        window.addEventListener('scroll', scheduleUpdate, true)
      } else {
        window.removeEventListener('keydown', onKeydown)
        window.removeEventListener('resize', scheduleUpdate)
        window.removeEventListener('scroll', scheduleUpdate, true)
      }
    }
    lockBody(open)
    if (open) {
      scheduleUpdate()
    } else {
      targetRect.value = null
    }
  },
  { immediate: true },
)

watch(
  () => props.stepIndex,
  () => {
    scheduleUpdate()
  },
)

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', onKeydown)
    window.removeEventListener('resize', scheduleUpdate)
    window.removeEventListener('scroll', scheduleUpdate, true)
  }
  lockBody(false)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div
        v-if="modelValue"
        class="onboarding-overlay"
        role="presentation"
        @click="onBackdropClick"
      >
        <div
          v-if="targetRect"
          class="onboarding-highlight"
          :style="highlightStyle"
        />

        <div
          class="onboarding-card"
          role="dialog"
          aria-live="polite"
          :style="cardStyle"
          @click.stop
        >
          <div class="flex items-center justify-between gap-3">
            <div class="space-y-1">
              <p class="onboarding-progress">
                Step {{ stepIndex + 1 }} of {{ steps.length }}
              </p>
              <h3 class="text-base font-semibold text-[color:var(--text)]">
                {{ activeStep?.title }}
              </h3>
            </div>
            <UiBadge tone="neutral">Onboarding</UiBadge>
          </div>

          <p class="mt-3 text-xs text-[color:var(--muted)]">
            {{ activeStep?.description }}
          </p>

          <p v-if="activeStep?.hint" class="mt-2 text-xs text-[color:var(--muted)]">
            {{ activeStep.hint }}
          </p>

          <div v-if="activeStep?.links?.length" class="onboarding-links mt-3 space-y-2 text-xs">
            <p class="text-[color:var(--muted-2)]">API key shortcuts</p>
            <div class="grid gap-2">
              <a
                v-for="link in activeStep.links"
                :key="link.href"
                :href="link.href"
                target="_blank"
                rel="noreferrer"
                class="inline-flex items-center justify-between rounded-2xl border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-2 text-[color:var(--text)]"
              >
                <span>{{ link.label }}</span>
                <span class="text-[color:var(--muted-2)]">Open</span>
              </a>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
            <UiButton variant="ghost" size="xs" @click="skip">
              Skip for now
            </UiButton>
            <div class="flex items-center gap-2">
              <UiButton
                v-if="hasPrev"
                variant="ghost"
                size="sm"
                @click="prevStep"
              >
                Back
              </UiButton>
              <UiButton variant="primary" size="sm" @click="nextStep">
                {{ hasNext ? 'Next' : 'Finish' }}
              </UiButton>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
