import { api } from './client'

export interface Notification {
  id: number
  type: 'alert' | 'ota' | 'system'
  level: 'info' | 'warning' | 'critical'
  title: string
  content: string
  status: 'unread' | 'read'
  created_at: number
}

export function listNotifications(page = 1, pageSize = 20) {
  return api.get<{ list: Notification[]; total: number; unread: number }>(
    `/api/admin/notifications?page=${page}&page_size=${pageSize}`,
  )
}

export function markNotificationRead(id: number) {
  return api.post<{ message: string }>(`/api/admin/notifications/${id}/read`)
}

export function markAllNotificationsRead() {
  return api.post<{ message: string }>('/api/admin/notifications/read-all')
}
