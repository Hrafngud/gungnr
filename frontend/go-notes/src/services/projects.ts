import { api, getApiBaseUrl } from '@/services/api'
import type {
  LocalProject,
  Project,
  ProjectStatus,
  ProjectArchiveOptions,
  ProjectArchivePlan,
  ProjectDetail,
  ProjectEnvRead,
  ProjectEnvWrite,
} from '@/types/projects'
import type { Job, JobListResponse } from '@/types/jobs'

const restartProjectTimeoutMs = 10 * 60 * 1000

export const projectsApi = {
  list: () => api.get<{ projects: Project[] }>('/api/v1/projects'),
  listStatuses: () => api.get<{ statuses: ProjectStatus[] }>('/api/v1/projects/status'),
  getDetail: (name: string) => api.get<ProjectDetail>(`/api/v1/projects/${encodeURIComponent(name)}`),
  listJobs: (name: string, params?: { page?: number; limit?: number }) =>
    api.get<JobListResponse>(`/api/v1/projects/${encodeURIComponent(name)}/jobs`, { params }),
  getArchivePlan: (name: string) =>
    api.get<{ plan: ProjectArchivePlan }>(`/api/v1/projects/${encodeURIComponent(name)}/archive/plan`),
  archiveProject: (name: string, payload: Partial<ProjectArchiveOptions>) =>
    api.post<{ job: Job; plan: ProjectArchivePlan }>(`/api/v1/projects/${encodeURIComponent(name)}/archive`, payload),
  listLocal: () => api.get<{ projects: LocalProject[] }>('/api/v1/projects/local'),
  restartStack: (name: string) =>
    api.post<{ job: Job }>(
      `/api/v1/projects/${encodeURIComponent(name)}/stack/restart`,
      {},
      { timeout: restartProjectTimeoutMs },
    ),
  stopContainer: (name: string, container: string) =>
    api.post<{ status: string }>(`/api/v1/projects/${encodeURIComponent(name)}/containers/stop`, { container }),
  restartContainer: (name: string, container: string) =>
    api.post<{ status: string }>(`/api/v1/projects/${encodeURIComponent(name)}/containers/restart`, { container }),
  removeContainer: (name: string, container: string, removeVolumes = false) =>
    api.post<{ status: string }>(`/api/v1/projects/${encodeURIComponent(name)}/containers/remove`, { container, removeVolumes }),
  projectLogsUrl: (
    name: string,
    container: string,
    options?: { tail?: number; follow?: boolean; timestamps?: boolean },
  ) => {
    const params = new URLSearchParams({
      container,
      tail: String(options?.tail ?? 200),
      follow: String(options?.follow ?? true),
      timestamps: String(options?.timestamps ?? true),
    })
    const base = getApiBaseUrl()
    return `${base}/api/v1/projects/${encodeURIComponent(name)}/logs?${params.toString()}`
  },
  loadEnv: (name: string) =>
    api.get<{ env: ProjectEnvRead }>(`/api/v1/projects/${encodeURIComponent(name)}/env`),
  saveEnv: (name: string, content: string, createBackup = true) =>
    api.put<{ env: ProjectEnvWrite }>(`/api/v1/projects/${encodeURIComponent(name)}/env`, {
      content,
      createBackup,
    }),
  createFromTemplate: (payload: {
    name: string
    subdomain?: string
    domain?: string
    proxyPort?: number
    dbPort?: number
    template?: string
  }) => api.post<{ job: Job }>('/api/v1/projects/template', payload),
  deployExisting: (payload: { name: string; subdomain: string; domain?: string; port?: number }) =>
    api.post<{ job: Job }>('/api/v1/projects/existing', payload),
  forwardLocal: (payload: { name: string; subdomain: string; domain?: string; port?: number }) =>
    api.post<{ job: Job }>('/api/v1/projects/forward', payload),
  quickService: (payload: {
    subdomain: string
    domain?: string
    port: number
    image?: string
    containerPort?: number
  }) =>
    api.post<{ job: Job; hostPort: number }>('/api/v1/projects/quick', payload),
}
