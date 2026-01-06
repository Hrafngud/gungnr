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

export interface JobListResponse {
  jobs: Job[]
  page: number
  pageSize: number
  total: number
  totalPages: number
}
