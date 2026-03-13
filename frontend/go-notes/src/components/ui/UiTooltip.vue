<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import type { Component, ComponentPublicInstance, CSSProperties } from 'vue'

defineOptions({ inheritAttrs: false })

type TooltipPlacement = 'auto' | 'top' | 'bottom' | 'left' | 'right'
type ResolvedTooltipPlacement = Exclude<TooltipPlacement, 'auto'>

const props = withDefaults(defineProps<{
  as?: string | Component
  text: string
  placement?: TooltipPlacement
  position?: 'top' | 'bottom'
  offset?: number
  maxWidth?: number
}>(), {
  as: 'span',
  placement: 'auto',
  position: undefined,
  offset: 10,
  maxWidth: 360,
})

const rootRef = ref<HTMLElement | ComponentPublicInstance | null>(null)
const tooltipRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)
const resolvedPlacement = ref<ResolvedTooltipPlacement>('bottom')
const tooltipTop = ref(0)
const tooltipLeft = ref(0)
const arrowOffset = ref(0)
const tooltipId = `ui-tooltip-${Math.random().toString(36).slice(2, 10)}`
const openDelayMs = 90
const closeDelayMs = 70
const viewportPadding = 10

let openTimer: ReturnType<typeof setTimeout> | null = null
let closeTimer: ReturnType<typeof setTimeout> | null = null

const requestedPlacement = computed<TooltipPlacement>(() => (
  props.position ?? props.placement
))

const tooltipStyle = computed<CSSProperties>(() => ({
  top: `${tooltipTop.value}px`,
  left: `${tooltipLeft.value}px`,
  maxWidth: `${props.maxWidth}px`,
}))

const arrowStyle = computed<CSSProperties>(() => {
  if (resolvedPlacement.value === 'top' || resolvedPlacement.value === 'bottom') {
    return { left: `${arrowOffset.value}px` }
  }

  return { top: `${arrowOffset.value}px` }
})

