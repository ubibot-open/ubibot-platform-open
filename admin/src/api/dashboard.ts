import { api } from './client'

export interface DashboardSummary {
  device_total: number
  device_online: number
  open_alerts: number
  today_records: number
}

export function getDashboardSummary() {
  return api.get<DashboardSummary>('/api/admin/dashboard/summary')
}

export interface DailyCount {
  day: string
  count: number
}

export function getDashboardTrends() {
  return api.get<{ days: DailyCount[] }>('/api/admin/dashboard/trends')
}
