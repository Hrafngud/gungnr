export interface AuditLogEntry {
  id: number
  userId: number
  userLogin: string
  action: string
  target: string
  metadata: string
  createdAt: string
}
