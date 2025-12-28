export interface Settings {
  baseDomain: string
  githubToken: string
  cloudflareToken: string
  cloudflaredConfigPath: string
}

export interface CloudflaredPreview {
  path: string
  contents: string
}
