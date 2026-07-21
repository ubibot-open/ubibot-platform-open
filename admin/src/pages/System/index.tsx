import { useLocation } from 'react-router-dom'
import PagePlaceholder from '../../components/PagePlaceholder'

const titleByPath: Record<string, string> = {
  '/system/admin': '管理员',
  '/system/role': '角色',
  '/system/log': '操作日志',
}

export default function SystemPage() {
  const { pathname } = useLocation()
  return <PagePlaceholder title={titleByPath[pathname] ?? '系统管理'} />
}
