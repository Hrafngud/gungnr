import { api, getApiBaseUrl } from '@/services/api'
import type { AuthUser } from '@/types/auth'

export const authApi = {
  me: () => api.get<AuthUser>('/auth/me'),
  logout: () => api.post('/auth/logout'),
}

export function loginUrl(): string {
  return `${getApiBaseUrl()}/auth/login`
}
