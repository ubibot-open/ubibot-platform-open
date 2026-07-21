import { api } from './client'

export interface SystemParam {
  key: string
  value: string
  description: string
}

export function listSystemParams() {
  return api.get<{ list: SystemParam[] }>('/api/admin/params')
}

export function setSystemParam(key: string, value: string, description?: string) {
  return api.patch<SystemParam>(`/api/admin/params/${encodeURIComponent(key)}`, { value, description })
}
