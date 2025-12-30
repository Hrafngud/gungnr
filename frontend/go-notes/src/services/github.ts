import { api } from '@/services/api'
import type { GitHubCatalog } from '@/types/github'

export const githubApi = {
  catalog: () => api.get<{ catalog: GitHubCatalog }>('/api/v1/github/catalog'),
}
