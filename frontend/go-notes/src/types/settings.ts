export interface Settings {
  baseDomain: string
  githubToken: string
  cloudflareToken: string
  cloudflareAccountId: string
  cloudflareZoneId: string
  cloudflaredTunnel: string
  cloudflaredConfigPath: string
}

export interface SettingsSources {
  baseDomain?: string
  githubToken?: string
  cloudflareToken?: string
  cloudflareAccountId?: string
  cloudflareZoneId?: string
  cloudflaredTunnel?: string
  cloudflaredConfigPath?: string
}

export interface SettingsResponse {
  settings: Settings
  sources?: SettingsSources
  cloudflaredTunnelName?: string
}

export interface CloudflaredPreview {
  path: string
  contents: string
}
