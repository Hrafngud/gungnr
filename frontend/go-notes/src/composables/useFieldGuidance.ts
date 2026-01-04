import { ref } from 'vue'
import type { GuidanceContent } from '@/components/ui/UiFieldGuidance.vue'

export const useFieldGuidance = () => {
  const open = ref(false)
  const content = ref<GuidanceContent | null>(null)

  const show = (next: GuidanceContent) => {
    content.value = next
    open.value = true
  }

  const clear = () => {
    open.value = false
  }

  return {
    open,
    content,
    show,
    clear,
  }
}
