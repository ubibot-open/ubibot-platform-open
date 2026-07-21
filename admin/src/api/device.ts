import { api } from './client'

export interface Device {
  id: number
  pid: string
  sn: string
  name: string
  status: number
  ci: number
  ui: number
  fe: string[] | null
  last_seen_at: number | null
  created_at: number
  secret?: string
}

export interface DeviceRecord {
  ts: number
  d: Record<string, unknown>
}

export interface DeviceCommand {
  id: string
  type: string
  args?: Record<string, unknown>
  status: 'pending' | 'acked' | 'nacked'
  nak_message?: string
  created_at: number
}

export const DeviceStatus = {
  Enabled: 1,
  Disabled: 2,
} as const

export function listDevices(page = 1, pageSize = 20) {
  return api.get<{ list: Device[]; total: number }>(`/api/admin/devices?page=${page}&page_size=${pageSize}`)
}

export function createDevice(input: { pid: string; sn: string; secret?: string; name?: string }) {
  return api.post<Device>('/api/admin/devices', input)
}

export function getDevice(id: number) {
  return api.get<{ device: Device; records: DeviceRecord[]; commands: DeviceCommand[] }>(`/api/admin/devices/${id}`)
}

export function updateDeviceConfig(id: number, input: { ci: number; ui: number; fe?: string[] }) {
  return api.patch<{ message: string }>(`/api/admin/devices/${id}/config`, input)
}

export function setDeviceStatus(id: number, status: number) {
  return api.post<{ message: string }>(`/api/admin/devices/${id}/status`, { status })
}

export function dispatchCommand(id: number, input: { type: string; args?: Record<string, unknown> }) {
  return api.post<DeviceCommand>(`/api/admin/devices/${id}/commands`, input)
}

export function listCommands(id: number, page = 1, pageSize = 20) {
  return api.get<{ list: DeviceCommand[]; total: number }>(
    `/api/admin/devices/${id}/commands?page=${page}&page_size=${pageSize}`,
  )
}
