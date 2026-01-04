import { defineStore } from 'pinia'
import { ref } from 'vue'

export const usePageLoadingStore = defineStore('pageLoading', () => {
  const loading = ref(false)
  const message = ref('Loading panel data...')

  const start = (nextMessage?: string) => {
    if (nextMessage) {
      message.value = nextMessage
    }
    loading.value = true
  }

  const stop = () => {
    loading.value = false
  }

  return {
    loading,
    message,
    start,
    stop,
  }
})
