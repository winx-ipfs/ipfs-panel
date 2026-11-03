<script setup lang="ts">
import { Monitor, Moon, Sun } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Tooltip from '@/components/Tooltip.vue'
import type { AppLocale } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore, type ThemePreference } from '@/stores/theme'

const { t } = useI18n()
const localeStore = useLocaleStore()
const theme = useThemeStore()
const themeOrder: ThemePreference[] = ['light', 'dark', 'system']

const themeIcon = computed(() => {
  if (theme.preference === 'light') return Sun
  if (theme.preference === 'dark') return Moon
  return Monitor
})

const themeName = computed(() => {
  if (theme.preference === 'light') return t('themeLight')
  if (theme.preference === 'dark') return t('themeDark')
  return t('themeSystem')
})

const themeButtonLabel = computed(() => `${t('themeLabel')}: ${themeName.value}`)

function chooseLocale(next: AppLocale) {
  localeStore.setLocale(next)
}

function cycleTheme() {
  const index = themeOrder.indexOf(theme.preference)
  const next = themeOrder[(index + 1) % themeOrder.length] ?? 'system'
  theme.setPreference(next)
}
</script>

<template>
  <div class="locale-theme">
    <div class="segment" role="group" :aria-label="t('languageLabel')">
      <button type="button" :aria-pressed="localeStore.locale === 'en'" :aria-label="t('languageNameEn')" @click="chooseLocale('en')">
        {{ t('languageEn') }}
      </button>
      <button type="button" :aria-pressed="localeStore.locale === 'zh'" :aria-label="t('languageNameZh')" @click="chooseLocale('zh')">
        {{ t('languageZh') }}
      </button>
    </div>
    <Tooltip :text="themeName">
      <button type="button" class="theme-cycle" :aria-label="themeButtonLabel" @click="cycleTheme">
        <component :is="themeIcon" :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="theme-cycle-name">{{ themeName }}</span>
      </button>
    </Tooltip>
  </div>
</template>
