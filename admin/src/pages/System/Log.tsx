import { useEffect, useState } from 'react'
import { Card, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { listAuditLogs, type AuditLog } from '../../api/rbac'
import { ApiError } from '../../api/client'

export default function AuditLogPage() {
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
      message.error(e instanceof ApiError ? e.message : '加载操作日志失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
  }, [])

  const columns: ColumnsType<AuditLog> = [
    { title: '时间', dataIndex: 'created_at', width: 180, render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    { title: '操作人', dataIndex: 'username', width: 120 },
    { title: '动作', dataIndex: 'action', width: 160 },
    { title: '对象', render: (_, r) => (r.target_type ? `${r.target_type}#${r.target_id}` : '-') },
    { title: '详情', dataIndex: 'detail' },
    { title: 'IP', dataIndex: 'ip', width: 140 },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        操作日志
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
