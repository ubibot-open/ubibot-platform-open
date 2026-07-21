import { useEffect, useState } from 'react'
import { Card, Select, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { listAllCommands, type GlobalCommand } from '../../api/command'
import { listDevices, type Device } from '../../api/device'
import { ApiError } from '../../api/client'

const statusTag: Record<GlobalCommand['status'], { color: string; text: string }> = {
  pending: { color: 'processing', text: '待确认' },
  acked: { color: 'success', text: '已确认' },
  nacked: { color: 'error', text: '执行失败' },
}

export default function CommandPage() {
  const [devices, setDevices] = useState<Device[]>([])
  const [deviceId, setDeviceId] = useState<number | undefined>(undefined)
  const [status, setStatus] = useState<string | undefined>(undefined)
  const [commands, setCommands] = useState<GlobalCommand[]>([])
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
      const res = await listAllCommands({ deviceId, status, page: p, pageSize: 20 })
      setCommands(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载指令历史失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceId, status])

  const columns: ColumnsType<GlobalCommand> = [
    { title: '指令ID', dataIndex: 'id', width: 90 },
    { title: '设备', dataIndex: 'device_name' },
    { title: '类型', dataIndex: 'type' },
    { title: '参数', dataIndex: 'args', render: (a?: Record<string, unknown>) => (a ? JSON.stringify(a) : '-') },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: GlobalCommand['status'], r) => (
        <Space direction="vertical" size={0}>
          <Tag color={statusTag[s].color}>{statusTag[s].text}</Tag>
          {r.nak_message && <span style={{ fontSize: 12, color: '#cf1322' }}>{r.nak_message}</span>}
        </Space>
      ),
    },
    { title: '下发时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        指令管理
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
            placeholder="按状态筛选"
            allowClear
            value={status}
            onChange={setStatus}
            options={[
              { value: 'pending', label: '待确认' },
              { value: 'acked', label: '已确认' },
              { value: 'nacked', label: '执行失败' },
            ]}
          />
        </Space>
        <Table
          rowKey={(r) => `${r.device_id}-${r.id}`}
          size="small"
          loading={loading}
          columns={columns}
          dataSource={commands}
          pagination={{ current: page, total, pageSize: 20, onChange: load }}
        />
      </Card>
    </div>
  )
}
