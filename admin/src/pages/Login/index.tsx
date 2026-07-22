import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Form, Input, Typography, Alert } from 'antd'
import { UserOutlined, LockOutlined, ApiOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../../contexts/AuthContext'
import { apiErrorMessage } from '../../api/errors'

export default function LoginPage() {
  const { t } = useTranslation('login')
  const { login } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const onFinish = async (values: { username: string; password: string }) => {
    setError(null)
    setSubmitting(true)
    try {
      await login(values.username, values.password)
      navigate('/dashboard', { replace: true })
    } catch (e) {
      setError(apiErrorMessage(e, t('loginFailed')))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#faf9f6',
      }}
    >
      <Card style={{ width: 360 }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <ApiOutlined style={{ fontSize: 28, color: '#185FA5' }} />
          <Typography.Title level={4} style={{ margin: '8px 0 0' }}>
            {t('title')}
          </Typography.Title>
        </div>
        {error && <Alert type="error" message={error} showIcon style={{ marginBottom: 16 }} />}
        <Form layout="vertical" onFinish={onFinish} autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: t('form.usernameRequired') }]}>
            <Input prefix={<UserOutlined />} placeholder={t('form.usernamePlaceholder')} size="large" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: t('form.passwordRequired') }]}>
            <Input.Password prefix={<LockOutlined />} placeholder={t('form.passwordPlaceholder')} size="large" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block size="large" loading={submitting}>
              {t('form.submit')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
