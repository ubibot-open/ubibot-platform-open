import { Card, Col, Row, Statistic, Typography } from 'antd'
import {
  WifiOutlined,
  DisconnectOutlined,
  AlertOutlined,
  CloudUploadOutlined,
} from '@ant-design/icons'

const stats = [
  { label: '在线设备', value: 1284, icon: <WifiOutlined />, color: '#0F6E56' },
  { label: '离线设备', value: 36, icon: <DisconnectOutlined />, color: '#5F5E5A' },
  { label: '今日告警', value: 5, icon: <AlertOutlined />, color: '#A32D2D' },
  { label: '今日上报', value: 128940, icon: <CloudUploadOutlined />, color: '#185FA5' },
]

// Placeholder numbers — swap for a real summary API once one exists.
export default function DashboardPage() {
  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        仪表盘
      </Typography.Title>
      <Row gutter={16}>
        {stats.map((s) => (
          <Col span={6} key={s.label}>
            <Card>
              <Statistic
                title={s.label}
                value={s.value}
                prefix={<span style={{ color: s.color }}>{s.icon}</span>}
              />
            </Card>
          </Col>
        ))}
      </Row>
      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={16}>
          <Card title="设备上报趋势">
            <Typography.Text type="secondary">图表待接入</Typography.Text>
          </Card>
        </Col>
        <Col span={8}>
          <Card title="最新告警">
            <Typography.Text type="secondary">暂无数据</Typography.Text>
          </Card>
        </Col>
      </Row>
    </div>
  )
}
