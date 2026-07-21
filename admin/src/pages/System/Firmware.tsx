import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Typography, Upload, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { deleteFirmware, listFirmware, uploadFirmware, type Firmware } from '../../api/ota'
import { ApiError } from '../../api/client'

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export default function FirmwarePage() {
  const [rows, setRows] = useState<Firmware[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listFirmware()
      setRows(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载固件列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onSubmit = async (values: { pid: string; version: string; signature?: string }) => {
    if (!file) {
      message.error('请选择固件文件')
      return
    }
    setSubmitting(true)
    try {
      await uploadFirmware({ ...values, file })
      message.success('固件已上传')
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
      await deleteFirmware(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  const columns: ColumnsType<Firmware> = [
    { title: 'PID', dataIndex: 'pid' },
    { title: '版本', dataIndex: 'version' },
    { title: '文件名', dataIndex: 'filename' },
    { title: '大小', dataIndex: 'size', render: formatSize },
    { title: 'SHA-256', dataIndex: 'sha256', render: (v: string) => <span style={{ fontSize: 12 }}>{v.slice(0, 16)}…</span> },
    { title: '签名', dataIndex: 'has_sig', render: (v: boolean) => (v ? '有' : '无') },
    { title: '上传时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 80,
      render: (_, r) => (
        <Popconfirm title="确认删除该固件？" onConfirm={() => onDelete(r.id)}>
          <a>删除</a>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          固件管理
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          上传固件
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal title="上传固件" open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="pid" label="产品型号 (PID)" rules={[{ required: true }]}>
            <Input placeholder="ubibot_open_dev_v1" />
          </Form.Item>
          <Form.Item name="version" label="版本号" rules={[{ required: true }]}>
            <Input placeholder="1.4.2" />
          </Form.Item>
          <Form.Item name="signature" label="签名 (可选)" extra="平台对固件的签名，用于设备端验签，见协议§7.3">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item label="固件文件" required>
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
