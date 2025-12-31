import { api } from '@/services/api'
import type { CloudflarePreflight } from '@/types/cloudflare'

export const cloudflareApi = {
  preflight: () => api.get<CloudflarePreflight>('/api/v1/cloudflare/preflight'),
}
