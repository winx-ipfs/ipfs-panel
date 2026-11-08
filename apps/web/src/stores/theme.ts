import { defineStore } from 'pinia'
import { ref } from 'vue'

export const themeStorageKey = 'ipfs-panel.theme'
export type ThemePreference = 'light' | 'dark' | 'system'

function isPreference(value: string | null): value is ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system'
}

function systemIsDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveTheme(preference: ThemePreference): 'light' | 'dark' {
  if (preference === 'system') return systemIsDark() ? 'dark' : 'light'
  return preference
}

export function applyTheme(preference: ThemePreference) {
  const resolved = resolveTheme(preference)
  document.documentElement.dataset.theme = resolved
  document.documentElement.style.colorScheme = resolved
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', resolved === 'dark' ? '#0c1210' : '#e4ece7')
  return resolved
}

export const useThemeStore = defineStore('theme', () => {
  const preference = ref<ThemePreference>('system')
  const resolved = ref<'light' | 'dark'>('light')
  let media: MediaQueryList | null = null

  function setPreference(next: ThemePreference) {
    preference.value = next
    resolved.value = applyTheme(next)
    try {
      localStorage.setItem(themeStorageKey, next)
    } catch {
      // The choice still applies for this tab if storage is blocked.
    }
  }

  function onSystemChange() {
    if (preference.value === 'system') {
      resolved.value = applyTheme('system')
    }
  }

  function init() {
    let stored: string | null = null
    try {
      stored = localStorage.getItem(themeStorageKey)
    } catch {
      // Storage can be blocked. The theme still applies for this tab.
    }
    const next = isPreference(stored) ? stored : 'system'
    preference.value = next
    resolved.value = applyTheme(next)
    media = window.matchMedia('(prefers-color-scheme: dark)')
    media.addEventListener('change', onSystemChange)
  }

  return { preference, resolved, setPreference, init }
})
