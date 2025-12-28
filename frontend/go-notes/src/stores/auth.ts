import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi, loginUrl } from '@/services/auth'
import type { AuthUser } from '@/types/auth'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)

  const isAuthenticated = computed(() => Boolean(user.value))

  async function fetchUser() {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await authApi.me()
      user.value = data
    } catch (err: any) {
      if (err?.response?.status !== 401) {
        error.value = err?.message ?? 'Unable to load user'
      }
      user.value = null
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      user.value = null
    }
  }

  return {
    user,
    loading,
    error,
    initialized,
    isAuthenticated,
    fetchUser,
    logout,
    loginUrl,
  }
})
