import { useEffect, useState } from 'react'
import { Card, Select, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { listAllCommands, type GlobalCommand } from '../../api/command'
import { listDevices, type Device } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

const statusColor: Record<GlobalCommand['status'], string> = {
  pending: 'processing',
  acked: 'success',
  nacked: 'error',
}

export default function CommandPage() {
  const { t } = useTranslation('command')
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
      .catch((e) => message.error(apiErrorMessage(e, t('messages.loadDevicesFailed'))))
  }, [])

  const load = async (p = 1) => {
    setLoading(true)
    try {
      const res = await listAllCommands({ deviceId, status, page: p, pageSize: 20 })
      setCommands(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(apiErrorMessage(e, t('messages.loadListFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceId, status])

  const columns: ColumnsType<GlobalCommand> = [
    { title: t('columns.id'), dataIndex: 'id', width: 90 },
    { title: t('columns.device'), dataIndex: 'device_name' },
    { title: t('columns.type'), dataIndex: 'type' },
    { title: t('columns.args'), dataIndex: 'args', render: (a?: Record<string, unknown>) => (a ? JSON.stringify(a) : '-') },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      width: 100,
      render: (s: GlobalCommand['status'], r) => (
        <Space direction="vertical" size={0}>
          <Tag color={statusColor[s]}>{t(`status.${s}`)}</Tag>
          {r.nak_message && <span style={{ fontSize: 12, color: '#cf1322' }}>{r.nak_message}</span>}
        </Space>
      ),
    },
    { title: t('columns.createdAt'), dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('title')}
      </Typography.Title>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            style={{ width: 200 }}
            placeholder={t('filters.devicePlaceholder')}
            allowClear
            value={deviceId}
            onChange={setDeviceId}
            options={devices.map((d) => ({ value: d.id, label: d.name || d.sn }))}
            showSearch
            filterOption={(input, opt) => (opt?.label as string).toLowerCase().includes(input.toLowerCase())}
          />
          <Select
            style={{ width: 140 }}
            placeholder={t('filters.statusPlaceholder')}
            allowClear
            value={status}
            onChange={setStatus}
            options={[
              { value: 'pending', label: t('status.pending') },
              { value: 'acked', label: t('status.acked') },
              { value: 'nacked', label: t('status.nacked') },
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
