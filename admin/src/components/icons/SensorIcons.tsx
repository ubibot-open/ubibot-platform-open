import type { ReactNode } from 'react'

// A small, cohesive hand-drawn icon set for common sensor-field types,
// replacing the mismatched @ant-design/icons glyphs previously used on the
// "数据仓库" page. Each icon is a plain inline SVG sized off font-size
// (width/height: 1em) so it drops into flex layouts the same way an icon
// font would, with no dependency on any icon package. Unrecognized field
// keys (including ones a probe reports that we've never seen before) fall
// back to DefaultFieldIcon; the icon library management page (系统管理 >
// 图标库) lets an operator upload a custom SVG per field key to override
// any of these, built-in or not (see hooks/useFieldIcons.tsx).

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

export function BatteryIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinejoin="round">
      <rect x="2" y="7" width="17" height="10" rx="2" />
      <line x1="21.5" y1="10.3" x2="21.5" y2="13.7" strokeLinecap="round" />
      <path d="M9.5 9.2h3l-1.6 2.6h2.4L9.5 16l0.7-3.4H8.4Z" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function VoltageIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor">
      <path d="M13 2 4 14h6l-1 8 9-12h-6l1-8Z" />
    </svg>
  )
}

export function SignalIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor">
      <rect x="2" y="15.5" width="3.6" height="6.5" rx="1" />
      <rect x="8.2" y="11" width="3.6" height="11" rx="1" />
      <rect x="14.4" y="6.5" width="3.6" height="15.5" rx="1" />
      <rect x="20.6" y="2" width="1.6" height="20" rx="0.8" opacity={0.35} />
    </svg>
  )
}

export function WifiIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round">
      <path d="M3 8.5a15 15 0 0 1 18 0" />
      <path d="M6.3 12.6a10 10 0 0 1 11.4 0" />
      <path d="M9.7 16.6a5 5 0 0 1 4.6 0" />
      <circle cx="12" cy="20" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function InterfaceIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round">
      <rect x="7" y="2.5" width="10" height="6" rx="1.5" />
      <line x1="12" y1="8.5" x2="12" y2="13.5" />
      <path d="M8 13.5h8v3a4 4 0 0 1-8 0Z" />
      <line x1="12" y1="16.5" x2="12" y2="21" />
    </svg>
  )
}

export function Co2Icon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinejoin="round">
      <path d="M7 18.5a4 4 0 0 1-.5-7.97 5 5 0 0 1 9.62-1.9A4.5 4.5 0 0 1 17.5 18.5Z" />
    </svg>
  )
}

export function PressureIcon() {
  return (
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round">
      <path d="M4 16a8 8 0 1 1 16 0" />
      <line x1="12" y1="16" x2="15.6" y2="11.8" />
      <circle cx="12" cy="16" r="1.2" fill="currentColor" stroke="none" />
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

// Best-effort icon + color per common sensor-field name -- a hint, not an
// exhaustive map (probes can report arbitrary field keys). Unrecognized
// keys fall back to DefaultFieldIcon via fieldIconMeta below.
export const BUILTIN_FIELD_ICONS: Record<string, FieldIconMeta> = {
  temperature: { icon: <TemperatureIcon />, color: '#fa541c' },
  humidity: { icon: <HumidityIcon />, color: '#1677ff' },
  light: { icon: <LightIcon />, color: '#faad14' },
  battery: { icon: <BatteryIcon />, color: '#52c41a' },
  voltage: { icon: <VoltageIcon />, color: '#faad14' },
  rssi: { icon: <SignalIcon />, color: '#8c8c8c' },
  signal: { icon: <SignalIcon />, color: '#8c8c8c' },
  wifi: { icon: <WifiIcon />, color: '#8c8c8c' },
  interface: { icon: <InterfaceIcon />, color: '#8c8c8c' },
  co2: { icon: <Co2Icon />, color: '#52c41a' },
  pressure: { icon: <PressureIcon />, color: '#722ed1' },
}

export function fieldIconMeta(key: string): FieldIconMeta {
  return BUILTIN_FIELD_ICONS[key.toLowerCase()] ?? { icon: <DefaultFieldIcon />, color: '#8c8c8c' }
}
