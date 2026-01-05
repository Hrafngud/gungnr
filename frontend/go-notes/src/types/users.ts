export type UserRole = 'superuser' | 'admin' | 'user'

export type UserSummary = {
  id: number
  login: string
  role: UserRole
  lastLoginAt: string
}

export type UsersResponse = {
  users: UserSummary[]
}
