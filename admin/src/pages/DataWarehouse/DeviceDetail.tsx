import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Avatar,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Empty,
  Row,
  Segmented,
  Select,
  Space,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd'
import {
  AppstoreOutlined,
  ArrowLeftOutlined,
  DownloadOutlined,
  HddOutlined,
  LeftOutlined,
  RightOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip as RTooltip,
  XAxis,
  YAxis,
} from 'recharts'
import {
  getDevice,
  getDeviceRecords,
  listDataWarehouse,
  type Device,
  type DeviceRecord,
} from '../../api/device'
import { listAlertEvents, type AlertEvent } from '../../api/alert'
import { apiErrorMessage } from '../../api/errors'
import { useFieldIcons } from '../../hooks/useFieldIcons'
import { formatFieldValue } from '../../utils/sensorValue'
import { toCsv, downloadCsv } from '../../utils/csv'
import RelativeTime from '../../components/RelativeTime'

const HISTORY_PAGE_SIZE = 500

function numericFields(records: DeviceRecord[]): string[] {
  const fields = new Set<string>()
  for (const r of records) {
    for (const [k, v] of Object.entries(r.d)) {
      if (typeof v === 'number') fields.add(k)
    }
  }
  return Array.from(fields).sort()
}

// One line chart for a single field, X axis = time, with a dashed
// reference line at the range's average -- a simplified stand-in for the
// commercial dashboard's high/low markers, computed straight from
// whatever's currently loaded rather than a separate aggregation query.
function FieldChart({ field, records, color, label }: { field: string; records: DeviceRecord[]; color: string; label: string }) {
  const data = records
    .map((r) => ({ ts: r.ts, value: r.d[field] }))
    .filter((d): d is { ts: number; value: number } => typeof d.value === 'number')
  const avg = data.length ? data.reduce((s, d) => s + d.value, 0) / data.length : null

  return (
    <Card size="small" title={label}>
      <div style={{ height: 220 }}>
        {data.length === 0 ? (
          <Empty style={{ marginTop: 60 }} />
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="ts"
                tickFormatter={(ts: number) => dayjs(ts * 1000).format('MM-DD HH:mm')}
                fontSize={11}
                minTickGap={50}
              />
              <YAxis fontSize={11} width={48} />
              <RTooltip
                labelFormatter={(ts: number) => new Date(ts * 1000).toLocaleString()}
                formatter={(v: number) => [v, label]}
              />
              {avg !== null && <ReferenceLine y={avg} stroke="#bfbfbf" strokeDasharray="4 4" />}
              <Line type="monotone" dataKey="value" stroke={color} dot={false} strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  )
}

export default function DataWarehouseDeviceDetailPage() {
  const { t } = useTranslation('dataWarehouseDetail')
  const { id } = useParams()
  const deviceId = Number(id)
  const navigate = useNavigate()
  const { renderFieldIcon, fieldColor } = useFieldIcons()

  const [device, setDevice] = useState<Device | null>(null)
  const [records, setRecords] = useState<DeviceRecord[]>([]) // newest-first, from getDevice
  const [loading, setLoading] = useState(false)
  const [openAlerts, setOpenAlerts] = useState<AlertEvent[]>([])
  const [siblingIds, setSiblingIds] = useState<number[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getDevice(deviceId)
      setDevice(res.device)
      setRecords(res.records)
    } catch (e) {
      message.error(apiErrorMessage(e, t('loadFailed')))
    } finally {
      setLoading(false)
    }
  }, [deviceId, t])

  useEffect(() => {
    load()
    listAlertEvents({ deviceId, status: 'open', page: 1, pageSize: 5 })
      .then((res) => setOpenAlerts(res.list))
      .catch(() => undefined)
    // Powers the </> pager below -- same ordering the 数据仓库 list uses,
    // so "12/26" here matches what the operator saw when they clicked in.
    listDataWarehouse(1, 200)
      .then((res) => setSiblingIds(res.list.map((it) => it.id)))
      .catch(() => undefined)
  }, [deviceId, load])

  const siblingIndex = siblingIds.indexOf(deviceId)
  const goSibling = (delta: number) => {
    const next = siblingIds[siblingIndex + delta]
    if (next) navigate(`/data-warehouse/${next}`)
  }

  const lastRecord = records[0] ?? null

  // --- History tab -----------------------------------------------------
  const [range, setRange] = useState<[Dayjs, Dayjs]>([dayjs().subtract(1, 'day'), dayjs()])
  const [historyRecords, setHistoryRecords] = useState<DeviceRecord[]>([])
  const [historyTotal, setHistoryTotal] = useState(0)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [selectedFields, setSelectedFields] = useState<string[]>([])
  const [chartView, setChartView] = useState<'grid' | 'list'>('grid')
  const fieldsInitialized = useRef(false)

  const queryHistory = useCallback(async () => {
    setHistoryLoading(true)
    try {
      const res = await getDeviceRecords(deviceId, {
        start: range[0].unix(),
        end: range[1].unix(),
        page: 1,
        pageSize: HISTORY_PAGE_SIZE,
      })
      setHistoryRecords(res.list)
      setHistoryTotal(res.total)
    } catch (e) {
      message.error(apiErrorMessage(e, t('history.loadFailed')))
    } finally {
      setHistoryLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceId, range[0], range[1], t])

  useEffect(() => {
    queryHistory()
  }, [queryHistory])

  const availableFields = useMemo(() => numericFields(historyRecords), [historyRecords])

  useEffect(() => {
    if (!fieldsInitialized.current && availableFields.length > 0) {
      setSelectedFields(availableFields)
      fieldsInitialized.current = true
    }
  }, [availableFields])

  const fieldLabel = (key: string) => t(`dataWarehouse:fields.${key.toLowerCase()}`, { defaultValue: key })

  const onExportHistory = () => {
    const fields = selectedFields.length > 0 ? selectedFields : availableFields
    const header = ['time', ...fields]
    const rows = historyRecords.map((r) => [
      new Date(r.ts * 1000).toISOString(),
      ...fields.map((f) => formatFieldValue(r.d[f])),
    ])
    downloadCsv(`device-${deviceId}-history-${new Date().toISOString().slice(0, 10)}.csv`, toCsv([header, ...rows]))
  }

  const alertState: 'alert' | 'offline' | 'normal' =
    openAlerts.length > 0 ? 'alert' : device && !device.online ? 'offline' : 'normal'
  const ringStyle = {
    alert: { bg: '#fff1f0', border: '#ffa39e', color: '#cf1322', label: t('status.alert') },
    offline: { bg: '#f5f5f5', border: '#d9d9d9', color: '#8c8c8c', label: t('status.offline') },
    normal: { bg: '#f6ffed', border: '#b7eb8f', color: '#389e0d', label: t('status.normal') },
  }[alertState]

  const dashboardTab = (
    <div>
      <Card style={{ marginBottom: 16 }} loading={loading}>
        <Row gutter={16} align="middle">
          <Col flex="none">
            <Avatar size={48} icon={<HddOutlined />} />
          </Col>
          <Col flex="auto">
            <Typography.Title level={4} style={{ margin: 0 }}>
              {device?.name || device?.sn}
            </Typography.Title>
            <Typography.Text type="secondary">
              {device?.sn} · {device?.pid}
            </Typography.Text>
          </Col>
          <Col flex="none">
            <div
              style={{
                width: 96,
                height: 96,
                borderRadius: '50%',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                textAlign: 'center',
                background: ringStyle.bg,
                border: `3px solid ${ringStyle.border}`,
                color: ringStyle.color,
              }}
            >
              <div style={{ fontSize: 13, fontWeight: 600 }}>{ringStyle.label}</div>
              {openAlerts.length > 0 && (
                <div style={{ fontSize: 12 }}>{t('status.openAlertsCount', { count: openAlerts.length })}</div>
              )}
            </div>
          </Col>
        </Row>

        {lastRecord && Object.keys(lastRecord.d).length > 0 && (
          <div style={{ marginTop: 20, borderTop: '1px solid #f0f0f0', paddingTop: 16 }}>
            <Space size={24} wrap>
              {Object.entries(lastRecord.d).map(([k, v]) => (
                <div key={k} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 44 }}>
                  <span style={{ color: fieldColor(k), fontSize: 20, lineHeight: 1 }}>{renderFieldIcon(k)}</span>
                  <span style={{ fontSize: 13, fontWeight: 600, marginTop: 4 }}>{formatFieldValue(v)}</span>
                  <span style={{ fontSize: 11, color: 'rgba(0,0,0,0.45)' }}>{fieldLabel(k)}</span>
                </div>
              ))}
            </Space>
          </div>
        )}
      </Card>

      <Card title={t('info.title')} style={{ marginBottom: 16 }} loading={loading}>
        <Descriptions bordered size="small" column={3}>
          <Descriptions.Item label="SN">{device?.sn}</Descriptions.Item>
          <Descriptions.Item label="PID">{device?.pid}</Descriptions.Item>
          <Descriptions.Item label={t('info.status')}>
            {device?.status === 1 ? (
              <Tag color="success">{t('common:enabled')}</Tag>
            ) : (
              <Tag>{t('common:disabled')}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={t('info.activated')}>
            {device?.activated ? (
              <Tag color="blue">{t('dataWarehouse:activated.yes')}</Tag>
            ) : (
              <Tag color="default">{t('dataWarehouse:activated.no')}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={t('info.online')}>
            {device?.online ? (
              <Tag color="green">{t('dataWarehouse:online.yes')}</Tag>
            ) : (
              <Tag color="default">{t('dataWarehouse:online.no')}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={t('info.interval')}>
            {device?.ci} / {device?.ui} {t('info.seconds')}
          </Descriptions.Item>
          <Descriptions.Item label={t('info.createdAt')}>
            {device ? new Date(device.created_at * 1000).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label={t('info.lastSeen')} span={2}>
            <RelativeTime ts={device?.last_seen_at ?? null} fallback={t('dataWarehouse:neverReported')} />
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title={t('sensors.title')} loading={loading}>
        {!lastRecord || Object.keys(lastRecord.d).length === 0 ? (
          <Empty description={t('sensors.noData')} />
        ) : (
          <Row gutter={[16, 16]}>
            {Object.entries(lastRecord.d).map(([k, v]) => (
              <Col key={k} xs={12} sm={8} md={6} lg={4}>
                <Card size="small" style={{ borderColor: fieldColor(k), textAlign: 'center' }}>
                  <div style={{ fontSize: 24, color: fieldColor(k) }}>{renderFieldIcon(k)}</div>
                  <div style={{ fontSize: 20, fontWeight: 600, marginTop: 6 }}>{formatFieldValue(v)}</div>
                  <div style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)' }}>{fieldLabel(k)}</div>
                </Card>
              </Col>
            ))}
          </Row>
        )}
      </Card>
    </div>
  )

  const historyTab = (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <DatePicker.RangePicker
          showTime
          value={range}
          onChange={(v) => v && v[0] && v[1] && setRange([v[0], v[1]])}
        />
        <Select
          mode="multiple"
          allowClear
          style={{ minWidth: 220 }}
          placeholder={t('history.fieldsPlaceholder')}
          value={selectedFields}
          onChange={setSelectedFields}
          options={availableFields.map((f) => ({ value: f, label: fieldLabel(f) }))}
        />
        <Button onClick={queryHistory} loading={historyLoading}>
          {t('common:refresh')}
        </Button>
        <Button icon={<DownloadOutlined />} onClick={onExportHistory} disabled={historyRecords.length === 0}>
          {t('history.exportButton')}
        </Button>
        <Segmented
          value={chartView}
          onChange={(v) => setChartView(v as 'grid' | 'list')}
          options={[
            { value: 'grid', icon: <AppstoreOutlined /> },
            { value: 'list', icon: <UnorderedListOutlined /> },
          ]}
        />
      </Space>

      {historyTotal > historyRecords.length && (
        <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
          {t('history.truncatedNotice', { limit: HISTORY_PAGE_SIZE })}
        </Typography.Paragraph>
      )}

      {selectedFields.length === 0 ? (
        <Empty description={t('history.noFields')} />
      ) : (
        <Row gutter={[16, 16]}>
          {selectedFields.map((f) => (
            <Col key={f} xs={24} lg={chartView === 'grid' ? 12 : 24}>
              <FieldChart field={f} records={historyRecords} color={fieldColor(f) ?? '#185FA5'} label={fieldLabel(f)} />
            </Col>
          ))}
        </Row>
      )}
    </div>
  )

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/data-warehouse')} style={{ paddingLeft: 0 }}>
          {t('backToList')}
        </Button>
        {siblingIndex >= 0 && (
          <Space>
            <Button icon={<LeftOutlined />} disabled={siblingIndex <= 0} onClick={() => goSibling(-1)} />
            <Typography.Text type="secondary">
              {siblingIndex + 1}/{siblingIds.length}
            </Typography.Text>
            <Button icon={<RightOutlined />} disabled={siblingIndex >= siblingIds.length - 1} onClick={() => goSibling(1)} />
          </Space>
        )}
      </div>

      <Tabs
        items={[
          { key: 'dashboard', label: t('tabs.dashboard'), children: dashboardTab },
          { key: 'history', label: t('tabs.history'), children: historyTab },
        ]}
      />
    </div>
  )
}
