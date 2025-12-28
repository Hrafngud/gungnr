import { ref } from 'vue'
import { defineStore } from 'pinia'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage } from '@/services/api'
import type { Job } from '@/types/jobs'

export const useJobsStore = defineStore('jobs', () => {
  const jobs = ref<Job[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)

  async function fetchJobs() {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await jobsApi.list()
      jobs.value = data.jobs
    } catch (err: any) {
      if (err?.response?.status === 401) {
        error.value = 'Sign in to view jobs.'
      } else {
        error.value = apiErrorMessage(err)
      }
      jobs.value = []
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  return {
    jobs,
    loading,
    error,
    initialized,
    fetchJobs,
  }
})
