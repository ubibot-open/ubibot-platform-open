import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, message } from 'antd'
import { WifiOutlined, DisconnectOutlined, AlertOutlined, CloudUploadOutlined } from '@ant-design/icons'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useTranslation } from 'react-i18next'
import { getDashboardSummary, getDashboardTrends, type DailyCount, type DashboardSummary } from '../../api/dashboard'
import { listAlertEvents, type AlertEvent } from '../../api/alert'
import { apiErrorMessage } from '../../api/errors'

export default function DashboardPage() {
  const { t } = useTranslation('dashboard')
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [trends, setTrends] = useState<DailyCount[]>([])
  const [alerts, setAlerts] = useState<AlertEvent[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    Promise.all([
      getDashboardSummary(),
      getDashboardTrends(),
      listAlertEvents({ status: 'open', page: 1, pageSize: 5 }),
    ])
      .then(([s, t, a]) => {
        setSummary(s)
        setTrends(t.days)
        setAlerts(a.list)
      })
      .catch((e) => message.error(apiErrorMessage(e, t('loadFailed'))))
      .finally(() => setLoading(false))
  }, [])

  const stats = [
    { label: t('stats.online'), value: summary?.device_online ?? 0, icon: <WifiOutlined />, color: '#0F6E56' },
    {
      label: t('stats.offline'),
      value: summary ? summary.device_total - summary.device_online : 0,
      icon: <DisconnectOutlined />,
      color: '#5F5E5A',
    },
    { label: t('stats.openAlerts'), value: summary?.open_alerts ?? 0, icon: <AlertOutlined />, color: '#A32D2D' },
    { label: t('stats.todayRecords'), value: summary?.today_records ?? 0, icon: <CloudUploadOutlined />, color: '#185FA5' },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('title')}
      </Typography.Title>
      <Row gutter={16}>
        {stats.map((s) => (
          <Col span={6} key={s.label}>
            <Card loading={loading}>
              <Statistic title={s.label} value={s.value} prefix={<span style={{ color: s.color }}>{s.icon}</span>} />
            </Card>
          </Col>
        ))}
      </Row>
      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={16}>
          <Card title={t('trendsTitle')} loading={loading}>
            <div style={{ height: 260 }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={trends}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="day" fontSize={12} />
                  <YAxis fontSize={12} allowDecimals={false} />
                  <Tooltip />
                  <Bar dataKey="count" fill="#185FA5" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card title={t('openAlertsTitle')} loading={loading}>
            {alerts.length === 0 ? (
              <Typography.Text type="secondary">{t('noOpenAlerts')}</Typography.Text>
            ) : (
              alerts.map((a) => (
                <div key={a.id} style={{ marginBottom: 10 }}>
                  <div style={{ fontSize: 13 }}>{a.device_name}</div>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {a.message}
                  </Typography.Text>
                </div>
              ))
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
