import { api, getToken } from './client'
import type { DeviceCommand } from './device'

export interface Firmware {
  id: number
  pid: string
  version: string
  filename: string
  size: number
  sha256: string
  has_sig: boolean
  created_at: number
}

export function listFirmware() {
  return api.get<{ list: Firmware[] }>('/api/admin/firmware')
}

// uploadFirmware bypasses api.post since this is the one endpoint that
// needs a real multipart/form-data body rather than JSON.
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export async function uploadFirmware(input: { pid: string; version: string; signature?: string; file: File }) {
  const form = new FormData()
  form.append('pid', input.pid)
  form.append('version', input.version)
  if (input.signature) form.append('signature', input.signature)
  form.append('file', input.file)

  const token = getToken()
  const res = await fetch(`${BASE_URL}/api/admin/firmware`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.message ?? '固件上传失败')
  return data as Firmware
}

export function deleteFirmware(id: number) {
  return api.del<{ message: string }>(`/api/admin/firmware/${id}`)
}

export interface DeviceOta {
  firmware_id: number
  version: string
  state: 'pending' | 'downloading' | 'verifying' | 'flashing' | 'rebooting' | 'success' | 'failed' | 'rolled_back'
  progress: number
  last_error?: string
}

export function getDeviceOta(deviceId: number) {
  return api.get<{ ota: DeviceOta | null }>(`/api/admin/devices/${deviceId}/ota`)
}

export function dispatchDeviceOta(deviceId: number, input: { firmware_id: number; force?: boolean }) {
  return api.post<DeviceCommand>(`/api/admin/devices/${deviceId}/ota`, input)
}

export function cancelDeviceOta(deviceId: number) {
  return api.post<DeviceCommand>(`/api/admin/devices/${deviceId}/ota/cancel`)
}
