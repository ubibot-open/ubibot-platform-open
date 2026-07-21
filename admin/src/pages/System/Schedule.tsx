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
import {
  createScheduledTask,
  deleteScheduledTask,
  listScheduledTasks,
  updateScheduledTask,
  type ScheduledTask,
} from '../../api/schedule'
import { listDevices, type Device } from '../../api/device'
import { ApiError } from '../../api/client'

export default function SchedulePage() {
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
      message.error(e instanceof ApiError ? e.message : '加载定时任务失败')
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
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteScheduledTask(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  const deviceName = (id: number) => (id === 0 ? '全部启用设备' : devices.find((d) => d.id === id)?.name || `#${id}`)

  const columns: ColumnsType<ScheduledTask> = [
    { title: '名称', dataIndex: 'name' },
    { title: '目标设备', render: (_, r) => deviceName(r.device_id) },
    { title: '指令类型', dataIndex: 'cmd_type' },
    {
      title: '调度',
      render: (_, r) =>
        r.schedule_type === 'interval'
          ? `每 ${r.interval_seconds} 秒`
          : `每天 ${String(Math.floor((r.daily_at_minute ?? 0) / 60)).padStart(2, '0')}:${String((r.daily_at_minute ?? 0) % 60).padStart(2, '0')}`,
    },
    { title: '状态', dataIndex: 'enabled', render: (v: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>) },
    { title: '下次执行', dataIndex: 'next_run_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>编辑</a>
          <Popconfirm title="确认删除该任务？" onConfirm={() => onDelete(r.id)}>
            <a>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          定时任务
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建任务
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title={editing ? '编辑任务' : '新建任务'} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label="任务名称" rules={[{ required: true }]}>
            <Input placeholder="如：每晚重启" />
          </Form.Item>
          <Form.Item name="device_id" label="目标设备" rules={[{ required: true }]}>
            <Select
              options={[{ value: 0, label: '全部启用设备' }, ...devices.map((d) => ({ value: d.id, label: d.name || d.sn }))]}
            />
          </Form.Item>
          <Form.Item name="cmd_type" label="指令类型" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'reboot', label: 'reboot（重启）' },
                { value: 'calibrate', label: 'calibrate（校准）' },
              ]}
              showSearch
            />
          </Form.Item>
          <Form.Item name="schedule_type" label="调度方式" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'interval', label: '固定间隔' },
                { value: 'daily', label: '每天固定时刻' },
              ]}
              onChange={setScheduleType}
            />
          </Form.Item>
          {scheduleType === 'interval' ? (
            <Form.Item name="interval_seconds" label="间隔秒数" rules={[{ required: true }]}>
              <InputNumber min={60} style={{ width: '100%' }} placeholder="至少60秒" />
            </Form.Item>
          ) : (
            <Form.Item name="daily_at" label="每天执行时刻" rules={[{ required: true }]}>
              <TimePicker format="HH:mm" style={{ width: '100%' }} />
            </Form.Item>
          )}
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
