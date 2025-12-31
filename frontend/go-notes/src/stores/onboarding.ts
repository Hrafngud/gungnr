import { defineStore } from 'pinia'
import { ref } from 'vue'
import { onboardingApi } from '@/services/onboarding'
import { apiErrorMessage } from '@/services/api'
import type { OnboardingState, OnboardingUpdate } from '@/types/onboarding'

const defaultState: OnboardingState = {
  home: false,
  hostSettings: false,
  networking: false,
  github: false,
}

export const useOnboardingStore = defineStore('onboarding', () => {
  const state = ref<OnboardingState>({ ...defaultState })
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)

  async function fetchState() {
    if (loading.value || initialized.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await onboardingApi.get()
      state.value = { ...defaultState, ...(data.state ?? {}) }
    } catch (err) {
      error.value = apiErrorMessage(err)
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  async function updateState(update: OnboardingUpdate) {
    if (loading.value) return
    loading.value = true
    error.value = null
    const previousState = { ...state.value }
    const nextState = { ...state.value, ...update }
    state.value = nextState
    try {
      const { data } = await onboardingApi.update(update)
      state.value = { ...nextState, ...(data.state ?? {}) }
    } catch (err) {
      error.value = apiErrorMessage(err)
      state.value = previousState
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  return {
    state,
    loading,
    error,
    initialized,
    fetchState,
    updateState,
  }
})
