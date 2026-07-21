import { api } from './client'

export interface ScheduledTask {
  id: number
  name: string
  device_id: number
  cmd_type: string
  cmd_args?: Record<string, unknown>
  schedule_type: 'interval' | 'daily'
  interval_seconds?: number
  daily_at_minute?: number
  enabled: boolean
  next_run_at: number
  last_run_at: number | null
}

export interface ScheduledTaskInput {
  name: string
  device_id: number
  cmd_type: string
  cmd_args?: Record<string, unknown>
  schedule_type: 'interval' | 'daily'
  interval_seconds?: number
  daily_at_minute?: number
  enabled: boolean
}

export function listScheduledTasks() {
  return api.get<{ list: ScheduledTask[] }>('/api/admin/scheduled-tasks')
}

export function createScheduledTask(input: ScheduledTaskInput) {
  return api.post<ScheduledTask>('/api/admin/scheduled-tasks', input)
}

export function updateScheduledTask(id: number, input: ScheduledTaskInput) {
  return api.patch<{ message: string }>(`/api/admin/scheduled-tasks/${id}`, input)
}

export function deleteScheduledTask(id: number) {
  return api.del<{ message: string }>(`/api/admin/scheduled-tasks/${id}`)
}
