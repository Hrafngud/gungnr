export type GitHubTemplateCatalog = {
  configured: boolean
  owner: string
  repo: string
  targetOwner: string
  private: boolean
}

export type GitHubAllowlist = {
  mode: 'users' | 'org' | 'none'
  users: string[]
  org: string
}

export type GitHubCatalog = {
  tokenConfigured: boolean
  template: GitHubTemplateCatalog
  allowlist: GitHubAllowlist
}
