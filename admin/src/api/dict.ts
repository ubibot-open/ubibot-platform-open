import { api } from './client'

export interface DictEntry {
  id: number
  type: string
  key: string
  label: string
  sort: number
}

export function listDictEntries(type?: string) {
  return api.get<{ list: DictEntry[] }>(`/api/admin/dict${type ? `?type=${encodeURIComponent(type)}` : ''}`)
}

export function createDictEntry(input: { type: string; key: string; label: string; sort: number }) {
  return api.post<DictEntry>('/api/admin/dict', input)
}

export function updateDictEntry(id: number, input: { label: string; sort: number }) {
  return api.patch<{ message: string }>(`/api/admin/dict/${id}`, input)
}

export function deleteDictEntry(id: number) {
  return api.del<{ message: string }>(`/api/admin/dict/${id}`)
}
