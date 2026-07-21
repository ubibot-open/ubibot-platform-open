import { api, getToken } from './client'

export interface FileAsset {
  id: number
  category: string
  filename: string
  size: number
  sha256: string
  created_at: number
}

export function listFileAssets() {
  return api.get<{ list: FileAsset[] }>('/api/admin/files')
}

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export async function uploadFileAsset(input: { category: string; file: File }) {
  const form = new FormData()
  form.append('category', input.category)
  form.append('file', input.file)

  const token = getToken()
  const res = await fetch(`${BASE_URL}/api/admin/files`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.message ?? '文件上传失败')
  return data as FileAsset
}

export function deleteFileAsset(id: number) {
  return api.del<{ message: string }>(`/api/admin/files/${id}`)
}
