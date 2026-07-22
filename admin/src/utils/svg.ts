// Light defense-in-depth sanitizer for admin-uploaded SVG icons: strips
// <script> tags, inline event-handler attributes, and javascript: URLs
// before the markup is ever handed to dangerouslySetInnerHTML. Uploading
// an icon already requires the system:manage permission, so this isn't the
// only safeguard, but rendering arbitrary uploaded markup completely as-is
// would still be an unnecessary risk.
export function sanitizeSvg(svg: string): string {
  let s = svg
  s = s.replace(/<script[\s\S]*?<\/script>/gi, '')
  s = s.replace(/\son\w+\s*=\s*"[^"]*"/gi, '')
  s = s.replace(/\son\w+\s*=\s*'[^']*'/gi, '')
  s = s.replace(/(href|xlink:href)\s*=\s*"javascript:[^"]*"/gi, '')
  s = s.replace(/(href|xlink:href)\s*=\s*'javascript:[^']*'/gi, '')
  return s
}

// Forces the icon to size itself off font-size (width/height: 1em), like
// our built-in sensor icons, regardless of whatever width/height the
// uploaded file originally declared.
export function normalizeSvgSize(svg: string): string {
  let s = svg.replace(/(<svg[^>]*?)\swidth="[^"]*"/i, '$1')
  s = s.replace(/(<svg[^>]*?)\sheight="[^"]*"/i, '$1')
  if (/<svg[\s>]/i.test(s)) {
    s = s.replace(/<svg(\s|>)/i, '<svg width="1em" height="1em"$1')
  }
  return s
}

export function prepareSvgForInline(svg: string): string {
  return normalizeSvgSize(sanitizeSvg(svg))
}
