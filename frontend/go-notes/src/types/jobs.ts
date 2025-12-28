export interface Job {
  id: number
  type: string
  status: string
  startedAt?: string | null
  finishedAt?: string | null
  error?: string | null
  createdAt: string
}
