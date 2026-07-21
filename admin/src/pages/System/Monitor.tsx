import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, message } from 'antd'
import { getSystemMetrics, type SystemMetrics } from '../../api/system'
import { ApiError } from '../../api/client'

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatUptime(seconds: number) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${d}天${h}时${m}分`
}

export default function SystemMonitorPage() {
  const [metrics, setMetrics] = useState<SystemMetrics | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      setMetrics(await getSystemMetrics())
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载系统指标失败')
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
        系统监控
      </Typography.Title>
      <Typography.Paragraph type="secondary">每10秒自动刷新。</Typography.Paragraph>
      <Row gutter={16}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="运行时长" value={metrics ? formatUptime(metrics.uptime_seconds) : '-'} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="Goroutine 数" value={metrics?.goroutines ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="堆内存占用" value={metrics ? formatBytes(metrics.heap_alloc_bytes) : '-'} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="数据库文件大小" value={metrics ? formatBytes(metrics.db_size_bytes) : '-'} />
          </Card>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="设备总数" value={metrics?.device_total ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="待确认指令" value={metrics?.pending_commands ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="进行中告警" value={metrics?.open_alerts ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="未读通知" value={metrics?.unread_notifications ?? 0} />
          </Card>
        </Col>
      </Row>
      <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
        Go 版本：{metrics?.go_version ?? '-'}
      </Typography.Paragraph>
    </div>
  )
}
