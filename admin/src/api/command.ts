import { api } from './client'
import type { DeviceCommand } from './device'

export interface GlobalCommand extends DeviceCommand {
  device_id: number
  device_name: string
}

// listAllCommands is the "指令管理" page's backing call — cross-device
// command history, as opposed to device.ts's listCommands which is scoped
// to one device's detail view.
export function listAllCommands(opts: {
  deviceId?: number
  status?: string
  type?: string
  page?: number
  pageSize?: number
}) {
  const params = new URLSearchParams()
  if (opts.deviceId) params.set('device_id', String(opts.deviceId))
  if (opts.status) params.set('status', opts.status)
  if (opts.type) params.set('type', opts.type)
  params.set('page', String(opts.page ?? 1))
  params.set('page_size', String(opts.pageSize ?? 20))
  return api.get<{ list: GlobalCommand[]; total: number }>(`/api/admin/commands?${params}`)
}
