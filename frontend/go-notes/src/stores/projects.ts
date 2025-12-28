import { ref } from 'vue'
import { defineStore } from 'pinia'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import type { Project } from '@/types/projects'

export const useProjectsStore = defineStore('projects', () => {
  const projects = ref<Project[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)

  async function fetchProjects() {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await projectsApi.list()
      projects.value = data.projects
    } catch (err: any) {
      if (err?.response?.status === 401) {
        error.value = 'Sign in to view projects.'
      } else {
        error.value = apiErrorMessage(err)
      }
      projects.value = []
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  return {
    projects,
    loading,
    error,
    initialized,
    fetchProjects,
  }
})
