import { useEffect, useState } from 'react'
import { Button, Card, Checkbox, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { PermissionCodes, createRole, deleteRole, listRoles, updateRole, type Role } from '../../api/rbac'
import { apiErrorMessage } from '../../api/errors'
import { model } from '../../constants/model'

// Maps the backend-defined permission codes (in api/rbac.ts) to translation
// keys under systemRole.json#permissionLabels, so the checkbox options show
// up in the active language instead of the hardcoded Chinese labels that
// PermissionCodes ships with. Falls back to the original label if a code is
// ever added without a matching translation key.
const permissionLabelKeys: Record<string, string> = {
  'device:read': 'permissionLabels.deviceRead',
  'device:write': 'permissionLabels.deviceWrite',
  'alert:manage': 'permissionLabels.alertManage',
  'system:manage': 'permissionLabels.systemManage',
  '*': 'permissionLabels.all',
}

export default function RolePage() {
  const { t } = useTranslation('systemRole')
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
      await deleteRole(id)
      message.success(t('common:deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('deleteFailed')))
    }
  }

  const columns: ColumnsType<Role> = [
    { title: t('columns.name'), dataIndex: 'name' },
    { title: t('columns.code'), dataIndex: 'code' },
    {
      title: t('columns.permissions'),
      dataIndex: 'permissions',
      render: (perms: string[]) =>
        perms.length ? perms.map((p) => <Tag key={p}>{p}</Tag>) : <Tag>{t('noPermission')}</Tag>,
    },
    {
      title: t('columns.actions'),
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>{t('common:edit')}</a>
          <Popconfirm
            title={t('deleteConfirmTitle')}
            onConfirm={() => onDelete(r.id)}
            disabled={r.code === model.RoleSuper}
          >
            <a style={{ color: r.code === model.RoleSuper ? '#999' : undefined }}>{t('common:delete')}</a>
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
        <Table rowKey="id" columns={columns} dataSource={roles} loading={loading} pagination={false} />
      </Card>

      <Modal title={editing ? t('modal.editTitle') : t('modal.createTitle')} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('form.name')} rules={[{ required: true }]}>
            <Input placeholder={t('form.namePlaceholder')} />
          </Form.Item>
          <Form.Item
            name="code"
            label={t('form.code')}
            rules={[{ required: true }]}
            extra={editing ? t('form.codeExtraEditing') : t('form.codeExtraCreating')}
          >
            <Input placeholder="operator" disabled={!!editing} />
          </Form.Item>
          <Form.Item name="permissions" label={t('form.permissions')}>
            <Checkbox.Group
              options={PermissionCodes.map((p) => ({
                value: p.value,
                label: permissionLabelKeys[p.value] ? t(permissionLabelKeys[p.value]) : p.label,
              }))}
            />
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
