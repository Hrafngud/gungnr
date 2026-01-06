import { ref } from 'vue'
import { defineStore } from 'pinia'
import { jobsApi } from '@/services/jobs'
import { apiErrorMessage } from '@/services/api'
import type { Job } from '@/types/jobs'

const DEFAULT_PAGE_SIZE = 25

export const useJobsStore = defineStore('jobs', () => {
  const jobs = ref<Job[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)
  const page = ref(1)
  const pageSize = ref(DEFAULT_PAGE_SIZE)
  const total = ref(0)
  const totalPages = ref(0)

  async function fetchJobs(options: { page?: number; pageSize?: number } = {}) {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const requestedPage = options.page ?? page.value
      const requestedSize = options.pageSize ?? pageSize.value
      const { data } = await jobsApi.list({ page: requestedPage, limit: requestedSize })
      jobs.value = data.jobs
      page.value = data.page
      pageSize.value = data.pageSize
      total.value = data.total
      totalPages.value = data.totalPages
    } catch (err: any) {
      if (err?.response?.status === 401) {
        error.value = 'Sign in to view jobs.'
      } else {
        error.value = apiErrorMessage(err)
      }
      jobs.value = []
      total.value = 0
      totalPages.value = 0
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
    page,
    pageSize,
    total,
    totalPages,
    fetchJobs,
  }
})
