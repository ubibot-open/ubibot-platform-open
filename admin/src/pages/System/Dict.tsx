import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import { createDictEntry, deleteDictEntry, listDictEntries, updateDictEntry, type DictEntry } from '../../api/dict'
import { ApiError } from '../../api/client'

export default function DictPage() {
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
      message.error(e instanceof ApiError ? e.message : '加载字典失败')
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
      await deleteDictEntry(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  const columns: ColumnsType<DictEntry> = [
    { title: '字典类型', dataIndex: 'type' },
    { title: 'Key', dataIndex: 'key' },
    { title: '显示名称', dataIndex: 'label' },
    { title: '排序', dataIndex: 'sort' },
    {
      title: '操作',
      width: 140,
      render: (_, r) => (
        <Space>
          <a onClick={() => openEdit(r)}>编辑</a>
          <Popconfirm title="确认删除该字典项？" onConfirm={() => onDelete(r.id)}>
            <a>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          字典管理
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建字典项
        </Button>
      </div>
      <Space style={{ marginBottom: 16 }}>
        <Select
          style={{ width: 200 }}
          placeholder="按类型筛选"
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

      <Modal title={editing ? '编辑字典项' : '新建字典项'} open={open} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="type" label="字典类型" rules={[{ required: true }]} extra={editing ? '创建后不可修改' : '如 command_type'}>
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="key" label="Key" rules={[{ required: true }]} extra={editing ? '创建后不可修改' : undefined}>
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="label" label="显示名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber style={{ width: '100%' }} />
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
