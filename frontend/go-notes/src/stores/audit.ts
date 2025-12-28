import { ref } from 'vue'
import { defineStore } from 'pinia'
import { auditApi } from '@/services/audit'
import { apiErrorMessage } from '@/services/api'
import type { AuditLogEntry } from '@/types/audit'

export const useAuditStore = defineStore('audit', () => {
  const logs = ref<AuditLogEntry[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)

  async function fetchLogs() {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await auditApi.list()
      logs.value = data.logs
    } catch (err: any) {
      if (err?.response?.status === 401) {
        error.value = 'Sign in to view activity.'
      } else {
        error.value = apiErrorMessage(err)
      }
      logs.value = []
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  return {
    logs,
    loading,
    error,
    initialized,
    fetchLogs,
  }
})
