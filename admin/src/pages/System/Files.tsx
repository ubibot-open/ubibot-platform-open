import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Typography, Upload, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { deleteFileAsset, listFileAssets, uploadFileAsset, type FileAsset } from '../../api/fileasset'
import { ApiError } from '../../api/client'

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export default function FilesPage() {
  const [rows, setRows] = useState<FileAsset[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listFileAssets()
      setRows(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载文件列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onSubmit = async (values: { category: string }) => {
    if (!file) {
      message.error('请选择文件')
      return
    }
    setSubmitting(true)
    try {
      await uploadFileAsset({ category: values.category || 'other', file })
      message.success('已上传')
      setOpen(false)
      form.resetFields()
      setFile(null)
      load()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '上传失败')
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteFileAsset(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  const columns: ColumnsType<FileAsset> = [
    { title: '分类', dataIndex: 'category' },
    { title: '文件名', dataIndex: 'filename' },
    { title: '大小', dataIndex: 'size', render: formatSize },
    { title: '上传时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 80,
      render: (_, r) => (
        <Popconfirm title="确认删除该文件？" onConfirm={() => onDelete(r.id)}>
          <a>删除</a>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          文件管理
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          上传文件
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title="上传文件" open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="category" label="分类" initialValue="other">
            <Input placeholder="如：export / attachment" />
          </Form.Item>
          <Form.Item label="文件" required>
            <Upload
              beforeUpload={(f) => {
                setFile(f)
                return false
              }}
              maxCount={1}
              onRemove={() => setFile(null)}
            >
              <Button icon={<UploadOutlined />}>选择文件</Button>
            </Upload>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                上传
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
