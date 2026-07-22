function csvCell(v: unknown): string {
  return `"${String(v).replace(/"/g, '""')}"`
}

export function toCsv(rows: unknown[][]): string {
  return rows.map((row) => row.map(csvCell).join(',')).join('\n')
}

// Triggers a browser download of csv as filename. The leading BOM is what
// makes Excel (still the most likely consumer) detect UTF-8 instead of
// mangling any non-ASCII device names/field keys.
export function downloadCsv(filename: string, csv: string) {
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
