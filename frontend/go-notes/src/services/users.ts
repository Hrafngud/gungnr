import { api } from '@/services/api'
import type { UserRole, UserSummary, UsersResponse } from '@/types/users'

export const usersApi = {
  list: () => api.get<UsersResponse>('/api/v1/users'),
  create: (login: string) => api.post<UserSummary>('/api/v1/users', { login }),
  updateRole: (id: number, role: UserRole) =>
    api.patch<UserSummary>(`/api/v1/users/${id}/role`, { role }),
  remove: (id: number) => api.delete(`/api/v1/users/${id}`),
}
