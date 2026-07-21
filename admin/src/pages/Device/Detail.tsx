import { useEffect, useState, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ArrowLeftOutlined, PlusOutlined, SendOutlined } from '@ant-design/icons'
import {
  dispatchCommand,
  getDevice,
  listProbes,
  upsertProbe,
  removeProbe,
  type Device,
  type DeviceCommand,
  type DeviceRecord,
  type Probe,
} from '../../api/device'
import { listAlertRules, createAlertRule, deleteAlertRule, type AlertRule } from '../../api/alert'
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

const probeStatusTag: Record<Probe['status'], { color: string; text: string }> = {
  pending: { color: 'processing', text: '待生效' },
  applied: { color: 'success', text: '已生效' },
  failed: { color: 'error', text: '失败' },
  removing: { color: 'warning', text: '删除中' },
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

  const [probes, setProbes] = useState<Probe[]>([])
  const [probeSubmitting, setProbeSubmitting] = useState(false)
  const [probeForm] = Form.useForm()

  const [alertRules, setAlertRules] = useState<AlertRule[]>([])
  const [ruleSubmitting, setRuleSubmitting] = useState(false)
  const [ruleForm] = Form.useForm()

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

  const loadProbes = useCallback(async () => {
    try {
      const res = await listProbes(deviceId)
      setProbes(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载探头配置失败')
    }
  }, [deviceId])

  const loadAlertRules = useCallback(async () => {
    try {
      const res = await listAlertRules(deviceId)
      setAlertRules(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载告警规则失败')
    }
  }, [deviceId])

  useEffect(() => {
    load()
    loadProbes()
    loadAlertRules()
  }, [load, loadProbes, loadAlertRules])

  const onUpsertProbe = async (values: {
    pid: string
    key: string
    iface: string
    proto: string
    params?: string
  }) => {
    let params: Record<string, unknown> | undefined
    if (values.params) {
      try {
        params = JSON.parse(values.params)
      } catch {
        message.error('探头参数必须是合法 JSON')
        return
      }
    }
    setProbeSubmitting(true)
    try {
      await upsertProbe(deviceId, { pid: values.pid, key: values.key, iface: values.iface, proto: values.proto, params })
      message.success('探头配置已下发，等待设备确认')
      probeForm.resetFields()
      loadProbes()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '配置探头失败')
    } finally {
      setProbeSubmitting(false)
    }
  }

  const onRemoveProbe = async (pid: string) => {
    try {
      await removeProbe(deviceId, pid)
      message.success('已请求删除探头，等待设备确认')
      loadProbes()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除探头失败')
    }
  }

  const onCreateRule = async (values: { field: string; op: AlertRule['op']; threshold: number }) => {
    setRuleSubmitting(true)
    try {
      await createAlertRule(deviceId, values)
      message.success('告警规则已创建')
      ruleForm.resetFields()
      loadAlertRules()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '创建告警规则失败')
    } finally {
      setRuleSubmitting(false)
    }
  }

  const onDeleteRule = async (id: number) => {
    try {
      await deleteAlertRule(id)
      message.success('已删除')
      loadAlertRules()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

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
            <Descriptions.Item label="在线状态">
              {device.online ? <Tag color="green">在线</Tag> : <Tag color="default">离线</Tag>}
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

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={14}>
          <Card title="探头自定义数据读取">
            <Form form={probeForm} layout="inline" onFinish={onUpsertProbe} style={{ marginBottom: 16, rowGap: 8 }}>
              <Form.Item name="pid" rules={[{ required: true, message: '探头ID' }]}>
                <Input placeholder="探头ID (pid)" style={{ width: 110 }} />
              </Form.Item>
              <Form.Item name="key">
                <Input placeholder="字段名 (key)" style={{ width: 110 }} />
              </Form.Item>
              <Form.Item name="iface" rules={[{ required: true, message: '接口' }]}>
                <Select placeholder="接口" style={{ width: 100 }} options={[
                  { value: 'rs485', label: 'RS485' },
                  { value: 'analog', label: 'Analog' },
                  { value: 'digital', label: 'Digital' },
                ]} />
              </Form.Item>
              <Form.Item name="proto" rules={[{ required: true, message: '协议' }]}>
                <Select placeholder="协议" style={{ width: 100 }} options={[
                  { value: 'modbus', label: 'Modbus' },
                  { value: 'raw', label: 'Raw' },
                ]} />
              </Form.Item>
              <Form.Item name="params" style={{ flex: 1, minWidth: 160 }}>
                <Input placeholder='其它参数 JSON，如 {"addr":1,"fc":3,"reg":0}' />
              </Form.Item>
              <Form.Item>
                <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={probeSubmitting}>
                  下发配置
                </Button>
              </Form.Item>
            </Form>
            <Table
              rowKey="pid"
              size="small"
              dataSource={probes}
              pagination={false}
              columns={[
                { title: 'pid', dataIndex: 'pid', width: 80 },
                { title: 'key', dataIndex: 'key', width: 100 },
                { title: '接口/协议', render: (_, r) => `${r.iface}/${r.proto}` },
                { title: '参数', dataIndex: 'params', render: (p) => (p ? JSON.stringify(p) : '-') },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 90,
                  render: (status: Probe['status'], r) => (
                    <Space direction="vertical" size={0}>
                      <Tag color={probeStatusTag[status].color}>{probeStatusTag[status].text}</Tag>
                      {r.last_error && (
                        <Typography.Text type="danger" style={{ fontSize: 12 }}>
                          {r.last_error}
                        </Typography.Text>
                      )}
                    </Space>
                  ),
                },
                {
                  title: '操作',
                  width: 80,
                  render: (_, r) => (
                    <Popconfirm title="确认删除该探头？" onConfirm={() => onRemoveProbe(r.pid)}>
                      <a>删除</a>
                    </Popconfirm>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
        <Col span={10}>
          <Card title="阈值告警规则">
            <Form form={ruleForm} layout="inline" onFinish={onCreateRule} style={{ marginBottom: 16, rowGap: 8 }}>
              <Form.Item name="field" rules={[{ required: true, message: '字段名' }]}>
                <Input placeholder="字段名，如 temperature" style={{ width: 140 }} />
              </Form.Item>
              <Form.Item name="op" rules={[{ required: true, message: '比较符' }]} initialValue=">">
                <Select
                  style={{ width: 80 }}
                  options={['>', '>=', '<', '<=', '=='].map((op) => ({ value: op, label: op }))}
                />
              </Form.Item>
              <Form.Item name="threshold" rules={[{ required: true, message: '阈值' }]}>
                <InputNumber placeholder="阈值" />
              </Form.Item>
              <Form.Item>
                <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={ruleSubmitting}>
                  添加
                </Button>
              </Form.Item>
            </Form>
            <Table
              rowKey="id"
              size="small"
              dataSource={alertRules}
              pagination={false}
              columns={[
                { title: '字段', dataIndex: 'field' },
                { title: '条件', render: (_, r) => `${r.op} ${r.threshold}` },
                {
                  title: '操作',
                  width: 80,
                  render: (_, r) => (
                    <Popconfirm title="确认删除该规则？" onConfirm={() => onDeleteRule(r.id)}>
                      <a>删除</a>
                    </Popconfirm>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
