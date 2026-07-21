import { api } from './client'

export interface AlertRule {
  id: number
  device_id: number
  field: string
  op: '>' | '>=' | '<' | '<=' | '=='
  threshold: number
  enabled: boolean
}

export function listAlertRules(deviceId: number) {
  return api.get<{ list: AlertRule[] }>(`/api/admin/devices/${deviceId}/alert-rules`)
}

export function createAlertRule(deviceId: number, input: { field: string; op: AlertRule['op']; threshold: number }) {
  return api.post<AlertRule>(`/api/admin/devices/${deviceId}/alert-rules`, input)
}

export function deleteAlertRule(id: number) {
  return api.del<{ message: string }>(`/api/admin/alert-rules/${id}`)
}

export interface AlertEvent {
  id: number
  device_id: number
  device_name: string
  rule_id: number
  type: 'threshold' | 'offline'
  message: string
  status: 'open' | 'resolved'
  triggered_at: number
  resolved_at: number | null
}

export function listAlertEvents(opts: { deviceId?: number; status?: string; page?: number; pageSize?: number }) {
  const params = new URLSearchParams()
  if (opts.deviceId) params.set('device_id', String(opts.deviceId))
  if (opts.status) params.set('status', opts.status)
  params.set('page', String(opts.page ?? 1))
  params.set('page_size', String(opts.pageSize ?? 20))
  return api.get<{ list: AlertEvent[]; total: number }>(`/api/admin/alert-events?${params}`)
}

export function resolveAlertEvent(id: number) {
  return api.post<{ message: string }>(`/api/admin/alert-events/${id}/resolve`)
}
