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
  const isAdmin = computed(() => user.value?.role === 'admin' || user.value?.role === 'superuser')
  const isSuperUser = computed(() => user.value?.role === 'superuser')

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
    isAdmin,
    isSuperUser,
    fetchUser,
    logout,
    loginUrl,
  }
})
