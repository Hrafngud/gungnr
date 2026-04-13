import { ref } from 'vue'
import { defineStore } from 'pinia'
import { projectsApi } from '@/services/projects'
import { apiErrorMessage } from '@/services/api'
import type { Project, ProjectStatus } from '@/types/projects'

export const useProjectsStore = defineStore('projects', () => {
  const projects = ref<Project[]>([])
  const projectStatuses = ref<Record<string, string>>({})
  const loading = ref(false)
  const statusesLoading = ref(false)
  const error = ref<string | null>(null)
  const statusesError = ref<string | null>(null)
  const initialized = ref(false)

  async function fetchProjects() {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const { data } = await projectsApi.list()
      projects.value = data.projects
      const nextStatuses: Record<string, string> = {}
      for (const project of data.projects) {
        const name = project.name?.trim()
        if (!name) continue
        if (projectStatuses.value[name]) {
          nextStatuses[name] = projectStatuses.value[name]
        }
      }
      projectStatuses.value = nextStatuses
    } catch (err: any) {
      if (err?.response?.status === 401) {
        error.value = 'Sign in to view projects.'
      } else {
        error.value = apiErrorMessage(err)
      }
      projects.value = []
      projectStatuses.value = {}
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  async function fetchProjectStatuses() {
    if (statusesLoading.value) return
    statusesLoading.value = true
    statusesError.value = null
    try {
      const { data } = await projectsApi.listStatuses()
      const nextStatuses: Record<string, string> = {}
      for (const status of data.statuses ?? []) {
        const entry = status as ProjectStatus
        const name = entry.name?.trim()
        if (!name) continue
        nextStatuses[name] = entry.status
      }
      projectStatuses.value = nextStatuses
    } catch (err: any) {
      if (err?.response?.status === 401) {
        statusesError.value = 'Sign in to view live project statuses.'
      } else {
        statusesError.value = apiErrorMessage(err)
      }
    } finally {
      statusesLoading.value = false
    }
  }

  return {
    projects,
    projectStatuses,
    loading,
    statusesLoading,
    error,
    statusesError,
    initialized,
    fetchProjects,
    fetchProjectStatuses,
  }
})
