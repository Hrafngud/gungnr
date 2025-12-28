import { api } from '@/services/api'
import type { CloudflaredPreview, Settings } from '@/types/settings'

export const settingsApi = {
  get: () => api.get<{ settings: Settings }>('/api/v1/settings'),
  update: (payload: Settings) =>
    api.put<{ settings: Settings }>('/api/v1/settings', payload),
  preview: () => api.get<{ preview: CloudflaredPreview }>('/api/v1/settings/cloudflared/preview'),
}
