import { useEffect, useState } from 'react'
import { Button, Card, Checkbox, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { PermissionCodes, createRole, deleteRole, listRoles, updateRole, type Role } from '../../api/rbac'
import { ApiError } from '../../api/client'
import { model } from '../../constants/model'

export default function RolePage() {
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Role | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listRoles()
      setRoles(res.list)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载角色列表失败')
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

  const openEdit = (role: Role) => {
    setEditing(role)
    form.setFieldsValue({ name: role.name, code: role.code, permissions: role.permissions })
    setOpen(true)
  }

  const onSubmit = async (values: { name: string; code: string; permissions: string[] }) => {
    setSubmitting(true)
    try {
      if (editing) {
        await updateRole(editing.id, { name: values.name, permissions: values.permissions })
      } else {
        await createRole(values)
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
      await deleteRole(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败（可能仍有管理员使用该角色）')
    }
  }

  const columns: ColumnsType<Role> = [
    { title: '名称', dataIndex: 'name' },
    { title: '代码', dataIndex: 'code' },
    {
      title: '权限',
      dataIndex: 'permissions',
      render: (perms: string[]) =>
        perms.length ? perms.map((p) => <Tag key={p}>{p}</Tag>) : <Tag>无</Tag>,
    },
    {
      title: '操作',
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>编辑</a>
          <Popconfirm
            title="确认删除该角色？"
            onConfirm={() => onDelete(r.id)}
            disabled={r.code === model.RoleSuper}
          >
            <a style={{ color: r.code === model.RoleSuper ? '#999' : undefined }}>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          角色管理
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建角色
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={roles} loading={loading} pagination={false} />
      </Card>

      <Modal title={editing ? '编辑角色' : '新建角色'} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label="角色名称" rules={[{ required: true }]}>
            <Input placeholder="如：运维操作员" />
          </Form.Item>
          <Form.Item
            name="code"
            label="角色代码"
            rules={[{ required: true }]}
            extra={editing ? '代码创建后不可修改' : '英文标识，如 operator'}
          >
            <Input placeholder="operator" disabled={!!editing} />
          </Form.Item>
          <Form.Item name="permissions" label="权限">
            <Checkbox.Group options={PermissionCodes.map((p) => ({ value: p.value, label: p.label }))} />
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
