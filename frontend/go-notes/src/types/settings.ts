export interface Settings {
  baseDomain: string
  additionalDomains: string[]
  githubAppId: string
  githubAppClientId: string
  githubAppClientSecret: string
  githubAppInstallationId: string
  githubAppPrivateKey: string
  cloudflareToken: string
  cloudflareAccountId: string
  cloudflareZoneId: string
  cloudflaredTunnel: string
  cloudflaredConfigPath: string
}

export interface SettingsSources {
  baseDomain?: string
  githubAppId?: string
  githubAppClientId?: string
  githubAppClientSecret?: string
  githubAppInstallationId?: string
  githubAppPrivateKey?: string
  templatesDir?: string
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
  templatesDir?: string
}

export interface CloudflaredPreview {
  path: string
  contents: string
}
