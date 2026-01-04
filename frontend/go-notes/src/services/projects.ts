import { api } from '@/services/api'
import type { LocalProject, Project } from '@/types/projects'
import type { Job } from '@/types/jobs'

export const projectsApi = {
  list: () => api.get<{ projects: Project[] }>('/api/v1/projects'),
  listLocal: () => api.get<{ projects: LocalProject[] }>('/api/v1/projects/local'),
  createFromTemplate: (payload: {
    name: string
    subdomain?: string
    proxyPort?: number
    dbPort?: number
    template?: string
  }) => api.post<{ job: Job }>('/api/v1/projects/template', payload),
  deployExisting: (payload: { name: string; subdomain: string; port?: number }) =>
    api.post<{ job: Job }>('/api/v1/projects/existing', payload),
  forwardLocal: (payload: { name: string; subdomain: string; port?: number }) =>
    api.post<{ job: Job }>('/api/v1/projects/forward', payload),
  quickService: (payload: {
    subdomain: string
    port: number
    image?: string
    containerPort?: number
  }) =>
    api.post<{ job: Job }>('/api/v1/projects/quick', payload),
}
