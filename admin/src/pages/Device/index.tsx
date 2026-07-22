import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  DeviceSource,
  DeviceStatus,
  approveDevice,
  createDevice,
  deleteDevice,
  listDevices,
  setDeviceStatus,
  type Device,
} from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

export default function DevicePage() {
  const { t } = useTranslation('device')
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [devices, setDevices] = useState<Device[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [approvedSecret, setApprovedSecret] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [form] = Form.useForm()

  function formatTime(ts: number | null) {
    if (!ts) return t('neverReported')
    return new Date(ts * 1000).toLocaleString()
  }

  const load = async (p = page) => {
    setLoading(true)
    try {
      const res = await listDevices(p, 20)
      setDevices(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(apiErrorMessage(e, t('loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onCreate = async (values: { pid: string; sn: string; name?: string; secret?: string }) => {
    setCreating(true)
    try {
      const dev = await createDevice(values)
      message.success(t('create.success'))
      setCreatedSecret(dev.secret ?? null)
      form.resetFields()
      load(1)
    } catch (e) {
      message.error(apiErrorMessage(e, t('create.failed')))
    } finally {
      setCreating(false)
    }
  }

  const onApprove = async (id: number) => {
    setBusyId(id)
    try {
      const dev = await approveDevice(id)
      message.success(t('approveSuccess'))
      setApprovedSecret(dev.secret ?? null)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('approveFailed')))
    } finally {
      setBusyId(null)
    }
  }

  const onReject = async (id: number) => {
    setBusyId(id)
    try {
      await setDeviceStatus(id, DeviceStatus.Disabled)
      message.success(t('rejectSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('rejectFailed')))
    } finally {
      setBusyId(null)
    }
  }

  const onDisable = async (id: number) => {
    setBusyId(id)
    try {
      await setDeviceStatus(id, DeviceStatus.Disabled)
      message.success(t('disableSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('disableFailed')))
    } finally {
      setBusyId(null)
    }
  }

  const onEnable = async (id: number) => {
    setBusyId(id)
    try {
      await setDeviceStatus(id, DeviceStatus.Enabled)
      message.success(t('enableSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('enableFailed')))
    } finally {
      setBusyId(null)
    }
  }

  const onDelete = async (id: number) => {
    setBusyId(id)
    try {
      await deleteDevice(id)
      message.success(t('deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('deleteFailed')))
    } finally {
      setBusyId(null)
    }
  }

  const columns: ColumnsType<Device> = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: t('columns.name'), dataIndex: 'name', render: (v: string, r) => v || r.sn },
    { title: 'SN', dataIndex: 'sn' },
    { title: 'PID', dataIndex: 'pid' },
    {
      title: t('source.title'),
      dataIndex: 'source',
      width: 100,
      render: (source: Device['source']) =>
        source === DeviceSource.SelfRegistered ? (
          <Tag color="purple">{t('source.selfRegistered')}</Tag>
        ) : (
          <Tag color="blue">{t('source.manual')}</Tag>
        ),
    },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      width: 100,
      render: (status: number) => {
        if (status === DeviceStatus.Pending) return <Tag color="gold">{t('status.pending')}</Tag>
        return status === DeviceStatus.Enabled ? (
          <Tag color="success">{t('common:enabled')}</Tag>
        ) : (
          <Tag color="default">{t('common:disabled')}</Tag>
        )
      },
    },
    {
      title: t('columns.activated'),
      dataIndex: 'activated',
      width: 90,
      render: (activated: boolean) =>
        activated ? (
          <Tag color="blue">{t('activated.yes')}</Tag>
        ) : (
          <Tag color="default">{t('activated.no')}</Tag>
        ),
    },
    {
      title: t('columns.online'),
      dataIndex: 'online',
      width: 80,
      render: (online: boolean) =>
        online ? <Tag color="green">{t('online.yes')}</Tag> : <Tag color="default">{t('online.no')}</Tag>,
    },
    { title: t('columns.interval'), render: (_, r) => `${r.ci} / ${r.ui}` },
    { title: t('columns.lastSeen'), dataIndex: 'last_seen_at', render: formatTime },
    {
      title: t('columns.actions'),
      width: 220,
      render: (_, r) => (
        <Space size="small" wrap>
          <a onClick={() => navigate(`/device/${r.id}`)}>{t('detail')}</a>
          {r.status === DeviceStatus.Pending && (
            <>
              <Popconfirm title={t('approveConfirmTitle')} onConfirm={() => onApprove(r.id)}>
                <a>{t('approveButton')}</a>
              </Popconfirm>
              <Popconfirm title={t('rejectConfirmTitle')} onConfirm={() => onReject(r.id)}>
                <a style={{ color: '#ff4d4f' }}>{t('rejectButton')}</a>
              </Popconfirm>
            </>
          )}
          {r.status === DeviceStatus.Enabled && (
            <Popconfirm title={t('disableConfirmTitle')} onConfirm={() => onDisable(r.id)}>
              <a>{t('disableButton')}</a>
            </Popconfirm>
          )}
          {r.status === DeviceStatus.Disabled && <a onClick={() => onEnable(r.id)}>{t('enableButton')}</a>}
          <Popconfirm
            title={t('deleteConfirmTitle')}
            description={t('deleteConfirmContent')}
            onConfirm={() => onDelete(r.id)}
          >
            <a style={{ color: '#ff4d4f' }}>{t('common:delete')}</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('title')}
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setCreatedSecret(null)
            setCreateOpen(true)
          }}
        >
          {t('create.button')}
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={devices}
        loading={loading || busyId !== null}
        pagination={{ current: page, total, pageSize: 20, onChange: load }}
      />

      <Modal
        title={t('create.modalTitle')}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        footer={null}
        destroyOnClose
      >
        {createdSecret ? (
          <div>
            <Typography.Paragraph>{t('create.createdNotice')}</Typography.Paragraph>
            <Typography.Text code copyable style={{ fontSize: 14 }}>
              {createdSecret}
            </Typography.Text>
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Button type="primary" onClick={() => setCreateOpen(false)}>
                {t('create.gotIt')}
              </Button>
            </div>
          </div>
        ) : (
          <Form form={form} layout="vertical" onFinish={onCreate}>
            <Form.Item name="pid" label={t('create.pidLabel')} rules={[{ required: true }]}>
              <Input placeholder={t('create.pidPlaceholder')} />
            </Form.Item>
            <Form.Item name="sn" label={t('create.snLabel')} rules={[{ required: true }]}>
              <Input placeholder={t('create.snPlaceholder')} />
            </Form.Item>
            <Form.Item name="name" label={t('create.nameLabel')}>
              <Input placeholder={t('create.namePlaceholder')} />
            </Form.Item>
            <Form.Item name="secret" label={t('create.secretLabel')} extra={t('create.secretExtra')}>
              <Input placeholder={t('create.secretPlaceholder')} />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
              <Space>
                <Button onClick={() => setCreateOpen(false)}>{t('common:cancel')}</Button>
                <Button type="primary" htmlType="submit" loading={creating}>
                  {t('create.submit')}
                </Button>
              </Space>
            </Form.Item>
          </Form>
        )}
      </Modal>

      <Modal
        title={t('approveConfirmTitle')}
        open={approvedSecret !== null}
        onCancel={() => setApprovedSecret(null)}
        footer={
          <Button type="primary" onClick={() => setApprovedSecret(null)}>
            {t('create.gotIt')}
          </Button>
        }
        destroyOnClose
      >
        <Typography.Paragraph>{t('approvedNotice')}</Typography.Paragraph>
        <Typography.Text code copyable style={{ fontSize: 14 }}>
          {approvedSecret}
        </Typography.Text>
      </Modal>
    </div>
  )
}
