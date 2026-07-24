import { api } from './client'

export interface Device {
  id: number
  pid: string
  sn: string
  name: string
  status: number
  online: boolean
  last_seen_at: number | null
  created_at: number
}

export interface DeviceRecord {
  ts: number
  d: Record<string, unknown>
}

export const DeviceStatus = {
  Enabled: 1,
  Disabled: 2,
} as const

export function listDevices(page = 1, pageSize = 20) {
  return api.get<{ list: Device[]; total: number }>(`/api/admin/devices?page=${page}&page_size=${pageSize}`)
}

// A Device plus its single most recent telemetry record (null if it has
// never reported) -- backs the "数据仓库" (data warehouse) page.
export interface DataWarehouseItem extends Device {
  last_record: DeviceRecord | null
}

export function listDataWarehouse(page = 1, pageSize = 20) {
  return api.get<{ list: DataWarehouseItem[]; total: number }>(
    `/api/admin/devices/data-warehouse?page=${page}&page_size=${pageSize}`,
  )
}

export function getDevice(id: number) {
  return api.get<{ device: Device; records: DeviceRecord[] }>(`/api/admin/devices/${id}`)
}

// renameDevice is the only per-device config left -- devices otherwise
// appear/disappear purely based on whether they've reported data (see
// docs/UbiBot开放平台硬件通信协议.md §6).
export function renameDevice(id: number, name: string) {
  return api.patch<Device>(`/api/admin/devices/${id}`, { name })
}

export function setDeviceStatus(id: number, status: number) {
  return api.post<{ message: string }>(`/api/admin/devices/${id}/status`, { status })
}

// deleteDevice permanently removes the device and all of its associated
// data (telemetry, alert rules/events). Irreversible -- callers must
// confirm with the user first.
export function deleteDevice(id: number) {
  return api.del<{ message: string }>(`/api/admin/devices/${id}`)
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
