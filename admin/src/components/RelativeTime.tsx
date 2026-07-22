import { Tooltip } from 'antd'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

// fromNow() picks up dayjs's global locale, which useAntdLocale keeps in
// sync with the current UI language -- so labels here relabel themselves
// on language switch without any extra wiring.
dayjs.extend(relativeTime)

// Shows "3 minutes ago" / "2 hours ago" etc. at a glance -- the exact
// timestamp is one hover away via the tooltip rather than cluttering
// whatever list or card this sits in. Used anywhere a "last seen"/
// "last reported" style timestamp is scanned in practice as recent-vs-stale
// rather than looked up as an exact value.
export default function RelativeTime({ ts, fallback }: { ts: number | null; fallback: string }) {
  if (!ts) return <span>{fallback}</span>
  return <Tooltip title={new Date(ts * 1000).toLocaleString()}>{dayjs(ts * 1000).fromNow()}</Tooltip>
}
