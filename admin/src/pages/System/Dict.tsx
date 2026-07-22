import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { createDictEntry, deleteDictEntry, listDictEntries, updateDictEntry, type DictEntry } from '../../api/dict'
import { apiErrorMessage } from '../../api/errors'

export default function DictPage() {
  const { t } = useTranslation('systemDict')
  const [rows, setRows] = useState<DictEntry[]>([])
  const [typeFilter, setTypeFilter] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<DictEntry | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listDictEntries(typeFilter)
      setRows(res.list)
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.loadFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [typeFilter])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setOpen(true)
  }

  const openEdit = (e: DictEntry) => {
    setEditing(e)
    form.setFieldsValue(e)
    setOpen(true)
  }

  const onSubmit = async (values: { type: string; key: string; label: string; sort: number }) => {
    setSubmitting(true)
    try {
      if (editing) {
        await updateDictEntry(editing.id, { label: values.label, sort: values.sort ?? 0 })
      } else {
        await createDictEntry(values)
      }
      message.success(t('common:saveSuccess'))
      setOpen(false)
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:saveFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    try {
      await deleteDictEntry(id)
      message.success(t('common:deleteSuccess'))
      load()
    } catch (e) {
      message.error(apiErrorMessage(e, t('common:deleteFailed')))
    }
  }

  const columns: ColumnsType<DictEntry> = [
    { title: t('table.dictType'), dataIndex: 'type' },
    { title: 'Key', dataIndex: 'key' },
    { title: t('table.label'), dataIndex: 'label' },
    { title: t('table.sort'), dataIndex: 'sort' },
    {
      title: t('common:actions'),
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>{t('common:edit')}</a>
          <Popconfirm title={t('deleteConfirm')} onConfirm={() => onDelete(r.id)}>
            <a>{t('common:delete')}</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('pageTitle')}
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          {t('createButton')}
        </Button>
      </div>
      <Space style={{ marginBottom: 16 }}>
        <Select
          style={{ width: 200 }}
          placeholder={t('filterPlaceholder')}
          allowClear
          value={typeFilter}
          onChange={setTypeFilter}
          options={[
            { value: 'command_type', label: 'command_type' },
            { value: 'probe_iface', label: 'probe_iface' },
            { value: 'probe_proto', label: 'probe_proto' },
          ]}
        />
      </Space>
      <Card>
        <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} pagination={false} />
      </Card>

      <Modal
        title={editing ? t('modal.editTitle') : t('modal.createTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item
            name="type"
            label={t('table.dictType')}
            rules={[{ required: true }]}
            extra={editing ? t('modal.typeExtraLocked') : t('modal.typeExtraHint')}
          >
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="key" label="Key" rules={[{ required: true }]} extra={editing ? t('modal.typeExtraLocked') : undefined}>
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="label" label={t('table.label')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="sort" label={t('table.sort')} initialValue={0}>
            <InputNumber style={{ width: '100%' }} />
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
