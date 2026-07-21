import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import {
  createAdminUser,
  deleteAdminUser,
  listAdminUsers,
  listRoles,
  updateAdminUser,
  type AdminUser,
  type Role,
} from '../../api/rbac'
import { ApiError } from '../../api/client'
import { useAuth } from '../../contexts/AuthContext'

export default function AdminUserPage() {
  const { username: myUsername } = useAuth()
  const [admins, setAdmins] = useState<AdminUser[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AdminUser | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const [a, r] = await Promise.all([listAdminUsers(), listRoles()])
      setAdmins(a.list)
      setRoles(r.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载管理员列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setOpen(true)
  }

  const openEdit = (a: AdminUser) => {
    setEditing(a)
    form.setFieldsValue({ role_id: a.role_id, password: '' })
    setOpen(true)
  }

  const onSubmit = async (values: { username?: string; password?: string; role_id: number }) => {
    setSubmitting(true)
    try {
      if (editing) {
        await updateAdminUser(editing.id, {
          role_id: values.role_id,
          password: values.password || undefined,
        })
      } else {
        if (!values.username || !values.password) {
          message.error('用户名和密码为必填项')
          setSubmitting(false)
          return
        }
        await createAdminUser({ username: values.username, password: values.password, role_id: values.role_id })
      }
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteAdminUser(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  const columns: ColumnsType<AdminUser> = [
    { title: '用户名', dataIndex: 'username' },
    { title: '角色', dataIndex: 'role_name' },
    { title: '创建时间', dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: '操作',
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>编辑</a>
          <Popconfirm title="确认删除该管理员？" onConfirm={() => onDelete(r.id)} disabled={r.username === myUsername}>
            <a style={{ color: r.username === myUsername ? '#999' : undefined }}>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          管理员
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建管理员
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={admins} loading={loading} pagination={false} />
      </Card>

      <Modal
        title={editing ? '编辑管理员' : '新建管理员'}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          {!editing && (
            <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
          )}
          <Form.Item
            name="password"
            label={editing ? '重置密码（留空则不修改）' : '密码'}
            rules={editing ? [] : [{ required: true }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item name="role_id" label="角色" rules={[{ required: true }]}>
            <Select options={roles.map((r) => ({ value: r.id, label: r.name }))} placeholder="选择角色" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
