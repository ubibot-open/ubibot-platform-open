import { useEffect, useState } from 'react'
import { Button, Card, Select, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { listAlertEvents, resolveAlertEvent, type AlertEvent } from '../../api/alert'
import { listDevices, type Device } from '../../api/device'
import { ApiError } from '../../api/client'

const typeTag: Record<AlertEvent['type'], { color: string; text: string }> = {
  threshold: { color: 'orange', text: '阈值告警' },
  offline: { color: 'red', text: '离线告警' },
}

export default function AlertPage() {
  const [devices, setDevices] = useState<Device[]>([])
  const [deviceId, setDeviceId] = useState<number | undefined>(undefined)
  const [status, setStatus] = useState<string>('open')
  const [events, setEvents] = useState<AlertEvent[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    listDevices(1, 200)
      .then((res) => setDevices(res.list))
      .catch((e) => message.error(e instanceof ApiError ? e.message : '加载设备列表失败'))
  }, [])

  const load = async (p = 1) => {
    setLoading(true)
    try {
      const res = await listAlertEvents({ deviceId, status: status || undefined, page: p, pageSize: 20 })
      setEvents(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载告警列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceId, status])

  const onResolve = async (id: number) => {
    try {
      await resolveAlertEvent(id)
      message.success('已标记为已处理')
      load(page)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '操作失败')
    }
  }

  const columns: ColumnsType<AlertEvent> = [
    { title: '设备', dataIndex: 'device_name' },
    { title: '类型', dataIndex: 'type', width: 100, render: (t: AlertEvent['type']) => (
      <Tag color={typeTag[t].color}>{typeTag[t].text}</Tag>
    ) },
    { title: '详情', dataIndex: 'message' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: AlertEvent['status']) =>
        s === 'open' ? <Tag color="red">进行中</Tag> : <Tag color="green">已处理</Tag>,
    },
    { title: '触发时间', dataIndex: 'triggered_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 90,
      render: (_, r) =>
        r.status === 'open' ? <Button size="small" onClick={() => onResolve(r.id)}>标记处理</Button> : '-',
    },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        告警中心
      </Typography.Title>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            style={{ width: 200 }}
            placeholder="按设备筛选"
            allowClear
            value={deviceId}
            onChange={setDeviceId}
            options={devices.map((d) => ({ value: d.id, label: d.name || d.sn }))}
            showSearch
            filterOption={(input, opt) => (opt?.label as string).toLowerCase().includes(input.toLowerCase())}
          />
          <Select
            style={{ width: 140 }}
            value={status}
            onChange={setStatus}
            options={[
              { value: 'open', label: '进行中' },
              { value: 'resolved', label: '已处理' },
              { value: '', label: '全部' },
            ]}
          />
        </Space>
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={events}
          pagination={{ current: page, total, pageSize: 20, onChange: load }}
        />
      </Card>
    </div>
  )
}
