import {
  DashboardOutlined,
  DatabaseOutlined,
  HddOutlined,
  UnorderedListOutlined,
  LineChartOutlined,
  SendOutlined,
  BellOutlined,
  SettingOutlined,
  TeamOutlined,
  SafetyCertificateOutlined,
  FileSearchOutlined,
  ScheduleOutlined,
  CloudUploadOutlined,
  KeyOutlined,
  FolderOutlined,
  BookOutlined,
  SlidersOutlined,
  DesktopOutlined,
  PictureOutlined,
  ProfileOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

export interface MenuNode {
  key: string
  /** i18n key (in the "menu" namespace) for the display label — resolve with t() at render time. */
  label: string
  path: string
  icon?: ReactNode
  children?: MenuNode[]
}

// Single source of truth for the sider menu, the route table, and the
// breadcrumb — add a page by adding one entry here instead of touching
// three places. `label` holds a translation key (namespace "menu"); the
// consumer (AppLayout) resolves it with t() so it re-renders on language change.
export const menuTree: MenuNode[] = [
  { key: 'dashboard', label: 'dashboard', path: '/dashboard', icon: <DashboardOutlined /> },
  {
    // path is a placeholder that no route/menu-click ever targets directly
    // (deliberately distinct from any child's path -- see findTrail in
    // AppLayout, which would otherwise short-circuit on it before matching
    // the actual child and both mis-highlight the sider and fail to expand
    // it). App.tsx redirects /data-warehouse-group to /data-warehouse for
    // anyone who ends up here via a stale link/bookmark.
    key: 'data-warehouse-group',
    label: 'dataWarehouseGroup',
    path: '/data-warehouse-group',
    icon: <DatabaseOutlined />,
    children: [
      { key: 'data-warehouse', label: 'myData', path: '/data-warehouse', icon: <ProfileOutlined /> },
      { key: 'monitor', label: 'monitor', path: '/monitor', icon: <LineChartOutlined /> },
    ],
  },
  {
    // Same placeholder-path rationale as data-warehouse-group above.
    key: 'device-management',
    label: 'deviceManagement',
    path: '/device-management',
    icon: <HddOutlined />,
    children: [
      { key: 'device', label: 'device', path: '/device', icon: <UnorderedListOutlined /> },
      { key: 'command', label: 'command', path: '/command', icon: <SendOutlined /> },
      { key: 'schedule', label: 'schedule', path: '/schedule', icon: <ScheduleOutlined /> },
      { key: 'firmware', label: 'firmware', path: '/firmware', icon: <CloudUploadOutlined /> },
    ],
  },
  { key: 'alert', label: 'alert', path: '/alert', icon: <BellOutlined /> },
  {
    key: 'system',
    label: 'system.root',
    path: '/system',
    icon: <SettingOutlined />,
    children: [
      { key: 'system-admin', label: 'system.admin', path: '/system/admin', icon: <TeamOutlined /> },
      { key: 'system-role', label: 'system.role', path: '/system/role', icon: <SafetyCertificateOutlined /> },
      { key: 'system-log', label: 'system.log', path: '/system/log', icon: <FileSearchOutlined /> },
      { key: 'system-apikey', label: 'system.apikey', path: '/system/apikey', icon: <KeyOutlined /> },
      { key: 'system-files', label: 'system.files', path: '/system/files', icon: <FolderOutlined /> },
      { key: 'system-dict', label: 'system.dict', path: '/system/dict', icon: <BookOutlined /> },
      { key: 'system-params', label: 'system.params', path: '/system/params', icon: <SlidersOutlined /> },
      { key: 'system-monitor', label: 'system.monitor', path: '/system/monitor', icon: <DesktopOutlined /> },
      { key: 'system-icons', label: 'system.icons', path: '/system/icons', icon: <PictureOutlined /> },
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
