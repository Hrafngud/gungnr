export interface Settings {
  baseDomain: string
  githubToken: string
  cloudflareToken: string
  cloudflareAccountId: string
  cloudflareZoneId: string
  cloudflaredConfigPath: string
}

export interface CloudflaredPreview {
  path: string
  contents: string
}