function clamp(value: number, min: number, max: number) {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

function resolveTriggerElement(target: HTMLElement | ComponentPublicInstance | null) {
  if (!target) return null
  if (target instanceof HTMLElement) return target
  if (target.$el instanceof HTMLElement) return target.$el
  return null
}

function resolveCandidates(preferred: TooltipPlacement): ResolvedTooltipPlacement[] {
  const all: ResolvedTooltipPlacement[] = ['bottom', 'top', 'right', 'left']
  if (preferred === 'auto') return all

  return [
    preferred,
    ...all.filter((placement) => placement !== preferred),
  ]
}

function cleanupTimers() {
  if (openTimer) {
    clearTimeout(openTimer)
    openTimer = null
  }

  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
}

function updatePosition() {
  const root = resolveTriggerElement(rootRef.value)
  const tooltip = tooltipRef.value
  if (!root || !tooltip) return

  const triggerRect = root.getBoundingClientRect()
  const bubbleRect = tooltip.getBoundingClientRect()
  const triggerCenterX = triggerRect.left + (triggerRect.width / 2)
  const triggerCenterY = triggerRect.top + (triggerRect.height / 2)
  const candidates = resolveCandidates(requestedPlacement.value)

  let bestCandidate: {
    placement: ResolvedTooltipPlacement
    left: number
    top: number
    overflow: number
  } | null = null

  for (const placement of candidates) {
    let rawLeft = 0
    let rawTop = 0

    if (placement === 'top') {
      rawLeft = triggerCenterX - (bubbleRect.width / 2)
      rawTop = triggerRect.top - bubbleRect.height - props.offset
    } else if (placement === 'bottom') {
      rawLeft = triggerCenterX - (bubbleRect.width / 2)
      rawTop = triggerRect.bottom + props.offset
    } else if (placement === 'left') {
      rawLeft = triggerRect.left - bubbleRect.width - props.offset
      rawTop = triggerCenterY - (bubbleRect.height / 2)
    } else {
      rawLeft = triggerRect.right + props.offset
      rawTop = triggerCenterY - (bubbleRect.height / 2)
    }

    const maxLeft = window.innerWidth - bubbleRect.width - viewportPadding
    const maxTop = window.innerHeight - bubbleRect.height - viewportPadding
    const left = clamp(rawLeft, viewportPadding, maxLeft)
    const top = clamp(rawTop, viewportPadding, maxTop)

    const overflowLeft = Math.max(0, viewportPadding - rawLeft)
    const overflowRight = Math.max(0, (rawLeft + bubbleRect.width) - (window.innerWidth - viewportPadding))
    const overflowTop = Math.max(0, viewportPadding - rawTop)
    const overflowBottom = Math.max(0, (rawTop + bubbleRect.height) - (window.innerHeight - viewportPadding))
    const overflow = overflowLeft + overflowRight + overflowTop + overflowBottom

    if (!bestCandidate || overflow < bestCandidate.overflow) {
      bestCandidate = { placement, left, top, overflow }
    }
  }

  if (!bestCandidate) return

  resolvedPlacement.value = bestCandidate.placement
  tooltipLeft.value = bestCandidate.left
  tooltipTop.value = bestCandidate.top

  const arrowEdgePadding = 16
  if (bestCandidate.placement === 'top' || bestCandidate.placement === 'bottom') {
    arrowOffset.value = clamp(
      triggerCenterX - bestCandidate.left,
      arrowEdgePadding,
      bubbleRect.width - arrowEdgePadding,
    )
    return
  }

  arrowOffset.value = clamp(
    triggerCenterY - bestCandidate.top,
    arrowEdgePadding,
    bubbleRect.height - arrowEdgePadding,
  )
}

function onViewportChange() {
  if (!isVisible.value) return
  updatePosition()
}

function attachViewportListeners() {
  window.addEventListener('scroll', onViewportChange, true)
  window.addEventListener('resize', onViewportChange)
}

function detachViewportListeners() {
  window.removeEventListener('scroll', onViewportChange, true)
  window.removeEventListener('resize', onViewportChange)
}

function showTooltip() {
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
  if (isVisible.value) {
    updatePosition()
    return
  }
  if (openTimer) return

  openTimer = setTimeout(async () => {
    openTimer = null
    isVisible.value = true
    await nextTick()
    updatePosition()
    attachViewportListeners()
  }, openDelayMs)
}

function hideTooltip() {
  if (openTimer) {
    clearTimeout(openTimer)
    openTimer = null
  }
  if (!isVisible.value || closeTimer) return

  closeTimer = setTimeout(() => {
    closeTimer = null
    isVisible.value = false
    detachViewportListeners()
  }, closeDelayMs)
}

function handleFocusOut(event: FocusEvent) {
  const nextTarget = event.relatedTarget
  const root = resolveTriggerElement(rootRef.value)
  if (nextTarget instanceof Node && root?.contains(nextTarget)) return
  hideTooltip()
}

onBeforeUnmount(() => {
  cleanupTimers()
  detachViewportListeners()
})
</script>

<template>
  <component
    :is="as"
    ref="rootRef"
    class="ui-tooltip"
    :aria-describedby="tooltipId"
    v-bind="$attrs"
    @mouseenter="showTooltip"
    @mouseleave="hideTooltip"
    @focusin="showTooltip"
    @focusout="handleFocusOut"
    @keydown.esc.stop="hideTooltip"
  >
    <slot />
  </component>

  <Teleport to="body">
    <Transition name="ui-tooltip-fade">
      <span
        v-if="isVisible"
        :id="tooltipId"
        ref="tooltipRef"
        role="tooltip"
        class="ui-tooltip__bubble"
        :class="`ui-tooltip__bubble--${resolvedPlacement}`"
        :style="tooltipStyle"
      >
        <span
          class="ui-tooltip__arrow"
          :style="arrowStyle"
        />
        <span class="ui-tooltip__text">
          {{ text }}
        </span>
      </span>
    </Transition>
  </Teleport>
</template>

<style scoped>
.ui-tooltip {
  display: inline-flex;
  min-width: 0;
}

.ui-tooltip__bubble {
  position: fixed;
  z-index: 80;
  min-width: 10.5rem;
  padding: 0.58rem 0.78rem;
  border-radius: 0.72rem;
  border: 1px solid color-mix(in srgb, var(--border) 86%, var(--accent) 14%);
  background: color-mix(in srgb, var(--surface-2) 94%, var(--accent) 6%);
  color: color-mix(in srgb, var(--text) 84%, var(--muted) 16%);
  box-shadow:
    0 12px 32px color-mix(in srgb, #000 34%, transparent),
    0 0 0 1px color-mix(in srgb, var(--surface) 32%, transparent) inset;
  pointer-events: none;
  white-space: normal;
}

.ui-tooltip__text {
  display: block;
  font-size: 0.84rem;
  line-height: 1.35;
  letter-spacing: 0.01em;
  text-wrap: pretty;
}

.ui-tooltip__arrow {
  position: absolute;
  width: 0.62rem;
  height: 0.62rem;
  border-radius: 0.1rem;
  border: 1px solid color-mix(in srgb, var(--border) 86%, var(--accent) 14%);
  border-top: none;
  border-left: none;
  background: color-mix(in srgb, var(--surface-2) 94%, var(--accent) 6%);
  transform: translate(-50%, 50%) rotate(45deg);
}

.ui-tooltip__bubble--top .ui-tooltip__arrow {
  bottom: -0.41rem;
}

.ui-tooltip__bubble--bottom .ui-tooltip__arrow {
  top: -0.41rem;
  transform: translate(-50%, -50%) rotate(225deg);
}

.ui-tooltip__bubble--left .ui-tooltip__arrow {
  right: -0.41rem;
  transform: translate(50%, -50%) rotate(-45deg);
}

.ui-tooltip__bubble--right .ui-tooltip__arrow {
  left: -0.41rem;
  transform: translate(-50%, -50%) rotate(135deg);
}

.ui-tooltip-fade-enter-active,
.ui-tooltip-fade-leave-active {
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}

.ui-tooltip-fade-enter-from,
.ui-tooltip-fade-leave-to {
  opacity: 0;
  transform: translateY(3px);
}

@media (prefers-reduced-motion: reduce) {
  .ui-tooltip-fade-enter-active,
  .ui-tooltip-fade-leave-active {
    transition: opacity 0.01s linear;
  }
}
</style>
