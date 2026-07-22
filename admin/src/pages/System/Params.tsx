import { useEffect, useState } from 'react'
import { Button, Card, Input, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { listSystemParams, setSystemParam, type SystemParam } from '../../api/param'
import { apiErrorMessage } from '../../api/errors'

export default function ParamsPage() {
  const { t } = useTranslation('systemParams')
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
      message.error(apiErrorMessage(e, t('message.loadFailed')))
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
      message.success(t('message.saveSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:saveFailed')))
    } finally {
      setSavingKey(null)
    }
  }

  const columns: ColumnsType<SystemParam> = [
    { title: 'Key', dataIndex: 'key', width: 220 },
    { title: t('common:description'), dataIndex: 'description' },
    {
      title: t('table.value'),
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
      title: t('common:actions'),
      width: 90,
      render: (_, p) => (
        <Button size="small" type="primary" loading={savingKey === p.key} onClick={() => onSave(p)}>
          {t('common:save')}
        </Button>
      ),
    },
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('pageTitle')}
      </Typography.Title>
      <Typography.Paragraph type="secondary">{t('description')}</Typography.Paragraph>
      <Card>
        <Table rowKey="key" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>
    </div>
  )
}
