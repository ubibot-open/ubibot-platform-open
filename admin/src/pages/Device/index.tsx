import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { DeviceStatus, deleteDevice, listDevices, renameDevice, setDeviceStatus, type Device } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

export default function DevicePage() {
  const { t } = useTranslation('device')
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [devices, setDevices] = useState<Device[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [renameTarget, setRenameTarget] = useState<Device | null>(null)
  const [renaming, setRenaming] = useState(false)
  const [renameForm] = Form.useForm()

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

  const openRename = (device: Device) => {
    setRenameTarget(device)
    renameForm.setFieldsValue({ name: device.name })
  }

  const onRename = async (values: { name: string }) => {
    if (!renameTarget) return
    setRenaming(true)
    try {
      await renameDevice(renameTarget.id, values.name)
      message.success(t('renameSuccess'))
      setRenameTarget(null)
      renameForm.resetFields()
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('renameFailed')))
    } finally {
      setRenaming(false)
    }
  }

  const columns: ColumnsType<Device> = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: t('columns.name'), dataIndex: 'name', render: (v: string, r) => v || r.sn },
    { title: 'SN', dataIndex: 'sn' },
    { title: 'PID', dataIndex: 'pid' },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      width: 100,
      render: (status: number) =>
        status === DeviceStatus.Enabled ? (
          <Tag color="success">{t('common:enabled')}</Tag>
        ) : (
          <Tag color="default">{t('common:disabled')}</Tag>
        ),
    },
    {
      title: t('columns.online'),
      dataIndex: 'online',
      width: 80,
      render: (online: boolean) =>
        online ? <Tag color="green">{t('online.yes')}</Tag> : <Tag color="default">{t('online.no')}</Tag>,
    },
    { title: t('columns.lastSeen'), dataIndex: 'last_seen_at', render: formatTime },
    {
      title: t('columns.actions'),
      width: 220,
      render: (_, r) => (
        <Space size="small" wrap>
          <a onClick={() => navigate(`/device/${r.id}`)}>{t('detail')}</a>
          <a onClick={() => openRename(r)}>{t('renameButton')}</a>
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
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={devices}
        loading={loading || busyId !== null}
        pagination={{ current: page, total, pageSize: 20, onChange: load }}
      />

      <Modal
        title={t('renameButton')}
        open={renameTarget !== null}
        onCancel={() => setRenameTarget(null)}
        footer={null}
        destroyOnClose
      >
        <Form form={renameForm} layout="vertical" onFinish={onRename}>
          <Form.Item name="name" label={t('columns.name')} rules={[{ required: true }]}>
            <Input placeholder={renameTarget?.sn} />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setRenameTarget(null)}>{t('common:cancel')}</Button>
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
