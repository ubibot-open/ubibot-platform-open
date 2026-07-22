import { api } from './client'

// IconAsset is a custom SVG uploaded to override the built-in icon for a
// sensor field key (see components/icons/SensorIcons.tsx) -- backs the
// "图标库" (icon library) management page and the 数据仓库 page's
// per-field icon lookup (see hooks/useFieldIcons.tsx).
export interface IconAsset {
  key: string
  name: string
  svg: string
  created_at: number
}

export function listIcons() {
  return api.get<{ list: IconAsset[] }>('/api/admin/icons')
}

// uploadIcon creates the icon for key if it doesn't exist yet, or replaces
// it if it does -- there's no separate update endpoint, re-uploading is
// the update.
export function uploadIcon(input: { key: string; name: string; svg: string }) {
  return api.post<IconAsset>('/api/admin/icons', input)
}

// deleteIcon reverts key back to whatever built-in default icon it has
// (or the generic fallback, if none).
export function deleteIcon(key: string) {
  return api.del<{ message: string }>(`/api/admin/icons/${encodeURIComponent(key)}`)
}
