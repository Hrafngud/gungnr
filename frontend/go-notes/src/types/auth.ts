export type AuthUser = {
  id: number
  login: string
  avatarUrl: string
  role: 'superuser' | 'admin' | 'user'
  expiresAt: string
}
