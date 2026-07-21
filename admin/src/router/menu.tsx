import {
  DashboardOutlined,
  HddOutlined,
  LineChartOutlined,
  SendOutlined,
  BellOutlined,
  SettingOutlined,
  TeamOutlined,
  SafetyCertificateOutlined,
  FileSearchOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

export interface MenuNode {
  key: string
  label: string
  path: string
  icon?: ReactNode
  children?: MenuNode[]
}

// Single source of truth for the sider menu, the route table, and the
// breadcrumb — add a page by adding one entry here instead of touching
// three places.
export const menuTree: MenuNode[] = [
  { key: 'dashboard', label: '仪表盘', path: '/dashboard', icon: <DashboardOutlined /> },
  { key: 'device', label: '设备管理', path: '/device', icon: <HddOutlined /> },
  { key: 'monitor', label: '数据监控', path: '/monitor', icon: <LineChartOutlined /> },
  { key: 'command', label: '指令下发', path: '/command', icon: <SendOutlined /> },
  { key: 'alert', label: '告警中心', path: '/alert', icon: <BellOutlined /> },
  {
    key: 'system',
    label: '系统管理',
    path: '/system',
    icon: <SettingOutlined />,
    children: [
      { key: 'system-admin', label: '管理员', path: '/system/admin', icon: <TeamOutlined /> },
      { key: 'system-role', label: '角色', path: '/system/role', icon: <SafetyCertificateOutlined /> },
      { key: 'system-log', label: '操作日志', path: '/system/log', icon: <FileSearchOutlined /> },
    ],
  },
]

// Flattened lookup keyed by path, used for breadcrumb rendering.
export const menuByPath = new Map<string, MenuNode>()
for (const node of menuTree) {
  menuByPath.set(node.path, node)
  for (const child of node.children ?? []) {
    menuByPath.set(child.path, child)
  }
}
