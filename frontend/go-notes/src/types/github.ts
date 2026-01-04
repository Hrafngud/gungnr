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

export type GitHubAppStatus = {
  configured: boolean
  appIdConfigured: boolean
  installationIdConfigured: boolean
  privateKeyConfigured: boolean
}

export type GitHubRepoAccessDiagnostics = {
  checked: boolean
  available: boolean
  error?: string
  requestId?: string
}

export type GitHubTemplateAccessDiagnostics = {
  installationOwner?: string
  installationOwnerType?: string
  installationError?: string
  repoAccess: GitHubRepoAccessDiagnostics
}

export type GitHubCatalog = {
  tokenConfigured: boolean
  template: GitHubTemplateCatalog
  templates?: GitHubTemplateCatalog[]
  allowlist: GitHubAllowlist
  app: GitHubAppStatus
  templateAccess?: GitHubTemplateAccessDiagnostics
}
