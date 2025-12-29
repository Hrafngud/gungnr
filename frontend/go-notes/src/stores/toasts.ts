import { ref } from 'vue'
import { defineStore } from 'pinia'

export type ToastTone = 'neutral' | 'ok' | 'warn' | 'error'

export type ToastItem = {
  id: string
  tone: ToastTone
  title: string
  message?: string
  timeout: number
}

type ToastInput = {
  tone?: ToastTone
  title?: string
  message?: string
  timeout?: number
}

const defaultTitles: Record<ToastTone, string> = {
  neutral: 'Notice',
  ok: 'Success',
  warn: 'Heads up',
  error: 'Action failed',
}

export const useToastStore = defineStore('toasts', () => {
  const toasts = ref<ToastItem[]>([])
  let counter = 0

  const remove = (id: string) => {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }

  const add = (input: ToastInput) => {
    const tone = input.tone ?? 'neutral'
    const title = input.title ?? defaultTitles[tone]
    const timeout = input.timeout ?? 4500
    const id = `${Date.now()}-${counter++}`
    const toast: ToastItem = {
      id,
      tone,
      title,
      message: input.message,
      timeout,
    }

    toasts.value = [...toasts.value, toast]

    if (timeout > 0) {
      window.setTimeout(() => remove(id), timeout)
    }

    return id
  }

  const success = (message: string, title?: string) =>
    add({ tone: 'ok', title, message })

  const warn = (message: string, title?: string) =>
    add({ tone: 'warn', title, message })

  const error = (message: string, title?: string) =>
    add({ tone: 'error', title, message })

  const neutral = (message: string, title?: string) =>
    add({ tone: 'neutral', title, message })

  const clear = () => {
    toasts.value = []
  }

  return {
    toasts,
    add,
    remove,
    clear,
    success,
    warn,
    error,
    neutral,
  }
})
