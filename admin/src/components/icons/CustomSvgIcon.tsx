import { useMemo } from 'react'
import { prepareSvgForInline } from '../../utils/svg'

// Renders an admin-uploaded SVG icon (from the 图标库 library) inline,
// sized to match our built-in icon components. svg is sanitized and
// re-sized on every render via prepareSvgForInline (utils/svg.ts) before it
// ever reaches dangerouslySetInnerHTML.
export default function CustomSvgIcon({ svg }: { svg: string }) {
  const html = useMemo(() => prepareSvgForInline(svg), [svg])
  return (
    <span
      style={{ display: 'inline-flex', width: '1em', height: '1em', verticalAlign: '-0.125em' }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
