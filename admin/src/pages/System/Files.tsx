import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Typography, Upload, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { deleteFileAsset, listFileAssets, uploadFileAsset, type FileAsset } from '../../api/fileasset'
import { apiErrorMessage } from '../../api/errors'

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export default function FilesPage() {
  const { t } = useTranslation('systemFiles')
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
      message.error(apiErrorMessage(e, t('message.loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onSubmit = async (values: { category: string }) => {
    if (!file) {
      message.error(t('message.selectFileRequired'))
      return
    }
    setSubmitting(true)
    try {
      await uploadFileAsset({ category: values.category || 'other', file })
      message.success(t('message.uploadSuccess'))
      setOpen(false)
      form.resetFields()
      setFile(null)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.uploadFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteFileAsset(id)
      message.success(t('common:deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:deleteFailed')))
    }
  }

  const columns: ColumnsType<FileAsset> = [
    { title: t('table.category'), dataIndex: 'category' },
    { title: t('table.filename'), dataIndex: 'filename' },
    { title: t('table.size'), dataIndex: 'size', render: formatSize },
    { title: t('table.uploadedAt'), dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: t('common:actions'),
      width: 80,
      render: (_, r) => (
        <Popconfirm title={t('deleteConfirm')} onConfirm={() => onDelete(r.id)}>
          <a>{t('common:delete')}</a>
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
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          {t('uploadButton')}
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title={t('modal.title')} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="category" label={t('modal.categoryLabel')} initialValue="other">
            <Input placeholder={t('modal.categoryPlaceholder')} />
          </Form.Item>
          <Form.Item label={t('modal.fileLabel')} required>
            <Upload
              beforeUpload={(f) => {
                setFile(f)
                return false
              }}
              maxCount={1}
              onRemove={() => setFile(null)}
            >
              <Button icon={<UploadOutlined />}>{t('modal.selectFileButton')}</Button>
            </Upload>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>{t('common:cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {t('modal.submitButton')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
