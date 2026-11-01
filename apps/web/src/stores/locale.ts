import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  applyDocumentLang,
  i18n,
  localeStorageKey,
  type AppLocale,
} from '@/i18n'

export const useLocaleStore = defineStore('locale', () => {
  const locale = ref<AppLocale>(i18n.global.locale.value as AppLocale)

  function setLocale(next: AppLocale) {
    locale.value = next
    i18n.global.locale.value = next
    applyDocumentLang(next)
    try {
      localStorage.setItem(localeStorageKey, next)
    } catch {
      // The switch still works for this tab if storage is blocked.
    }
  }

  function init() {
    applyDocumentLang(locale.value)
  }

  return { locale, setLocale, init }
})
