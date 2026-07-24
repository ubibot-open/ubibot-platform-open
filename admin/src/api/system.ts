import { api } from './client'

export interface SystemMetrics {
  go_version: string
  goroutines: number
  heap_alloc_bytes: number
  uptime_seconds: number
  db_size_bytes: number
  device_total: number
  open_alerts: number
  unread_notifications: number
}

export function getSystemMetrics() {
  return api.get<SystemMetrics>('/api/admin/system/metrics')
}
