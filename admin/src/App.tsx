import { ConfigProvider, theme } from 'antd'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { useThemeMode } from './contexts/ThemeContext'
import { useAntdLocale } from './i18n/useAntdLocale'
import { AuthProvider } from './contexts/AuthContext'
import RequireAuth from './components/RequireAuth'
import AppLayout from './layouts/AppLayout'
import LoginPage from './pages/Login'
import DashboardPage from './pages/Dashboard'
import DevicePage from './pages/Device'
import DeviceDetailPage from './pages/Device/Detail'
import MonitorPage from './pages/Monitor'
import CommandPage from './pages/Command'
import AlertPage from './pages/Alert'
import AdminUserPage from './pages/System/Admin'
import RolePage from './pages/System/Role'
import AuditLogPage from './pages/System/Log'
import SchedulePage from './pages/System/Schedule'
import FirmwarePage from './pages/System/Firmware'
import ApiKeyPage from './pages/System/ApiKey'
import FilesPage from './pages/System/Files'
import DictPage from './pages/System/Dict'
import ParamsPage from './pages/System/Params'
import SystemMonitorPage from './pages/System/Monitor'

export default function App() {
  const { mode } = useThemeMode()
  const antdLocale = useAntdLocale()

  return (
    <ConfigProvider
      locale={antdLocale}
      theme={{
        algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: { colorPrimary: '#185FA5', borderRadius: 8 },
      }}
    >
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route
              element={
                <RequireAuth>
                  <AppLayout />
                </RequireAuth>
              }
            >
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/device" element={<DevicePage />} />
              <Route path="/device/:id" element={<DeviceDetailPage />} />
              <Route path="/monitor" element={<MonitorPage />} />
              <Route path="/command" element={<CommandPage />} />
              <Route path="/alert" element={<AlertPage />} />
              <Route path="/schedule" element={<SchedulePage />} />
              <Route path="/firmware" element={<FirmwarePage />} />
              <Route path="/system/admin" element={<AdminUserPage />} />
              <Route path="/system/role" element={<RolePage />} />
              <Route path="/system/log" element={<AuditLogPage />} />
              <Route path="/system/apikey" element={<ApiKeyPage />} />
              <Route path="/system/files" element={<FilesPage />} />
              <Route path="/system/dict" element={<DictPage />} />
              <Route path="/system/params" element={<ParamsPage />} />
              <Route path="/system/monitor" element={<SystemMonitorPage />} />
              <Route path="/system" element={<Navigate to="/system/admin" replace />} />
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Route>
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </ConfigProvider>
  )
}
