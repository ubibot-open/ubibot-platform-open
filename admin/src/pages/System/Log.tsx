import { useEffect, useState } from 'react'
import { Card, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { listAuditLogs, type AuditLog } from '../../api/rbac'
import { apiErrorMessage } from '../../api/errors'

export default function AuditLogPage() {
  const { t } = useTranslation('systemLog')
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)

  const load = async (p = 1) => {
    setLoading(true)
    try {
      const res = await listAuditLogs(p, 20)
      setLogs(res.list)
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
  }, [])

  const columns: ColumnsType<AuditLog> = [
    { title: t('columns.time'), dataIndex: 'created_at', width: 180, render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    { title: t('columns.operator'), dataIndex: 'username', width: 120 },
    { title: t('columns.action'), dataIndex: 'action', width: 160 },
    { title: t('columns.target'), render: (_, r) => (r.target_type ? `${r.target_type}#${r.target_id}` : '-') },
    { title: t('columns.detail'), dataIndex: 'detail' },
    { title: t('columns.ip'), dataIndex: 'ip', width: 140 },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('title')}
      </Typography.Title>
      <Card>
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={logs}
          pagination={{ current: page, total, pageSize: 20, onChange: load }}
        />
      </Card>
    </div>
  )
}
