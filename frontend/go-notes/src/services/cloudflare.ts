import { api } from '@/services/api'
import type { CloudflarePreflight, CloudflareZonesResponse } from '@/types/cloudflare'

export const cloudflareApi = {
  preflight: () => api.get<CloudflarePreflight>('/api/v1/cloudflare/preflight'),
  zones: () => api.get<CloudflareZonesResponse>('/api/v1/cloudflare/zones'),
}
