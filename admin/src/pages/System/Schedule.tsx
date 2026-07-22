import { useEffect, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  TimePicker,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  createScheduledTask,
  deleteScheduledTask,
  listScheduledTasks,
  updateScheduledTask,
  type ScheduledTask,
} from '../../api/schedule'
import { listDevices, type Device } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

export default function SchedulePage() {
  const { t } = useTranslation('systemSchedule')
  const [rows, setRows] = useState<ScheduledTask[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ScheduledTask | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [scheduleType, setScheduleType] = useState<'interval' | 'daily'>('interval')
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const [t, d] = await Promise.all([listScheduledTasks(), listDevices(1, 200)])
      setRows(t.list)
      setDevices(d.list)
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const openCreate = () => {
    setEditing(null)
    setScheduleType('interval')
    form.resetFields()
    form.setFieldsValue({ device_id: 0, schedule_type: 'interval', enabled: true })
    setOpen(true)
  }

  const openEdit = (t: ScheduledTask) => {
    setEditing(t)
    setScheduleType(t.schedule_type)
    form.setFieldsValue({
      name: t.name,
      device_id: t.device_id,
      cmd_type: t.cmd_type,
      schedule_type: t.schedule_type,
      interval_seconds: t.interval_seconds,
      daily_at: t.daily_at_minute != null ? dayjs().startOf('day').add(t.daily_at_minute, 'minute') : undefined,
      enabled: t.enabled,
    })
    setOpen(true)
  }

  const onSubmit = async (values: {
    name: string
    device_id: number
    cmd_type: string
    schedule_type: 'interval' | 'daily'
    interval_seconds?: number
    daily_at?: dayjs.Dayjs
    enabled: boolean
  }) => {
    setSubmitting(true)
    try {
      const input = {
        name: values.name,
        device_id: values.device_id,
        cmd_type: values.cmd_type,
        schedule_type: values.schedule_type,
        interval_seconds: values.schedule_type === 'interval' ? values.interval_seconds : undefined,
        daily_at_minute:
          values.schedule_type === 'daily' && values.daily_at
            ? values.daily_at.hour() * 60 + values.daily_at.minute()
            : undefined,
        enabled: values.enabled,
      }
      if (editing) {
        await updateScheduledTask(editing.id, input)
      } else {
        await createScheduledTask(input)
      }
      message.success(t('common:saveSuccess'))
      setOpen(false)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:saveFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteScheduledTask(id)
      message.success(t('common:deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:deleteFailed')))
    }
  }

  const deviceName = (id: number) => (id === 0 ? t('allEnabledDevices') : devices.find((d) => d.id === id)?.name || `#${id}`)

  const columns: ColumnsType<ScheduledTask> = [
    { title: t('common:name'), dataIndex: 'name' },
    { title: t('table.targetDevice'), render: (_, r) => deviceName(r.device_id) },
    { title: t('table.cmdType'), dataIndex: 'cmd_type' },
    {
      title: t('table.schedule'),
      render: (_, r) =>
        r.schedule_type === 'interval'
          ? t('table.intervalDisplay', { seconds: r.interval_seconds })
          : t('table.dailyDisplay', {
              time: `${String(Math.floor((r.daily_at_minute ?? 0) / 60)).padStart(2, '0')}:${String((r.daily_at_minute ?? 0) % 60).padStart(2, '0')}`,
            }),
    },
    {
      title: t('common:status'),
      dataIndex: 'enabled',
      render: (v: boolean) => (v ? <Tag color="green">{t('common:enabled')}</Tag> : <Tag>{t('common:disabled')}</Tag>),
    },
    { title: t('table.nextRunAt'), dataIndex: 'next_run_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: t('common:actions'),
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>{t('common:edit')}</a>
          <Popconfirm title={t('deleteConfirm')} onConfirm={() => onDelete(r.id)}>
            <a>{t('common:delete')}</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('pageTitle')}
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          {t('createButton')}
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal
        title={editing ? t('modal.editTitle') : t('modal.createTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('modal.nameLabel')} rules={[{ required: true }]}>
            <Input placeholder={t('modal.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="device_id" label={t('table.targetDevice')} rules={[{ required: true }]}>
            <Select
              options={[{ value: 0, label: t('allEnabledDevices') }, ...devices.map((d) => ({ value: d.id, label: d.name || d.sn }))]}
            />
          </Form.Item>
          <Form.Item name="cmd_type" label={t('table.cmdType')} rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'reboot', label: `reboot（${t('cmdType.reboot')}）` },
                { value: 'calibrate', label: `calibrate（${t('cmdType.calibrate')}）` },
              ]}
              showSearch
            />
          </Form.Item>
          <Form.Item name="schedule_type" label={t('modal.scheduleTypeLabel')} rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'interval', label: t('scheduleType.interval') },
                { value: 'daily', label: t('scheduleType.daily') },
              ]}
              onChange={setScheduleType}
            />
          </Form.Item>
          {scheduleType === 'interval' ? (
            <Form.Item name="interval_seconds" label={t('modal.intervalSecondsLabel')} rules={[{ required: true }]}>
              <InputNumber min={60} style={{ width: '100%' }} placeholder={t('modal.intervalSecondsPlaceholder')} />
            </Form.Item>
          ) : (
            <Form.Item name="daily_at" label={t('modal.dailyAtLabel')} rules={[{ required: true }]}>
              <TimePicker format="HH:mm" style={{ width: '100%' }} />
            </Form.Item>
          )}
          <Form.Item name="enabled" label={t('common:enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>{t('common:cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {t('common:save')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
