import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  createAdminUser,
  deleteAdminUser,
  listAdminUsers,
  listRoles,
  updateAdminUser,
  type AdminUser,
  type Role,
} from '../../api/rbac'
import { apiErrorMessage } from '../../api/errors'
import { useAuth } from '../../contexts/AuthContext'

export default function AdminUserPage() {
  const { t } = useTranslation('systemAdmin')
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
      message.error(apiErrorMessage(e, t('loadFailed')))
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
          message.error(t('usernamePasswordRequired'))
          setSubmitting(false)
          return
        }
        await createAdminUser({ username: values.username, password: values.password, role_id: values.role_id })
      }
      message.success(t('common:saveSuccess'))
      setOpen(false)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('saveFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteAdminUser(id)
      message.success(t('common:deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('deleteFailed')))
    }
  }

  const columns: ColumnsType<AdminUser> = [
    { title: t('columns.username'), dataIndex: 'username' },
    { title: t('columns.role'), dataIndex: 'role_name' },
    { title: t('columns.createdAt'), dataIndex: 'created_at', render: (ts: number) => new Date(ts * 1000).toLocaleString() },
    {
      title: t('columns.actions'),
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>{t('common:edit')}</a>
          <Popconfirm title={t('deleteConfirmTitle')} onConfirm={() => onDelete(r.id)} disabled={r.username === myUsername}>
            <a style={{ color: r.username === myUsername ? '#999' : undefined }}>{t('common:delete')}</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('title')}
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          {t('createButton')}
        </Button>
      </div>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={admins} loading={loading} pagination={false} />
      </Card>

      <Modal
        title={editing ? t('modal.editTitle') : t('modal.createTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          {!editing && (
            <Form.Item name="username" label={t('form.username')} rules={[{ required: true }]}>
              <Input />
            </Form.Item>
          )}
          <Form.Item
            name="password"
            label={editing ? t('form.passwordResetLabel') : t('form.password')}
            rules={editing ? [] : [{ required: true }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item name="role_id" label={t('form.role')} rules={[{ required: true }]}>
            <Select options={roles.map((r) => ({ value: r.id, label: r.name }))} placeholder={t('form.rolePlaceholder')} />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setOpen(false)}>{t('common:cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {t('common:save')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
