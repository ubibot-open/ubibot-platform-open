import { useEffect, useState, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ArrowLeftOutlined, SendOutlined } from '@ant-design/icons'
import {
  dispatchCommand,
  getDevice,
  type Device,
  type DeviceCommand,
  type DeviceRecord,
} from '../../api/device'
import { ApiError } from '../../api/client'

const statusTag: Record<DeviceCommand['status'], { color: string; text: string }> = {
  pending: { color: 'processing', text: '待确认' },
  acked: { color: 'success', text: '已确认' },
  nacked: { color: 'error', text: '执行失败' },
}

function formatTime(ts: number | null) {
  if (!ts) return '从未上报'
  return new Date(ts * 1000).toLocaleString()
}

export default function DeviceDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const deviceId = Number(id)

  const [device, setDevice] = useState<Device | null>(null)
  const [records, setRecords] = useState<DeviceRecord[]>([])
  const [commands, setCommands] = useState<DeviceCommand[]>([])
  const [loading, setLoading] = useState(false)
  const [dispatching, setDispatching] = useState(false)
  const [form] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getDevice(deviceId)
      setDevice(res.device)
      setRecords(res.records)
      setCommands(res.commands)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载设备详情失败')
    } finally {
      setLoading(false)
    }
  }, [deviceId])

  useEffect(() => {
    load()
  }, [load])

  const onDispatch = async (values: { type: string; args?: string }) => {
    let args: Record<string, unknown> | undefined
    if (values.args) {
      try {
        args = JSON.parse(values.args)
      } catch {
        message.error('指令参数必须是合法 JSON')
        return
      }
    }

    setDispatching(true)
    try {
      await dispatchCommand(deviceId, { type: values.type, args })
      message.success('指令已下发，等待设备下次上报确认')
      form.resetFields()
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '下发失败')
    } finally {
      setDispatching(false)
    }
  }

  const recordColumns: ColumnsType<DeviceRecord> = [
    { title: '时间', dataIndex: 'ts', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    { title: '数据', dataIndex: 'd', render: (d: Record<string, unknown>) => JSON.stringify(d) },
  ]

  const commandColumns: ColumnsType<DeviceCommand> = [
    { title: '指令ID', dataIndex: 'id', width: 90 },
    { title: '类型', dataIndex: 'type' },
    { title: '参数', dataIndex: 'args', render: (a?: Record<string, unknown>) => (a ? JSON.stringify(a) : '-') },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (status: DeviceCommand['status'], r) => (
        <Space direction="vertical" size={0}>
          <Tag color={statusTag[status].color}>{statusTag[status].text}</Tag>
          {r.nak_message && (
            <Typography.Text type="danger" style={{ fontSize: 12 }}>
              {r.nak_message}
            </Typography.Text>
          )}
        </Space>
      ),
    },
    { title: '下发时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
  ]

  return (
    <div>
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/device')} style={{ paddingLeft: 0 }}>
        返回设备列表
      </Button>

      {device && (
        <Card style={{ marginBottom: 16 }}>
          <Descriptions title={device.name || device.sn} column={3}>
            <Descriptions.Item label="SN">{device.sn}</Descriptions.Item>
            <Descriptions.Item label="PID">{device.pid}</Descriptions.Item>
            <Descriptions.Item label="状态">
              {device.status === 1 ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="采集间隔">{device.ci} 秒</Descriptions.Item>
            <Descriptions.Item label="上报间隔">{device.ui} 秒</Descriptions.Item>
            <Descriptions.Item label="最近上报">{formatTime(device.last_seen_at)}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <Row gutter={16}>
        <Col span={14}>
          <Card title="最近上报数据" loading={loading}>
            <Table rowKey="ts" size="small" columns={recordColumns} dataSource={records} pagination={false} />
          </Card>
        </Col>
        <Col span={10}>
          <Card title="下发指令" style={{ marginBottom: 16 }}>
            <Form form={form} layout="vertical" onFinish={onDispatch}>
              <Form.Item name="type" label="指令类型" rules={[{ required: true, message: '请选择指令类型' }]}>
                <Select
                  placeholder="选择或输入指令类型"
                  options={[
                    { value: 'reboot', label: 'reboot（重启）' },
                    { value: 'calibrate', label: 'calibrate（校准）' },
                    { value: 'set_cfg', label: 'set_cfg（更新配置）' },
                  ]}
                  showSearch
                  allowClear
                />
              </Form.Item>
              <Form.Item name="args" label="参数 (JSON，可选)">
                <Input.TextArea rows={3} placeholder='例如 {"ui": 300}' />
              </Form.Item>
              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" htmlType="submit" icon={<SendOutlined />} loading={dispatching} block>
                  下发指令
                </Button>
              </Form.Item>
            </Form>
          </Card>
          <Card title="指令历史" loading={loading}>
            <Table rowKey="id" size="small" columns={commandColumns} dataSource={commands} pagination={false} />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
