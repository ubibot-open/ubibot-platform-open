import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { createApiKey, listApiKeys, revokeApiKey, type ApiKey } from '../../api/apikey'
import { ApiError } from '../../api/client'

export default function ApiKeyPage() {
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
      message.error(e instanceof ApiError ? e.message : '加载API Key列表失败')
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
      message.error(e instanceof ApiError ? e.message : '创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  const onRevoke = async (id: number) => {
    try {
      await revokeApiKey(id)
      message.success('已吊销')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '操作失败')
    }
  }

  const columns: ColumnsType<ApiKey> = [
    { title: '名称', dataIndex: 'name' },
    { title: '前缀', dataIndex: 'prefix', render: (v: string) => <code>{v}...</code> },
    { title: '状态', dataIndex: 'revoked', render: (v: boolean) => (v ? <Tag>已吊销</Tag> : <Tag color="green">有效</Tag>) },
    {
      title: '最近使用',
      dataIndex: 'last_used_at',
      render: (ts: number | null) => (ts ? new Date(ts * 1000).toLocaleString() : '从未使用'),
    },
    { title: '创建时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 80,
      render: (_, r) =>
        r.revoked ? (
          '-'
        ) : (
          <Popconfirm title="确认吊销该Key？吊销后立即失效。" onConfirm={() => onRevoke(r.id)}>
            <a>吊销</a>
          </Popconfirm>
        ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          开放API密钥
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setRawKey(null)
            setOpen(true)
          }}
        >
          新建Key
        </Button>
      </div>
      <Typography.Paragraph type="secondary">
        用于第三方系统调用只读开放接口（GET /api/open/v1/...），通过 X-Api-Key 请求头鉴权。
      </Typography.Paragraph>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title="新建API Key" open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        {rawKey ? (
          <div>
            <Typography.Paragraph>Key已创建，只会显示这一次，请立即保存：</Typography.Paragraph>
            <Typography.Text code copyable style={{ fontSize: 14 }}>
              {rawKey}
            </Typography.Text>
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Button type="primary" onClick={() => setOpen(false)}>
                知道了
              </Button>
            </div>
          </div>
        ) : (
          <Form form={form} layout="vertical" onFinish={onCreate}>
            <Form.Item name="name" label="用途说明" rules={[{ required: true }]}>
              <Input placeholder="如：BI报表系统集成" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
              <Space>
                <Button onClick={() => setOpen(false)}>取消</Button>
                <Button type="primary" htmlType="submit" loading={submitting}>
                  创建
                </Button>
              </Space>
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  )
}
