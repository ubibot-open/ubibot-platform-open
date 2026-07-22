import { useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Space, Table, Tag, Typography, Upload, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, UploadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { deleteIcon, uploadIcon } from '../../api/icon'
import { apiErrorMessage } from '../../api/errors'
import { useFieldIcons } from '../../hooks/useFieldIcons'
import { BUILTIN_FIELD_ICONS } from '../../components/icons/SensorIcons'

interface IconRow {
  key: string
  isCustom: boolean
}

// Manages the per-sensor-field icon overrides the 数据仓库 page renders
// (see hooks/useFieldIcons.tsx): every built-in field plus every key that
// currently has a custom upload, each row showing whichever icon is
// actually in effect right now (custom if present, built-in default
// otherwise) so this page doubles as a live preview.
export default function IconLibraryPage() {
  const { t } = useTranslation('systemIcon')
  const { customIcons, loaded, reload, renderFieldIcon, fieldColor } = useFieldIcons()
  const [target, setTarget] = useState<{ key: string; locked: boolean } | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const rows: IconRow[] = Array.from(new Set([...Object.keys(BUILTIN_FIELD_ICONS), ...Object.keys(customIcons)]))
    .sort()
    .map((key) => ({ key, isCustom: Boolean(customIcons[key]) }))

  const openUpload = (key: string, locked: boolean) => {
    setFile(null)
    form.setFieldsValue({ key, name: customIcons[key]?.name ?? '' })
    setTarget({ key, locked })
  }

  const onDelete = async (key: string) => {
    try {
      await deleteIcon(key)
      message.success(t('message.deleteSuccess'))
      reload()
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.deleteFailed')))
    }
  }

  const onSubmit = async (values: { key: string; name: string }) => {
    if (!file) {
      message.error(t('message.fileRequired'))
      return
    }
    setSubmitting(true)
    try {
      const svg = await file.text()
      if (!svg.includes('<svg')) {
        message.error(t('message.invalidFile'))
        return
      }
      const key = values.key.trim()
      await uploadIcon({ key, name: values.name.trim() || key, svg })
      message.success(t('message.uploadSuccess'))
      setTarget(null)
      reload()
    } catch (e) {
      message.error(apiErrorMessage(e, t('message.uploadFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  const columns: ColumnsType<IconRow> = [
    {
      title: t('table.icon'),
      width: 70,
      render: (_, r) => <span style={{ fontSize: 22, color: fieldColor(r.key) }}>{renderFieldIcon(r.key)}</span>,
    },
    { title: t('table.key'), dataIndex: 'key' },
    {
      title: t('table.name'),
      render: (_, r) => customIcons[r.key]?.name ?? t(`dataWarehouse:fields.${r.key}`, { defaultValue: r.key }),
    },
    {
      title: t('table.source'),
      width: 110,
      render: (_, r) =>
        r.isCustom ? <Tag color="blue">{t('source.custom')}</Tag> : <Tag color="default">{t('source.builtin')}</Tag>,
    },
    {
      title: t('common:actions'),
      width: 160,
      render: (_, r) => (
        <Space>
          <a onClick={() => openUpload(r.key, true)}>{t('replaceButton')}</a>
          {r.isCustom && (
            <Popconfirm title={t('deleteConfirm')} onConfirm={() => onDelete(r.key)}>
              <a>{t('common:delete')}</a>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Typography.Title level={4} style={{ margin: 0 }}>
            {t('pageTitle')}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {t('subtitle')}
          </Typography.Paragraph>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openUpload('', false)}>
          {t('addButton')}
        </Button>
      </div>
      <Card>
        <Table rowKey="key" columns={columns} dataSource={rows} loading={!loaded} pagination={false} />
      </Card>

      <Modal title={t('modal.title')} open={target !== null} onCancel={() => setTarget(null)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item
            name="key"
            label={t('modal.keyLabel')}
            rules={[{ required: true, message: t('message.keyRequired') }]}
            extra={target?.locked ? t('modal.keyExtraLocked') : t('modal.keyExtraHint')}
          >
            <Input placeholder={t('modal.keyPlaceholder')} disabled={target?.locked} />
          </Form.Item>
          <Form.Item name="name" label={t('modal.nameLabel')}>
            <Input placeholder={t('modal.namePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('modal.fileLabel')} required>
            <Upload
              accept=".svg,image/svg+xml"
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
              <Button onClick={() => setTarget(null)}>{t('common:cancel')}</Button>
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
