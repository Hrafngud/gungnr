import { api } from '@/services/api'
import type { AuditLogEntry } from '@/types/audit'

export const auditApi = {
  list: (limit = 100) =>
    api.get<{ logs: AuditLogEntry[] }>('/api/v1/audit-logs', {
      params: { limit },
    }),
}
