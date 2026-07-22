// i18n bootstrap. Namespaces are auto-discovered from
// src/i18n/locales/<lang>/<namespace>.json via Vite's import.meta.glob --
// adding a new namespace file for an existing language is enough; nothing
// here needs to change. Every page/component gets its own namespace named
// after it (see each locales/*/*.json file for the convention), plus two
// shared namespaces: "common" (generic buttons/labels/toasts reused across
// pages) and "errors" (translations for the backend's stable adminErr
// codes -- see src/api/errors.ts).
import i18next, { type Resource } from 'i18next'
import { initReactI18next } from 'react-i18next'

export const SUPPORTED_LANGUAGES = ['zh-CN', 'en-US', 'ja-JP'] as const
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number]

export const LANGUAGE_LABELS: Record<SupportedLanguage, string> = {
  'zh-CN': '简体中文',
  'en-US': 'English',
  'ja-JP': '日本語',
}

const STORAGE_KEY = 'ubibot_admin_lang'

const modules = import.meta.glob('./locales/*/*.json', { eager: true }) as Record<
  string,
  { default: Record<string, unknown> }
>

const resources: Resource = {}
for (const path in modules) {
  const match = path.match(/\.\/locales\/([^/]+)\/([^/]+)\.json$/)
  if (!match) continue
  const [, lang, ns] = match
  resources[lang] ??= {}
  resources[lang][ns] = modules[path].default
}

function isSupported(lang: string): lang is SupportedLanguage {
  return (SUPPORTED_LANGUAGES as readonly string[]).includes(lang)
}

function detectInitialLanguage(): SupportedLanguage {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved && isSupported(saved)) return saved

  // navigator.language is typically "zh-CN"/"zh-TW"/"ja"/"ja-JP"/"en-US" etc.
  // -- match on the primary subtag so regional variants still resolve.
  const primary = navigator.language.split('-')[0].toLowerCase()
  if (primary === 'zh') return 'zh-CN'
  if (primary === 'ja') return 'ja-JP'
  return 'en-US'
}

i18next.use(initReactI18next).init({
  resources,
  lng: detectInitialLanguage(),
  fallbackLng: 'en-US',
  interpolation: { escapeValue: false },
  returnNull: false,
});

export function setLanguage(lang: SupportedLanguage) {
  localStorage.setItem(STORAGE_KEY, lang)
  void i18next.changeLanguage(lang)
}

export function getLanguage(): SupportedLanguage {
  const current = i18next.language
  return isSupported(current) ? current : 'en-US'
}

export default i18next
