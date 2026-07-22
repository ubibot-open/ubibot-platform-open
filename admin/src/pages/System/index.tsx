import { useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import PagePlaceholder from '../../components/PagePlaceholder'

export default function SystemPage() {
  const { t } = useTranslation('system')
  const { pathname } = useLocation()

  const titleByPath: Record<string, string> = {
    '/system/admin': t('titles.admin'),
    '/system/role': t('titles.role'),
    '/system/log': t('titles.log'),
  }

  return (
    <PagePlaceholder
      title={titleByPath[pathname] ?? t('titles.default')}
      description={t('placeholderDescription')}
    />
  )
}
