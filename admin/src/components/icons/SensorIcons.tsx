import type { ReactNode } from 'react'

// A small, cohesive hand-drawn icon set for the sensor-field types with a
// conventional default meaning (field1/field2/field3 -- see docs/UbiBot开放
// 平台硬件通信协议.md §5), replacing the mismatched @ant-design/icons glyphs
// previously used on the "数据仓库" page. Each icon is a plain inline SVG
// sized off font-size (width/height: 1em) so it drops into flex layouts the
// same way an icon font would, with no dependency on any icon package.
// field4~field20 (and anything else) fall back to DefaultFieldIcon; the
// icon library management page (系统管理 > 图标库) lets an operator upload a
// custom SVG per field key to override any of these, built-in or not (see
// hooks/useFieldIcons.tsx).

export function TemperatureIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 14.76V5a2 2 0 1 0-4 0v9.76a4.5 4.5 0 1 0 4 0Z" />
      <line x1="11" y1="7" x2="11" y2="13" />
    </svg>
  )
}

export function HumidityIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor">
      <path d="M12 2C12 2 5 11.2 5 15.5A7 7 0 0 0 19 15.5C19 11.2 12 2 12 2Z" />
    </svg>
  )
}

export function LightIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round">
      <circle cx="12" cy="12" r="4" fill="currentColor" stroke="none" />
      <line x1="12" y1="1.5" x2="12" y2="4" />
      <line x1="12" y1="20" x2="12" y2="22.5" />
      <line x1="1.5" y1="12" x2="4" y2="12" />
      <line x1="20" y1="12" x2="22.5" y2="12" />
      <line x1="4.4" y1="4.4" x2="6.1" y2="6.1" />
      <line x1="17.9" y1="17.9" x2="19.6" y2="19.6" />
      <line x1="4.4" y1="19.6" x2="6.1" y2="17.9" />
      <line x1="17.9" y1="6.1" x2="19.6" y2="4.4" />
    </svg>
  )
}

export function DefaultFieldIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinejoin="round">
      <path d="M12 3 3 12v6a3 3 0 0 0 3 3h6l9-9V3Z" />
      <circle cx="8" cy="8" r="1.3" fill="currentColor" stroke="none" />
    </svg>
  )
}

export interface FieldIconMeta {
  icon: ReactNode
  color: string
}

// field1/field2/field3 have a conventional default meaning (temperature/
// humidity/light -- protocol §5); everything else (field4~field20, or any
// other key) falls back to DefaultFieldIcon via fieldIconMeta below.
export const BUILTIN_FIELD_ICONS: Record<string, FieldIconMeta> = {
  field1: { icon: <TemperatureIcon />, color: '#fa541c' },
  field2: { icon: <HumidityIcon />, color: '#1677ff' },
  field3: { icon: <LightIcon />, color: '#faad14' },
}

export function fieldIconMeta(key: string): FieldIconMeta {
  return BUILTIN_FIELD_ICONS[key.toLowerCase()] ?? { icon: <DefaultFieldIcon />, color: '#8c8c8c' }
}
