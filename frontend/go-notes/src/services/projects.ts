import { api } from '@/services/api'
import type { Project } from '@/types/projects'

export const projectsApi = {
  list: () => api.get<{ projects: Project[] }>('/api/v1/projects'),
}
