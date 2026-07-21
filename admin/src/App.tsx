import { ConfigProvider, theme } from 'antd'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { useThemeMode } from './contexts/ThemeContext'
import AppLayout from './layouts/AppLayout'
import DashboardPage from './pages/Dashboard'
import DevicePage from './pages/Device'
import MonitorPage from './pages/Monitor'
import CommandPage from './pages/Command'
import AlertPage from './pages/Alert'
import SystemPage from './pages/System'

export default function App() {
  const { mode } = useThemeMode()

  return (
    <ConfigProvider
      theme={{
        algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: { colorPrimary: '#185FA5', borderRadius: 8 },
      }}
    >
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/device" element={<DevicePage />} />
            <Route path="/monitor" element={<MonitorPage />} />
            <Route path="/command" element={<CommandPage />} />
            <Route path="/alert" element={<AlertPage />} />
            <Route path="/system/admin" element={<SystemPage />} />
            <Route path="/system/role" element={<SystemPage />} />
            <Route path="/system/log" element={<SystemPage />} />
            <Route path="/system" element={<Navigate to="/system/admin" replace />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  )
}
