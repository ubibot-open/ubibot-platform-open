import { GlobalOutlined } from '@ant-design/icons'
import { Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { useTranslation } from 'react-i18next'
import { LANGUAGE_LABELS, SUPPORTED_LANGUAGES, setLanguage, type SupportedLanguage } from '../i18n'

// Dropdown language switcher, meant to sit in AppLayout's header next to
// the theme toggle / user menu. Persists the choice (see src/i18n/index.ts)
// so it survives a reload. useTranslation() subscribes this component to
// i18next's languageChanged event, so it re-renders (and highlights the
// right entry) immediately after switching.
export default function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const current: SupportedLanguage = (SUPPORTED_LANGUAGES as readonly string[]).includes(
    i18n.language,
  )
    ? (i18n.language as SupportedLanguage)
    : 'en-US'

  const items: MenuProps['items'] = SUPPORTED_LANGUAGES.map((lang) => ({
    key: lang,
    label: LANGUAGE_LABELS[lang],
  }))

  const onClick: MenuProps['onClick'] = ({ key }) => {
    setLanguage(key as SupportedLanguage)
  }

  return (
    <Dropdown menu={{ items, onClick, selectedKeys: [current] }} placement="bottomRight">
      <span style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        <GlobalOutlined />
        {LANGUAGE_LABELS[current]}
      </span>
    </Dropdown>
  )
}
