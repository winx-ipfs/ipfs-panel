import { describe, expect, it } from 'vitest'

import { documentLang, supportedLocales } from '@/i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'

function leafKeys(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object') {
    return prefix ? [prefix] : []
  }
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (child !== null && typeof child === 'object') {
      return leafKeys(child, path)
    }
    return [path]
  })
}

describe('locales', () => {
  it('keeps English and Chinese keys aligned', () => {
    expect(leafKeys(zh)).toEqual(leafKeys(en))
  })

  it('supports en and zh', () => {
    expect([...supportedLocales]).toEqual(['en', 'zh'])
  })
})

describe('documentLang', () => {
  it('maps the UI locale to a document language tag', () => {
    expect(documentLang('en')).toBe('en')
    expect(documentLang('zh')).toBe('zh-Hans')
  })
})
