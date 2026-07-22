import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { createApiKey, listApiKeys, revokeApiKey, type ApiKey } from '../../api/apikey'
import { apiErrorMessage } from '../../api/errors'

export default function ApiKeyPage() {
  const { t } = useTranslation('systemApiKey')
  const [rows, setRows] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [rawKey, setRawKey] = useState<string | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listApiKeys()
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

  const onCreate = async (values: { name: string }) => {
    setSubmitting(true)
    try {
      const res = await createApiKey(values.name)
      setRawKey(res.raw_key)
      form.resetFields()
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.createFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const onRevoke = async (id: number) => {
    try {
      await revokeApiKey(id)
      message.success(t('message.revokeSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.revokeFailed')))
    }
  }

  const columns: ColumnsType<ApiKey> = [
    { title: t('common:name'), dataIndex: 'name' },
    { title: t('table.prefix'), dataIndex: 'prefix', render: (v: string) => <code>{v}...</code> },
    {
      title: t('common:status'),
      dataIndex: 'revoked',
      render: (v: boolean) => (v ? <Tag>{t('table.revoked')}</Tag> : <Tag color="green">{t('table.active')}</Tag>),
    },
    {
      title: t('table.lastUsedAt'),
      dataIndex: 'last_used_at',
      render: (ts: number | null) => (ts ? new Date(ts * 1000).toLocaleString() : t('table.neverUsed')),
    },
    { title: t('common:createdAt'), dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: t('common:actions'),
      width: 80,
      render: (_, r) =>
        r.revoked ? (
          '-'
        ) : (
          <Popconfirm title={t('revokeConfirm')} onConfirm={() => onRevoke(r.id)}>
            <a>{t('table.revokeAction')}</a>
          </Popconfirm>
        ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('pageTitle')}
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setRawKey(null)
            setOpen(true)
          }}
        >
          {t('createButton')}
        </Button>
      </div>
      <Typography.Paragraph type="secondary">{t('description')}</Typography.Paragraph>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title={t('modal.createTitle')} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        {rawKey ? (
          <div>
            <Typography.Paragraph>{t('modal.keyCreatedNotice')}</Typography.Paragraph>
            <Typography.Text code copyable style={{ fontSize: 14 }}>
              {rawKey}
            </Typography.Text>
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Button type="primary" onClick={() => setOpen(false)}>
                {t('modal.gotIt')}
              </Button>
            </div>
          </div>
        ) : (
          <Form form={form} layout="vertical" onFinish={onCreate}>
            <Form.Item name="name" label={t('modal.nameLabel')} rules={[{ required: true }]}>
              <Input placeholder={t('modal.namePlaceholder')} />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
              <Space>
                <Button onClick={() => setOpen(false)}>{t('common:cancel')}</Button>
                <Button type="primary" htmlType="submit" loading={submitting}>
                  {t('common:create')}
                </Button>
              </Space>
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  )
}
