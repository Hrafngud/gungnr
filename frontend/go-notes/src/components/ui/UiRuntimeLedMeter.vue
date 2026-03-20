<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { clampPercent, formatPercent } from '@/utils/runtimeMetrics'

type SegmentTone = 'ok' | 'warn' | 'error'

const props = withDefaults(defineProps<{
  label: string
  percent: number | null | undefined
  segmentCount?: number
  warnThreshold?: number
  errorThreshold?: number
}>(), {
  segmentCount: 48,
  warnThreshold: 55,
  errorThreshold: 80,
})

const normalizedPercent = computed(() => clampPercent(props.percent))
const ariaLabel = computed(() => `${props.label} ${formatPercent(props.percent)}`)
const meterStyle = computed(() => ({ '--runtime-led-segments': String(props.segmentCount) }))
const prefersReducedMotion = ref(false)
const animatedPercent = ref(0)
const isAnimating = ref(false)
const isMounted = ref(false)

let animationFrameId: number | null = null
let motionQuery: MediaQueryList | null = null

const displayPercent = computed(() => (isMounted.value ? animatedPercent.value : normalizedPercent.value))

const easeOutQuint = (progress: number) => 1 - Math.pow(1 - progress, 5)

const stopAnimation = () => {
  if (animationFrameId !== null) {
    cancelAnimationFrame(animationFrameId)
    animationFrameId = null
  }
}

const syncMotionPreference = () => {
  prefersReducedMotion.value = Boolean(motionQuery?.matches)
}

const motionPreferenceChange = () => {
  syncMotionPreference()
  if (prefersReducedMotion.value) {
    stopAnimation()
    isAnimating.value = false
    animatedPercent.value = normalizedPercent.value
  }
}

const resolveAnimationDuration = (from: number, to: number) => {
  const delta = Math.abs(to - from)
  if (delta < 0.5) return 120
  if (delta < 10) return 220
  return 680
}

const animateTo = (target: number, immediate = false) => {
  stopAnimation()
  if (immediate || prefersReducedMotion.value) {
    animatedPercent.value = target
    isAnimating.value = false
    return
  }

  const from = animatedPercent.value
  if (Math.abs(target - from) < 0.05) {
    animatedPercent.value = target
    isAnimating.value = false
    return
  }

  const duration = resolveAnimationDuration(from, target)
  const startedAt = performance.now()
  isAnimating.value = true

  const tick = (now: number) => {
    const elapsed = now - startedAt
    const progress = Math.min(elapsed / duration, 1)
    const eased = easeOutQuint(progress)
    animatedPercent.value = from + (target - from) * eased

    if (progress >= 1) {
      animatedPercent.value = target
      isAnimating.value = false
      animationFrameId = null
      return
    }
    animationFrameId = requestAnimationFrame(tick)
  }

  animationFrameId = requestAnimationFrame(tick)
}

onMounted(() => {
  isMounted.value = true
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  syncMotionPreference()
  motionQuery.addEventListener('change', motionPreferenceChange)
  animatedPercent.value = 0
  animateTo(normalizedPercent.value, prefersReducedMotion.value)
})

watch(normalizedPercent, (next) => {
  if (!isMounted.value) return
  animateTo(next, prefersReducedMotion.value)
})

onBeforeUnmount(() => {
  stopAnimation()
  if (!motionQuery) return
  motionQuery.removeEventListener('change', motionPreferenceChange)
})

const segments = computed(() =>
  Array.from({ length: props.segmentCount }, (_, index) => {
    const threshold = ((index + 1) / props.segmentCount) * 100
    let tone: SegmentTone = 'error'
    if (threshold <= props.warnThreshold) {
      tone = 'ok'
    } else if (threshold <= props.errorThreshold) {
      tone = 'warn'
    }
    return {
      key: `${props.label}-${index}`,
      active: threshold <= displayPercent.value,
      tone,
    }
  }),
)
</script>

<template>
  <div class="runtime-led-bar" :class="{ 'is-animating': isAnimating }" role="img" :aria-label="ariaLabel" :style="meterStyle">
    <span
      v-for="segment in segments"
      :key="segment.key"
      :class="[
        'runtime-led-segment',
        segment.active ? `is-active tone-${segment.tone}` : 'is-idle',
      ]"
    />
  </div>
</template>

<style scoped>
.runtime-led-bar {
  display: grid;
  grid-template-columns: repeat(var(--runtime-led-segments, 48), minmax(0, 1fr));
  gap: 2px;
  border: 1px solid color-mix(in oklch, var(--border-soft) 78%, black);
  background: color-mix(in oklch, var(--surface-inset) 84%, black);
  border-radius: 4px;
  padding: 4px;
}

.runtime-led-segment {
  display: block;
  height: 10px;
  border-radius: 1px;
  background: color-mix(in oklch, var(--surface-3) 82%, black);
  transition: background-color 0.18s ease, opacity 0.18s ease;
}

.runtime-led-bar.is-animating .runtime-led-segment {
  will-change: background-color, opacity;
}

.runtime-led-segment.is-idle {
  opacity: 0.38;
}

.runtime-led-segment.is-active {
  opacity: 1;
}

.runtime-led-segment.is-active.tone-ok {
  background: color-mix(in oklch, var(--success) 86%, #29bf12);
}

.runtime-led-segment.is-active.tone-warn {
  background: color-mix(in oklch, var(--warning) 88%, #ff7a00);
}

.runtime-led-segment.is-active.tone-error {
  background: color-mix(in oklch, var(--danger) 88%, #ff2d2d);
}

@media (max-width: 768px) {
  .runtime-led-segment {
    height: 8px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .runtime-led-segment {
    transition-duration: 0.01ms;
  }
}
</style>
