import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Form, Input, Modal, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { createDevice, listDevices, type Device } from '../../api/device'
import { ApiError } from '../../api/client'

function formatTime(ts: number | null) {
  if (!ts) return '从未上报'
  return new Date(ts * 1000).toLocaleString()
}

export default function DevicePage() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [devices, setDevices] = useState<Device[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [form] = Form.useForm()

  const load = async (p = page) => {
    setLoading(true)
    try {
      const res = await listDevices(p, 20)
      setDevices(res.list)
      setTotal(res.total)
      setPage(p)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载设备列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onCreate = async (values: { pid: string; sn: string; name?: string; secret?: string }) => {
    setCreating(true)
    try {
      const dev = await createDevice(values)
      message.success('设备创建成功')
      setCreatedSecret(dev.secret ?? null)
      form.resetFields()
      load(1)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const columns: ColumnsType<Device> = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '设备名称', dataIndex: 'name', render: (v: string, r) => v || r.sn },
    { title: 'SN', dataIndex: 'sn' },
    { title: 'PID', dataIndex: 'pid' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (status: number) =>
        status === 1 ? <Tag color="success">启用</Tag> : <Tag color="default">停用</Tag>,
    },
    {
      title: '在线',
      dataIndex: 'online',
      width: 80,
      render: (online: boolean) => (online ? <Tag color="green">在线</Tag> : <Tag color="default">离线</Tag>),
    },
    { title: '采集/上报间隔(s)', render: (_, r) => `${r.ci} / ${r.ui}` },
    { title: '最近上报', dataIndex: 'last_seen_at', render: formatTime },
    {
      title: '操作',
      width: 100,
      render: (_, r) => <a onClick={() => navigate(`/device/${r.id}`)}>详情</a>,
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          设备管理
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setCreatedSecret(null)
            setCreateOpen(true)
          }}
        >
          新建设备
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={devices}
        loading={loading}
        pagination={{ current: page, total, pageSize: 20, onChange: load }}
      />

      <Modal
        title="新建设备"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        footer={null}
        destroyOnClose
      >
        {createdSecret ? (
          <div>
            <Typography.Paragraph>
              设备已创建。设备密钥只会显示这一次，请立即记录并烧录到设备中：
            </Typography.Paragraph>
            <Typography.Text code copyable style={{ fontSize: 14 }}>
              {createdSecret}
            </Typography.Text>
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Button type="primary" onClick={() => setCreateOpen(false)}>
                知道了
              </Button>
            </div>
          </div>
        ) : (
          <Form form={form} layout="vertical" onFinish={onCreate}>
            <Form.Item name="pid" label="产品型号 (PID)" rules={[{ required: true }]}>
              <Input placeholder="ubibot_open_dev_v1" />
            </Form.Item>
            <Form.Item name="sn" label="设备序列号 (SN)" rules={[{ required: true }]}>
              <Input placeholder="sn_ws1_20002_1" />
            </Form.Item>
            <Form.Item name="name" label="设备名称">
              <Input placeholder="选填" />
            </Form.Item>
            <Form.Item name="secret" label="设备密钥" extra="留空则自动生成">
              <Input placeholder="选填，留空自动生成" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
              <Space>
                <Button onClick={() => setCreateOpen(false)}>取消</Button>
                <Button type="primary" htmlType="submit" loading={creating}>
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
