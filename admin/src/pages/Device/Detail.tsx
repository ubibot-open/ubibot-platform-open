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
  Progress,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ArrowLeftOutlined, CloudUploadOutlined, PlusOutlined, SendOutlined, StopOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
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
import { listFirmware, getDeviceOta, dispatchDeviceOta, cancelDeviceOta, type Firmware, type DeviceOta } from '../../api/ota'
import { apiErrorMessage } from '../../api/errors'

export default function DeviceDetailPage() {
  const { t } = useTranslation('deviceDetail')
  const { id } = useParams()
  const navigate = useNavigate()
  const deviceId = Number(id)

  const statusTag: Record<DeviceCommand['status'], { color: string; text: string }> = {
    pending: { color: 'processing', text: t('command.status.pending') },
    acked: { color: 'success', text: t('command.status.acked') },
    nacked: { color: 'error', text: t('command.status.nacked') },
  }

  const probeStatusTag: Record<Probe['status'], { color: string; text: string }> = {
    pending: { color: 'processing', text: t('probe.status.pending') },
    applied: { color: 'success', text: t('probe.status.applied') },
    failed: { color: 'error', text: t('probe.status.failed') },
    removing: { color: 'warning', text: t('probe.status.removing') },
  }

  const otaStateTag: Record<DeviceOta['state'], { color: string; text: string }> = {
    pending: { color: 'default', text: t('ota.state.pending') },
    downloading: { color: 'processing', text: t('ota.state.downloading') },
    verifying: { color: 'processing', text: t('ota.state.verifying') },
    flashing: { color: 'processing', text: t('ota.state.flashing') },
    rebooting: { color: 'processing', text: t('ota.state.rebooting') },
    success: { color: 'success', text: t('ota.state.success') },
    failed: { color: 'error', text: t('ota.state.failed') },
    rolled_back: { color: 'error', text: t('ota.state.rolled_back') },
  }

  function formatTime(ts: number | null) {
    if (!ts) return t('neverReported')
    return new Date(ts * 1000).toLocaleString()
  }

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

  const [firmwareList, setFirmwareList] = useState<Firmware[]>([])
  const [selectedFirmwareId, setSelectedFirmwareId] = useState<number | undefined>(undefined)
  const [ota, setOta] = useState<DeviceOta | null>(null)
  const [otaSubmitting, setOtaSubmitting] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getDevice(deviceId)
      setDevice(res.device)
      setRecords(res.records)
      setCommands(res.commands)
    } catch (e) {
      message.error(apiErrorMessage(e, t('basic.loadFailed')))
    } finally {
      setLoading(false)
    }
  }, [deviceId, t])

  const loadProbes = useCallback(async () => {
    try {
      const res = await listProbes(deviceId)
      setProbes(res.list)
    } catch (e) {
      message.error(apiErrorMessage(e, t('probe.loadFailed')))
    }
  }, [deviceId, t])

  const loadAlertRules = useCallback(async () => {
    try {
      const res = await listAlertRules(deviceId)
      setAlertRules(res.list)
    } catch (e) {
      message.error(apiErrorMessage(e, t('alert.loadFailed')))
    }
  }, [deviceId, t])

  const loadOta = useCallback(async () => {
    try {
      const res = await getDeviceOta(deviceId)
      setOta(res.ota)
    } catch (e) {
      message.error(apiErrorMessage(e, t('ota.loadFailed')))
    }
  }, [deviceId, t])

  useEffect(() => {
    load()
    loadProbes()
    loadAlertRules()
    loadOta()
    listFirmware()
      .then((res) => setFirmwareList(res.list))
      .catch(() => undefined)
  }, [load, loadProbes, loadAlertRules, loadOta])

  // While an upgrade is in flight, poll its status so progress/state
  // updates show up without the operator manually refreshing the page.
  useEffect(() => {
    if (!ota || ota.state === 'success' || ota.state === 'failed' || ota.state === 'rolled_back') return
    const timer = setInterval(loadOta, 5000)
    return () => clearInterval(timer)
  }, [ota, loadOta])

  const onDispatchOta = async () => {
    if (!selectedFirmwareId) {
      message.error(t('ota.selectFirmwareRequired'))
      return
    }
    setOtaSubmitting(true)
    try {
      await dispatchDeviceOta(deviceId, { firmware_id: selectedFirmwareId })
      message.success(t('ota.dispatchSuccess'))
      loadOta()
    } catch (e) {
      message.error(apiErrorMessage(e, t('ota.dispatchFailed')))
    } finally {
      setOtaSubmitting(false)
    }
  }

  const onCancelOta = async () => {
    try {
      await cancelDeviceOta(deviceId)
      message.success(t('ota.cancelSuccess'))
      loadOta()
    } catch (e) {
      message.error(apiErrorMessage(e, t('ota.cancelFailed')))
    }
  }

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
        message.error(t('probe.paramsInvalidJson'))
        return
      }
    }
    setProbeSubmitting(true)
    try {
      await upsertProbe(deviceId, { pid: values.pid, key: values.key, iface: values.iface, proto: values.proto, params })
      message.success(t('probe.upsertSuccess'))
      probeForm.resetFields()
      loadProbes()
    } catch (e) {
      message.error(apiErrorMessage(e, t('probe.upsertFailed')))
    } finally {
      setProbeSubmitting(false)
    }
  }

  const onRemoveProbe = async (pid: string) => {
    try {
      await removeProbe(deviceId, pid)
      message.success(t('probe.removeSuccess'))
      loadProbes()
    } catch (e) {
      message.error(apiErrorMessage(e, t('probe.removeFailed')))
    }
  }

  const onCreateRule = async (values: { field: string; op: AlertRule['op']; threshold: number }) => {
    setRuleSubmitting(true)
    try {
      await createAlertRule(deviceId, values)
      message.success(t('alert.createSuccess'))
      ruleForm.resetFields()
      loadAlertRules()
    } catch (e) {
      message.error(apiErrorMessage(e, t('alert.createFailed')))
    } finally {
      setRuleSubmitting(false)
    }
  }

  const onDeleteRule = async (id: number) => {
    try {
      await deleteAlertRule(id)
      message.success(t('common:deleteSuccess'))
      loadAlertRules()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:deleteFailed')))
    }
  }

  const onDispatch = async (values: { type: string; args?: string }) => {
    let args: Record<string, unknown> | undefined
    if (values.args) {
      try {
        args = JSON.parse(values.args)
      } catch {
        message.error(t('command.argsInvalidJson'))
        return
      }
    }

    setDispatching(true)
    try {
      await dispatchCommand(deviceId, { type: values.type, args })
      message.success(t('command.dispatchSuccess'))
      form.resetFields()
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('command.dispatchFailed')))
    } finally {
      setDispatching(false)
    }
  }

  const recordColumns: ColumnsType<DeviceRecord> = [
    { title: t('records.columns.time'), dataIndex: 'ts', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    { title: t('records.columns.data'), dataIndex: 'd', render: (d: Record<string, unknown>) => JSON.stringify(d) },
  ]

  const commandColumns: ColumnsType<DeviceCommand> = [
    { title: t('command.columns.id'), dataIndex: 'id', width: 90 },
    { title: t('command.columns.type'), dataIndex: 'type' },
    { title: t('command.columns.args'), dataIndex: 'args', render: (a?: Record<string, unknown>) => (a ? JSON.stringify(a) : '-') },
    {
      title: t('command.columns.status'),
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
    { title: t('command.columns.createdAt'), dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
  ]

  return (
    <div>
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/device')} style={{ paddingLeft: 0 }}>
        {t('backToList')}
      </Button>

      {device && (
        <Card style={{ marginBottom: 16 }}>
          <Descriptions title={device.name || device.sn} column={3}>
            <Descriptions.Item label="SN">{device.sn}</Descriptions.Item>
            <Descriptions.Item label="PID">{device.pid}</Descriptions.Item>
            <Descriptions.Item label={t('basic.statusLabel')}>
              {device.status === 1 ? (
                <Tag color="success">{t('common:enabled')}</Tag>
              ) : (
                <Tag>{t('common:disabled')}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.activatedLabel')}>
              {device.activated ? (
                <Tag color="blue">{t('basic.activated.yes')}</Tag>
              ) : (
                <Tag color="default">{t('basic.activated.no')}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.onlineLabel')}>
              {device.online ? (
                <Tag color="green">{t('basic.online.yes')}</Tag>
              ) : (
                <Tag color="default">{t('basic.online.no')}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.ciLabel')}>
              {device.ci} {t('basic.seconds')}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.uiLabel')}>
              {device.ui} {t('basic.seconds')}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.lastSeenLabel')}>{formatTime(device.last_seen_at)}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <Row gutter={16}>
        <Col span={14}>
          <Card title={t('records.title')} loading={loading}>
            <Table rowKey="ts" size="small" columns={recordColumns} dataSource={records} pagination={false} />
          </Card>
        </Col>
        <Col span={10}>
          <Card title={t('command.dispatchTitle')} style={{ marginBottom: 16 }}>
            <Form form={form} layout="vertical" onFinish={onDispatch}>
              <Form.Item name="type" label={t('command.form.typeLabel')} rules={[{ required: true, message: t('command.form.typeRequired') }]}>
                <Select
                  placeholder={t('command.form.typePlaceholder')}
                  options={[
                    { value: 'reboot', label: t('command.form.typeOptions.reboot') },
                    { value: 'calibrate', label: t('command.form.typeOptions.calibrate') },
                    { value: 'set_cfg', label: t('command.form.typeOptions.setCfg') },
                  ]}
                  showSearch
                  allowClear
                />
              </Form.Item>
              <Form.Item name="args" label={t('command.form.argsLabel')}>
                <Input.TextArea rows={3} placeholder={t('command.form.argsPlaceholder')} />
              </Form.Item>
              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" htmlType="submit" icon={<SendOutlined />} loading={dispatching} block>
                  {t('command.form.submit')}
                </Button>
              </Form.Item>
            </Form>
          </Card>
          <Card title={t('command.historyTitle')} loading={loading}>
            <Table rowKey="id" size="small" columns={commandColumns} dataSource={commands} pagination={false} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={14}>
          <Card title={t('probe.title')}>
            <Form form={probeForm} layout="inline" onFinish={onUpsertProbe} style={{ marginBottom: 16, rowGap: 8 }}>
              <Form.Item name="pid" rules={[{ required: true, message: t('probe.form.pidRequired') }]}>
                <Input placeholder={t('probe.form.pidPlaceholder')} style={{ width: 110 }} />
              </Form.Item>
              <Form.Item name="key">
                <Input placeholder={t('probe.form.keyPlaceholder')} style={{ width: 110 }} />
              </Form.Item>
              <Form.Item name="iface" rules={[{ required: true, message: t('probe.form.ifaceRequired') }]}>
                <Select placeholder={t('probe.form.ifacePlaceholder')} style={{ width: 100 }} options={[
                  { value: 'rs485', label: 'RS485' },
                  { value: 'analog', label: 'Analog' },
                  { value: 'digital', label: 'Digital' },
                ]} />
              </Form.Item>
              <Form.Item name="proto" rules={[{ required: true, message: t('probe.form.protoRequired') }]}>
                <Select placeholder={t('probe.form.protoPlaceholder')} style={{ width: 100 }} options={[
                  { value: 'modbus', label: 'Modbus' },
                  { value: 'raw', label: 'Raw' },
                ]} />
              </Form.Item>
              <Form.Item name="params" style={{ flex: 1, minWidth: 160 }}>
                <Input placeholder={t('probe.form.paramsPlaceholder')} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={probeSubmitting}>
                  {t('probe.form.submit')}
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
                { title: t('probe.columns.ifaceProto'), render: (_, r) => `${r.iface}/${r.proto}` },
                { title: t('probe.columns.params'), dataIndex: 'params', render: (p) => (p ? JSON.stringify(p) : '-') },
                {
                  title: t('probe.columns.status'),
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
                  title: t('probe.columns.actions'),
                  width: 80,
                  render: (_, r) => (
                    <Popconfirm title={t('probe.deleteConfirm')} onConfirm={() => onRemoveProbe(r.pid)}>
                      <a>{t('common:delete')}</a>
                    </Popconfirm>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
        <Col span={10}>
          <Card title={t('alert.title')}>
            <Form form={ruleForm} layout="inline" onFinish={onCreateRule} style={{ marginBottom: 16, rowGap: 8 }}>
              <Form.Item name="field" rules={[{ required: true, message: t('alert.form.fieldRequired') }]}>
                <Input placeholder={t('alert.form.fieldPlaceholder')} style={{ width: 140 }} />
              </Form.Item>
              <Form.Item name="op" rules={[{ required: true, message: t('alert.form.opRequired') }]} initialValue=">">
                <Select
                  style={{ width: 80 }}
                  options={['>', '>=', '<', '<=', '=='].map((op) => ({ value: op, label: op }))}
                />
              </Form.Item>
              <Form.Item name="threshold" rules={[{ required: true, message: t('alert.form.thresholdRequired') }]}>
                <InputNumber placeholder={t('alert.form.thresholdPlaceholder')} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={ruleSubmitting}>
                  {t('common:add')}
                </Button>
              </Form.Item>
            </Form>
            <Table
              rowKey="id"
              size="small"
              dataSource={alertRules}
              pagination={false}
              columns={[
                { title: t('alert.columns.field'), dataIndex: 'field' },
                { title: t('alert.columns.condition'), render: (_, r) => `${r.op} ${r.threshold}` },
                {
                  title: t('alert.columns.actions'),
                  width: 80,
                  render: (_, r) => (
                    <Popconfirm title={t('alert.deleteConfirm')} onConfirm={() => onDeleteRule(r.id)}>
                      <a>{t('common:delete')}</a>
                    </Popconfirm>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={24}>
          <Card title={t('ota.title')}>
            <Space wrap style={{ marginBottom: 16 }}>
              <Select
                style={{ width: 260 }}
                placeholder={t('ota.selectPlaceholder')}
                value={selectedFirmwareId}
                onChange={setSelectedFirmwareId}
                options={firmwareList
                  .filter((f) => !device || f.pid === device.pid)
                  .map((f) => ({ value: f.id, label: `${f.version}（${f.filename}）` }))}
              />
              <Button type="primary" icon={<CloudUploadOutlined />} loading={otaSubmitting} onClick={onDispatchOta}>
                {t('ota.dispatchButton')}
              </Button>
              {ota && !['success', 'failed', 'rolled_back'].includes(ota.state) && (
                <Button danger icon={<StopOutlined />} onClick={onCancelOta}>
                  {t('ota.cancelButton')}
                </Button>
              )}
            </Space>
            {ota ? (
              <div>
                <Space>
                  <span>{t('ota.targetVersionLabel', { version: ota.version })}</span>
                  <Tag color={otaStateTag[ota.state].color}>{otaStateTag[ota.state].text}</Tag>
                </Space>
                {['downloading', 'flashing'].includes(ota.state) && (
                  <Progress percent={ota.progress} style={{ maxWidth: 320, marginTop: 8 }} />
                )}
                {ota.last_error && (
                  <div>
                    <Typography.Text type="danger" style={{ fontSize: 12 }}>
                      {ota.last_error}
                    </Typography.Text>
                  </div>
                )}
              </div>
            ) : (
              <Typography.Text type="secondary">{t('ota.noTask')}</Typography.Text>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
