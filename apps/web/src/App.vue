<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useLocaleStore } from '@/stores/locale'
import type { AppLocale } from '@/i18n'

const { t } = useI18n()
const localeStore = useLocaleStore()

function choose(next: AppLocale) {
  localeStore.setLocale(next)
}
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
        <span class="brand-name">{{ t('brand') }}</span>
        <span class="tagline">{{ t('tagline') }}</span>
      </div>
      <div class="languages" role="group" :aria-label="t('languageLabel')">
        <button
          type="button"
          :aria-pressed="localeStore.locale === 'en'"
          @click="choose('en')"
        >
          {{ t('languageEn') }}
        </button>
        <button
          type="button"
          :aria-pressed="localeStore.locale === 'zh'"
          @click="choose('zh')"
        >
          {{ t('languageZh') }}
        </button>
      </div>
    </header>
    <div class="body">
      <nav class="nav" :aria-label="t('navLabel')">
        <RouterLink to="/">{{ t('navNode') }}</RouterLink>
      </nav>
      <main class="main">
        <RouterView />
      </main>
    </div>
  </div>
</template>
