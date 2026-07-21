import { api } from './client'

export interface LoginResponse {
  token: string
  expires_in: number
  username: string
}

export function login(username: string, password: string) {
  return api.post<LoginResponse>('/api/admin/login', { username, password })
}

export function me() {
  return api.get<{ username: string }>('/api/admin/me')
}
