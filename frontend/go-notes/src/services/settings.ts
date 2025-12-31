import { api } from '@/services/api'
import type { CloudflaredPreview, Settings, SettingsResponse } from '@/types/settings'

export const settingsApi = {
  get: () => api.get<SettingsResponse>('/api/v1/settings'),
  update: (payload: Settings) => api.put<SettingsResponse>('/api/v1/settings', payload),
  preview: () => api.get<{ preview: CloudflaredPreview }>('/api/v1/settings/cloudflared/preview'),
}
