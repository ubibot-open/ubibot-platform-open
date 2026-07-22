import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Avatar,
  Button,
  Card,
  Col,
  Empty,
  Input,
  Row,
  Segmented,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { AppstoreOutlined, DownloadOutlined, HddOutlined, SearchOutlined, UnorderedListOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { listDataWarehouse, type DataWarehouseItem } from '../../api/device'
import { apiErrorMessage } from '../../api/errors'
import { useFieldIcons } from '../../hooks/useFieldIcons'
import { formatFieldValue } from '../../utils/sensorValue'
import { toCsv, downloadCsv } from '../../utils/csv'
import RelativeTime from '../../components/RelativeTime'

type OnlineFilter = 'all' | 'online' | 'offline'

export default function DataWarehousePage() {
  const { t } = useTranslation('dataWarehouse')
  const { renderFieldIcon, fieldColor } = useFieldIcons()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [items, setItems] = useState<DataWarehouseItem[]>([])
  const [total, setTotal] = useState(0)
  const [keyword, setKeyword] = useState('')
  const [onlineFilter, setOnlineFilter] = useState<OnlineFilter>('all')
  const [viewMode, setViewMode] = useState<'list' | 'grid'>('list')

  const load = async () => {
    setLoading(true)
    try {
      // A single generous page covers the vast majority of self-hosted
      // deployments. Search/status filtering below runs client-side over
      // this set instead of round-tripping to the server per keystroke --
      // there is no server-side search endpoint for this view.
      const res = await listDataWarehouse(1, 200)
      setItems(res.list)
      setTotal(res.total)
    } catch (e) {
      message.error(apiErrorMessage(e, t('loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const kw = keyword.trim().toLowerCase()
  const filtered = items.filter((it) => {
    if (onlineFilter === 'online' && !it.online) return false
    if (onlineFilter === 'offline' && it.online) return false
    if (kw && !(it.name.toLowerCase().includes(kw) || it.sn.toLowerCase().includes(kw))) return false
    return true
  })

  const onExport = () => {
    const fieldKeys = Array.from(
      new Set(filtered.flatMap((it) => (it.last_record ? Object.keys(it.last_record.d) : []))),
    ).sort()
    const header = ['name', 'sn', 'pid', 'activated', 'online', 'last_seen_at', 'created_at', ...fieldKeys]
    const rows = filtered.map((it) => [
      it.name || it.sn,
      it.sn,
      it.pid,
      it.activated ? t('activated.yes') : t('activated.no'),
      it.online ? t('online.yes') : t('online.no'),
      it.last_seen_at ? new Date(it.last_seen_at * 1000).toISOString() : '',
      new Date(it.created_at * 1000).toISOString(),
      ...fieldKeys.map((k) => formatFieldValue(it.last_record?.d[k])),
    ])
    downloadCsv(`data-warehouse-${new Date().toISOString().slice(0, 10)}.csv`, toCsv([header, ...rows]))
  }

  function renderSensorTags(item: DataWarehouseItem) {
    const entries = item.last_record ? Object.entries(item.last_record.d) : []
    if (entries.length === 0) return <Typography.Text type="secondary">{t('noData')}</Typography.Text>
    return (
      <Space size={18} wrap align="start">
        {entries.map(([k, v]) => {
          // fields.<key> is only populated for the common sensor names our
          // built-in icon set covers; anything else just shows the raw
          // field key as its tooltip label instead of a translated one. A
          // custom icon uploaded via 图标库 keeps this same label lookup --
          // only the icon graphic itself is swapped out.
          const label = t(`fields.${k.toLowerCase()}`, { defaultValue: k })
          return (
            <Tooltip key={k} title={label}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 36 }}>
                <span style={{ color: fieldColor(k), fontSize: 18, lineHeight: 1 }}>{renderFieldIcon(k)}</span>
                <span style={{ fontSize: 12, color: 'rgba(0, 0, 0, 0.65)', marginTop: 4 }}>
                  {formatFieldValue(v)}
                </span>
              </div>
            </Tooltip>
          )
        })}
      </Space>
    )
  }

  const columns: ColumnsType<DataWarehouseItem> = [
    {
      title: t('columns.device'),
      render: (_, r) => (
        <Space
          style={{ cursor: 'pointer' }}
          onClick={() => navigate(`/data-warehouse/${r.id}`)}
        >
          <Avatar icon={<HddOutlined />} />
          <div>
            <div>{r.name || r.sn}</div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {r.sn}
            </Typography.Text>
          </div>
        </Space>
      ),
    },
    {
      title: t('columns.status'),
      width: 100,
      render: (_, r) =>
        r.online ? <Tag color="green">{t('online.yes')}</Tag> : <Tag color="default">{t('online.no')}</Tag>,
    },
    { title: t('columns.sensorData'), render: (_, r) => renderSensorTags(r) },
    {
      title: t('columns.lastSeen'),
      width: 160,
      render: (_, r) => <RelativeTime ts={r.last_seen_at} fallback={t('neverReported')} />,
    },
    {
      title: t('columns.createdAt'),
      width: 180,
      render: (_, r) => new Date(r.created_at * 1000).toLocaleString(),
    },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('title')}
      </Typography.Title>
      <Typography.Paragraph type="secondary">{t('subtitle', { count: total })}</Typography.Paragraph>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            placeholder={t('searchPlaceholder')}
            prefix={<SearchOutlined />}
            allowClear
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            style={{ width: 240 }}
          />
          <Select
            value={onlineFilter}
            onChange={(v) => setOnlineFilter(v as OnlineFilter)}
            style={{ width: 140 }}
            options={[
              { value: 'all', label: t('filters.all') },
              { value: 'online', label: t('online.yes') },
              { value: 'offline', label: t('online.no') },
            ]}
          />
          <Button icon={<DownloadOutlined />} onClick={onExport} disabled={filtered.length === 0}>
            {t('exportButton')}
          </Button>
          <Segmented
            value={viewMode}
            onChange={(v) => setViewMode(v as 'list' | 'grid')}
            options={[
              { value: 'list', icon: <UnorderedListOutlined /> },
              { value: 'grid', icon: <AppstoreOutlined /> },
            ]}
          />
          <Button onClick={load}>{t('common:refresh')}</Button>
        </Space>

        {viewMode === 'list' ? (
          <Table
            rowKey="id"
            loading={loading}
            columns={columns}
            dataSource={filtered}
            pagination={{ pageSize: 20, showTotal: (n) => t('common:total', { count: n }) }}
          />
        ) : filtered.length === 0 ? (
          <Empty description={t('common:noData')} />
        ) : (
          <Row gutter={[16, 16]}>
            {filtered.map((it) => (
              <Col key={it.id} xs={24} sm={12} md={8} lg={6}>
                <Card
                  size="small"
                  hoverable
                  onClick={() => navigate(`/data-warehouse/${it.id}`)}
                  title={
                    <Space>
                      <Avatar icon={<HddOutlined />} size="small" />
                      <span>{it.name || it.sn}</span>
                    </Space>
                  }
                >
                  <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
                    {it.sn}
                  </Typography.Paragraph>
                  <Space style={{ marginBottom: 8 }}>
                    {it.online ? (
                      <Tag color="green">{t('online.yes')}</Tag>
                    ) : (
                      <Tag color="default">{t('online.no')}</Tag>
                    )}
                    {it.activated ? (
                      <Tag color="blue">{t('activated.yes')}</Tag>
                    ) : (
                      <Tag color="default">{t('activated.no')}</Tag>
                    )}
                  </Space>
                  <div style={{ marginBottom: 8 }}>{renderSensorTags(it)}</div>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {t('columns.lastSeen')}: <RelativeTime ts={it.last_seen_at} fallback={t('neverReported')} />
                  </Typography.Text>
                </Card>
              </Col>
            ))}
          </Row>
        )}
      </Card>
    </div>
  )
}
