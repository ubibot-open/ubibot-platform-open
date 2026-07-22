import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, message } from 'antd'
import { useTranslation } from 'react-i18next'
import { getSystemMetrics, type SystemMetrics } from '../../api/system'
import { apiErrorMessage } from '../../api/errors'

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatUptime(seconds: number, t: (key: string, opts?: Record<string, unknown>) => string) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return t('uptimeFormat', { d, h, m })
}

export default function SystemMonitorPage() {
  const { t } = useTranslation('systemMonitor')
  const [metrics, setMetrics] = useState<SystemMetrics | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      setMetrics(await getSystemMetrics())
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const timer = setInterval(load, 10000)
    return () => clearInterval(timer)
  }, [])

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('pageTitle')}
      </Typography.Title>
      <Typography.Paragraph type="secondary">{t('refreshNote')}</Typography.Paragraph>
      <Row gutter={16}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.uptime')} value={metrics ? formatUptime(metrics.uptime_seconds, t) : '-'} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.goroutines')} value={metrics?.goroutines ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.heapAlloc')} value={metrics ? formatBytes(metrics.heap_alloc_bytes) : '-'} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.dbSize')} value={metrics ? formatBytes(metrics.db_size_bytes) : '-'} />
          </Card>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.deviceTotal')} value={metrics?.device_total ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.pendingCommands')} value={metrics?.pending_commands ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.openAlerts')} value={metrics?.open_alerts ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title={t('stats.unreadNotifications')} value={metrics?.unread_notifications ?? 0} />
          </Card>
        </Col>
      </Row>
      <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
        {t('goVersionLabel', { version: metrics?.go_version ?? '-' })}
      </Typography.Paragraph>
    </div>
  )
}
