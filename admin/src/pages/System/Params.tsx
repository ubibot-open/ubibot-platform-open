import { useEffect, useState } from 'react'
import { Button, Card, Input, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { listSystemParams, setSystemParam, type SystemParam } from '../../api/param'
import { ApiError } from '../../api/client'

export default function ParamsPage() {
  const [rows, setRows] = useState<SystemParam[]>([])
  const [loading, setLoading] = useState(false)
  const [edits, setEdits] = useState<Record<string, string>>({})
  const [savingKey, setSavingKey] = useState<string | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const res = await listSystemParams()
      setRows(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载系统参数失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onSave = async (p: SystemParam) => {
    const value = edits[p.key] ?? p.value
    setSavingKey(p.key)
    try {
      await setSystemParam(p.key, value, p.description)
      message.success('已保存并生效')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    } finally {
      setSavingKey(null)
    }
  }

  const columns: ColumnsType<SystemParam> = [
    { title: 'Key', dataIndex: 'key', width: 220 },
    { title: '说明', dataIndex: 'description' },
    {
      title: '值',
      dataIndex: 'value',
      width: 220,
      render: (v: string, p) => (
        <Input
          defaultValue={v}
          onChange={(e) => setEdits((prev) => ({ ...prev, [p.key]: e.target.value }))}
        />
      ),
    },
    {
      title: '操作',
      width: 90,
      render: (_, p) => (
        <Button size="small" type="primary" loading={savingKey === p.key} onClick={() => onSave(p)}>
          保存
        </Button>
      ),
    },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        系统参数
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        修改后立即生效（如请求限流阈值、离线判定宽限时间），无需重启服务。
      </Typography.Paragraph>
      <Card>
        <Table rowKey="key" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>
    </div>
  )
}
