import { api } from './client'

export interface ApiKey {
  id: number
  name: string
  prefix: string
  revoked: boolean
  last_used_at: number | null
  created_at: number
}

export function listApiKeys() {
  return api.get<{ list: ApiKey[] }>('/api/admin/api-keys')
}

export function createApiKey(name: string) {
  return api.post<{ key: ApiKey; raw_key: string }>('/api/admin/api-keys', { name })
}

export function revokeApiKey(id: number) {
  return api.post<{ message: string }>(`/api/admin/api-keys/${id}/revoke`)
}
