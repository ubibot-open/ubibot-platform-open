import { api } from './client'

export interface Device {
  id: number
  pid: string
  sn: string
  name: string
  status: number
  activated: boolean
  online: boolean
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

// getDeviceRecords is the "历史数据查询" backing call — start/end are Unix
// seconds, omit either to leave that bound open.
export function getDeviceRecords(id: number, opts: { start?: number; end?: number; page?: number; pageSize?: number }) {
  const params = new URLSearchParams()
  if (opts.start) params.set('start', String(opts.start))
  if (opts.end) params.set('end', String(opts.end))
  params.set('page', String(opts.page ?? 1))
  params.set('page_size', String(opts.pageSize ?? 50))
  return api.get<{ list: DeviceRecord[]; total: number }>(`/api/admin/devices/${id}/records?${params}`)
}

// --- probes (protocol §7.2 set_probe) -------------------------------------

export interface Probe {
  pid: string
  key: string
  iface: string
  proto: string
  params: Record<string, unknown> | null
  status: 'pending' | 'applied' | 'failed' | 'removing'
  last_command_id?: string
  last_error?: string
}

export function listProbes(deviceId: number) {
  return api.get<{ list: Probe[] }>(`/api/admin/devices/${deviceId}/probes`)
}

export function upsertProbe(
  deviceId: number,
  input: { pid: string; key: string; iface: string; proto: string; params?: Record<string, unknown> },
) {
  return api.post<{ probe: Probe; command: DeviceCommand }>(`/api/admin/devices/${deviceId}/probes`, input)
}

export function removeProbe(deviceId: number, pid: string) {
  return api.del<{ command: DeviceCommand }>(`/api/admin/devices/${deviceId}/probes/${encodeURIComponent(pid)}`)
}
