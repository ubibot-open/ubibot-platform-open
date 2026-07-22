// Keeps antd's own locale (date pickers, table pagination text, popconfirm
// buttons, etc.) and dayjs's locale in sync with i18next's current
// language, so the parts of the UI antd renders itself follow the same
// language switch as our own t()-translated text.
import { useEffect, useState } from 'react'
import type { Locale } from 'antd/es/locale'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import jaJP from 'antd/locale/ja_JP'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/en'
import 'dayjs/locale/ja'
import i18n, { getLanguage, type SupportedLanguage } from './index'

const ANTD_LOCALES: Record<SupportedLanguage, Locale> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'ja-JP': jaJP,
}

const DAYJS_LOCALES: Record<SupportedLanguage, string> = {
  'zh-CN': 'zh-cn',
  'en-US': 'en',
  'ja-JP': 'ja',
}

export function useAntdLocale(): Locale {
  const [lang, setLang] = useState<SupportedLanguage>(getLanguage())

  useEffect(() => {
    const applyLanguage = () => {
      const next = getLanguage()
      setLang(next)
      dayjs.locale(DAYJS_LOCALES[next])
      document.documentElement.lang = next
    }
    applyLanguage()
    i18n.on('languageChanged', applyLanguage)
    return () => {
      i18n.off('languageChanged', applyLanguage)
    }
  }, [])

  return ANTD_LOCALES[lang]
}
