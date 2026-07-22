// Shared "how do we print an arbitrary reported sensor value" rule --
// used by both the 数据仓库 list page and its per-device detail page so a
// given field renders identically in both places (and in CSV exports).
export function formatFieldValue(v: unknown): string {
  if (v === null || v === undefined) return '-'
  if (typeof v === 'number') return Number.isInteger(v) ? String(v) : v.toFixed(2)
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
