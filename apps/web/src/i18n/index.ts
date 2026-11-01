import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'

export const localeStorageKey = 'ipfs-panel.lang'
export const supportedLocales = ['en', 'zh'] as const
export type AppLocale = (typeof supportedLocales)[number]

function isAppLocale(value: string | null): value is AppLocale {
  return value === 'en' || value === 'zh'
}

export function readStoredLocale(): AppLocale | null {
  try {
    const saved = localStorage.getItem(localeStorageKey)
    return isAppLocale(saved) ? saved : null
  } catch {
    return null
  }
}

export function detectLocale(): AppLocale {
  const stored = readStoredLocale()
  if (stored) return stored
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

export function documentLang(locale: AppLocale): 'en' | 'zh-Hans' {
  return locale === 'zh' ? 'zh-Hans' : 'en'
}

export function applyDocumentLang(locale: AppLocale) {
  document.documentElement.lang = documentLang(locale)
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'en',
  messages: { en, zh },
})
