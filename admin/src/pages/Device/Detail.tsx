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
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ArrowLeftOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { getDevice, renameDevice, type Device, type DeviceRecord } from '../../api/device'
import { listAlertRules, createAlertRule, deleteAlertRule, type AlertRule } from '../../api/alert'
import { apiErrorMessage } from '../../api/errors'

export default function DeviceDetailPage() {
  const { t } = useTranslation('deviceDetail')
  const { id } = useParams()
  const navigate = useNavigate()
  const deviceId = Number(id)

  function formatTime(ts: number | null) {
    if (!ts) return t('neverReported')
    return new Date(ts * 1000).toLocaleString()
  }

  const [device, setDevice] = useState<Device | null>(null)
  const [records, setRecords] = useState<DeviceRecord[]>([])
  const [loading, setLoading] = useState(false)

  const [renameOpen, setRenameOpen] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [renameForm] = Form.useForm()

  const [alertRules, setAlertRules] = useState<AlertRule[]>([])
  const [ruleSubmitting, setRuleSubmitting] = useState(false)
  const [ruleForm] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getDevice(deviceId)
      setDevice(res.device)
      setRecords(res.records)
    } catch (e) {
      message.error(apiErrorMessage(e, t('basic.loadFailed')))
    } finally {
      setLoading(false)
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

  useEffect(() => {
    load()
    loadAlertRules()
  }, [load, loadAlertRules])

  const openRename = () => {
    renameForm.setFieldsValue({ name: device?.name })
    setRenameOpen(true)
  }

  const onRename = async (values: { name: string }) => {
    setRenaming(true)
    try {
      await renameDevice(deviceId, values.name)
      message.success(t('basic.renameSuccess'))
      setRenameOpen(false)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('basic.renameFailed')))
    } finally {
      setRenaming(false)
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

  const recordColumns: ColumnsType<DeviceRecord> = [
    { title: t('records.columns.time'), dataIndex: 'ts', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    { title: t('records.columns.data'), dataIndex: 'd', render: (d: Record<string, unknown>) => JSON.stringify(d) },
  ]

  return (
    <div>
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/device')} style={{ paddingLeft: 0 }}>
        {t('backToList')}
      </Button>

      {device && (
        <Card style={{ marginBottom: 16 }}>
          <Descriptions
            title={
              <Space>
                {device.name || device.sn}
                <a onClick={openRename}>
                  <EditOutlined /> {t('basic.renameButton')}
                </a>
              </Space>
            }
            column={3}
          >
            <Descriptions.Item label="SN">{device.sn}</Descriptions.Item>
            <Descriptions.Item label="PID">{device.pid}</Descriptions.Item>
            <Descriptions.Item label={t('basic.statusLabel')}>
              {device.status === 1 ? (
                <Tag color="success">{t('common:enabled')}</Tag>
              ) : (
                <Tag>{t('common:disabled')}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t('basic.onlineLabel')}>
              {device.online ? (
                <Tag color="green">{t('basic.online.yes')}</Tag>
              ) : (
                <Tag color="default">{t('basic.online.no')}</Tag>
              )}
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

      <Modal
        title={t('basic.renameButton')}
        open={renameOpen}
        onCancel={() => setRenameOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Form form={renameForm} layout="vertical" onFinish={onRename}>
          <Form.Item name="name" label={t('common:name')} rules={[{ required: true }]}>
            <Input placeholder={device?.sn} />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setRenameOpen(false)}>{t('common:cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={renaming}>
                {t('common:save')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
