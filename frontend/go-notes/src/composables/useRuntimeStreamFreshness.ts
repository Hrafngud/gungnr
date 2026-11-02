import { computed, onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

type StreamState = 'idle' | 'connecting' | 'live' | 'error'

export function useRuntimeStreamFreshness(
  collectedAt: () => string | undefined,
  state: Ref<StreamState>,
  error: Ref<string | null>,
) {
  const clock = ref(Date.now())
  const ageSeconds = computed(() => {
    const at = Date.parse(collectedAt() ?? '')
    return Number.isFinite(at) ? Math.max(0, Math.floor((clock.value - at) / 1000)) : null
  })
  const ageLabel = computed(() => ageSeconds.value === null ? 'Waiting for a sample' : `Updated ${ageSeconds.value}s ago`)
  let timer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    timer = setInterval(() => {
      clock.value = Date.now()
      if (state.value === 'live' && (ageSeconds.value === null || ageSeconds.value > 15)) {
        state.value = 'error'
        error.value = 'Updates are delayed. Showing the last collected values while reconnecting.'
      }
    }, 1000)
  })
  onBeforeUnmount(() => { if (timer) clearInterval(timer) })
  return { ageLabel }
}
