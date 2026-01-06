export type GitHubTemplateCatalog = {
  configured: boolean
  owner: string
  repo: string
  targetOwner: string
  private: boolean
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
  app: GitHubAppStatus
  templateAccess?: GitHubTemplateAccessDiagnostics
}
