import { useEffect, useState } from 'react'
import { Button, Card, DatePicker, Select, Space, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs, { Dayjs } from 'dayjs'
import { useTranslation } from 'react-i18next'
import { getDeviceRecords, listDevices, type Device, type DeviceRecord } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'

export default function MonitorPage() {
  const { t } = useTranslation('monitor')
  const [devices, setDevices] = useState<Device[]>([])
  const [deviceId, setDeviceId] = useState<number | null>(null)
  const [range, setRange] = useState<[Dayjs, Dayjs] | null>([dayjs().subtract(1, 'day'), dayjs()])
  const [records, setRecords] = useState<DeviceRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    listDevices(1, 200)
      .then((res) => {
        setDevices(res.list)
        if (res.list.length > 0) setDeviceId(res.list[0].id)
      })
      .catch((e) => message.error(apiErrorMessage(e, t('messages.loadDevicesFailed'))))
  }, [])

  const query = async (p = 1) => {
    if (!deviceId) return
    setLoading(true)
    try {
      const res = await getDeviceRecords(deviceId, {
        start: range?.[0]?.unix(),
        end: range?.[1]?.unix(),
        page: p,
        pageSize: 50,
      })
      setRecords(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(apiErrorMessage(e, t('messages.queryFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (deviceId) query(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceId])

  const fields = Array.from(new Set(records.flatMap((r) => Object.keys(r.d)))).sort()

  const columns: ColumnsType<DeviceRecord> = [
    { title: t('columns.time'), dataIndex: 'ts', width: 200, render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    ...fields.map((f) => ({
      title: f,
      key: f,
      render: (_: unknown, r: DeviceRecord) => {
        const v = r.d[f]
        return v === undefined ? '-' : typeof v === 'object' ? JSON.stringify(v) : String(v)
      },
    })),
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('title')}
      </Typography.Title>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            style={{ width: 240 }}
            placeholder={t('devicePlaceholder')}
            value={deviceId}
            onChange={setDeviceId}
            options={devices.map((d) => ({ value: d.id, label: d.name || d.sn }))}
            showSearch
            filterOption={(input, opt) => (opt?.label as string).toLowerCase().includes(input.toLowerCase())}
          />
          <DatePicker.RangePicker
            showTime
            value={range}
            onChange={(v) => setRange(v as [Dayjs, Dayjs] | null)}
          />
          <Button type="primary" onClick={() => query(1)} loading={loading}>
            {t('queryButton')}
          </Button>
        </Space>
        <Table
          rowKey="ts"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={records}
          pagination={{ current: page, total, pageSize: 50, onChange: query }}
          scroll={{ x: true }}
        />
      </Card>
    </div>
  )
}
