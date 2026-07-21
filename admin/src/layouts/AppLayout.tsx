import { useMemo, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Layout, Menu, Breadcrumb, Button, Badge, Dropdown, Avatar, List, Popover, Typography } from 'antd'
import type { MenuProps } from 'antd'
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  BulbOutlined,
  BulbFilled,
  BellOutlined,
  UserOutlined,
  SettingOutlined,
  LogoutOutlined,
  DownOutlined,
  ApiOutlined,
} from '@ant-design/icons'
import { menuTree, type MenuNode } from '../router/menu'
import { useThemeMode } from '../contexts/ThemeContext'
import { useAuth } from '../contexts/AuthContext'

const { Header, Sider, Content } = Layout

function toMenuItems(nodes: MenuNode[]): MenuProps['items'] {
  return nodes.map((node) =>
    node.children
      ? { key: node.key, icon: node.icon, label: node.label, children: toMenuItems(node.children) }
      : { key: node.key, icon: node.icon, label: node.label },
  )
}

const menuItems = toMenuItems(menuTree)

function findTrail(pathname: string): MenuNode[] {
  for (const top of menuTree) {
    if (top.path === pathname) return [top]
    for (const child of top.children ?? []) {
      if (child.path === pathname) return [top, child]
    }
  }
  return []
}

const mockNotifications = [
  { id: 1, text: 'sn_ws1_20034 温度超限 38.2℃', time: '2分钟前' },
  { id: 2, text: 'sn_ws1_20011 探头配置下发失败', time: '18分钟前' },
  { id: 3, text: 'sn_ws1_19987 超过30分钟未上报', time: '1小时前' },
]

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const { mode, toggle } = useThemeMode()
  const { username, logout } = useAuth()

  const trail = useMemo(() => findTrail(location.pathname), [location.pathname])
  const selectedKeys = trail.length ? [trail[trail.length - 1].key] : []
  const defaultOpenKeys = trail.length > 1 ? [trail[0].key] : []

  const userMenuItems: MenuProps['items'] = [
    { key: 'profile', icon: <SettingOutlined />, label: '个人设置' },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
  ]

  const handleUserMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'logout') {
      logout()
      navigate('/login', { replace: true })
    }
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} trigger={null} theme="light" width={196}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            height: 52,
            padding: '0 16px',
            fontSize: 15,
            fontWeight: 500,
            overflow: 'hidden',
            whiteSpace: 'nowrap',
          }}
        >
          <ApiOutlined />
          {!collapsed && <span>UbiBot 后台</span>}
        </div>
        <Menu
          mode="inline"
          theme="light"
          items={menuItems}
          selectedKeys={selectedKeys}
          defaultOpenKeys={defaultOpenKeys}
          onClick={({ key }) => {
            const node = [...menuTree, ...menuTree.flatMap((n) => n.children ?? [])].find(
              (n) => n.key === key,
            )
            if (node) navigate(node.path)
          }}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            display: 'flex',
            alignItems: 'center',
            padding: '0 16px',
            background: '#fff',
            borderBottom: '1px solid #e6e5e0',
          }}
        >
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed((c) => !c)}
          />
          <Breadcrumb
            style={{ marginLeft: 12, flex: 1 }}
            items={[{ title: '首页' }, ...trail.map((n) => ({ title: n.label }))]}
          />
          <Button
            type="text"
            icon={mode === 'dark' ? <BulbFilled /> : <BulbOutlined />}
            onClick={toggle}
            aria-label="切换主题"
          />
          <Popover
            placement="bottomRight"
            trigger="click"
            content={
              <List
                style={{ width: 280 }}
                size="small"
                dataSource={mockNotifications}
                renderItem={(item) => (
                  <List.Item>
                    <div>
                      <div>{item.text}</div>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {item.time}
                      </Typography.Text>
                    </div>
                  </List.Item>
                )}
              />
            }
          >
            <Button type="text" icon={<Badge dot offset={[-2, 2]}><BellOutlined /></Badge>} aria-label="通知" />
          </Popover>
          <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} trigger={['click']}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginLeft: 8, cursor: 'pointer' }}>
              <Avatar size={28} icon={<UserOutlined />} />
              <div style={{ lineHeight: 1.3 }}>
                <div style={{ fontSize: 13, fontWeight: 500 }}>{username}</div>
                <div style={{ fontSize: 11, color: 'rgba(0,0,0,0.45)' }}>管理员</div>
              </div>
              <DownOutlined style={{ fontSize: 11 }} />
            </div>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16, padding: 16, background: '#fff', borderRadius: 8 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
