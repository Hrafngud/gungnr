export interface Job {
  id: number
  type: string
  status: string
  startedAt?: string | null
  finishedAt?: string | null
  error?: string | null
  createdAt: string
}

export interface JobDetail extends Job {
  logLines: string[]
}

export interface HostDeployRequest<T = unknown> {
  jobType: string
  payload: T
}

export interface HostDeployResponse {
  job: Job
  token: string
  expiresAt?: string | null
  action: string
}
