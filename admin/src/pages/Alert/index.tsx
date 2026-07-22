import { useEffect, useState } from 'react'
import { Button, Card, Select, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { listAlertEvents, resolveAlertEvent, type AlertEvent } from '../../api/alert'
import { listDevices, type Device } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

const typeColor: Record<AlertEvent['type'], string> = {
  threshold: 'orange',
  offline: 'red',
}

export default function AlertPage() {
  const { t } = useTranslation('alert')
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
      .catch((e) => message.error(apiErrorMessage(e, t('messages.loadDevicesFailed'))))
  }, [])

  const load = async (p = 1) => {
    setLoading(true)
    try {
      const res = await listAlertEvents({ deviceId, status: status || undefined, page: p, pageSize: 20 })
      setEvents(res.list)
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

  const onResolve = async (id: number) => {
    try {
      await resolveAlertEvent(id)
      message.success(t('messages.resolveSuccess'))
      load(page)
    } catch (e) {
      message.error(apiErrorMessage(e, t('messages.resolveFailed')))
    }
  }

  const columns: ColumnsType<AlertEvent> = [
    { title: t('columns.device'), dataIndex: 'device_name' },
    { title: t('columns.type'), dataIndex: 'type', width: 100, render: (tp: AlertEvent['type']) => (
      <Tag color={typeColor[tp]}>{t(`type.${tp}`)}</Tag>
    ) },
    { title: t('columns.detail'), dataIndex: 'message' },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      width: 100,
      render: (s: AlertEvent['status']) =>
        s === 'open' ? <Tag color="red">{t('status.open')}</Tag> : <Tag color="green">{t('status.resolved')}</Tag>,
    },
    { title: t('columns.triggeredAt'), dataIndex: 'triggered_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: t('columns.actions'),
      width: 90,
      render: (_, r) =>
        r.status === 'open' ? <Button size="small" onClick={() => onResolve(r.id)}>{t('resolveButton')}</Button> : '-',
    },
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
            value={status}
            onChange={setStatus}
            options={[
              { value: 'open', label: t('status.open') },
              { value: 'resolved', label: t('status.resolved') },
              { value: '', label: t('status.all') },
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
