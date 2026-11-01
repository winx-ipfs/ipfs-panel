<script setup lang="ts">
import { Menu, Monitor, Moon, Sun } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref, watch, watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterView, useRoute } from 'vue-router'
import Dialog from '@/components/Dialog.vue'
import IconButton from '@/components/IconButton.vue'
import SidebarNav from '@/components/SidebarNav.vue'
import Toast from '@/components/Toast.vue'
import Tooltip from '@/components/Tooltip.vue'
import type { AppLocale } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore, type ThemePreference } from '@/stores/theme'

const { t } = useI18n()
const route = useRoute()
const localeStore = useLocaleStore()
const theme = useThemeStore()

const navOpen = ref(false)
const themeOpen = ref(false)
const narrow = ref(false)
const themeAnchor = ref<HTMLElement | null>(null)
let media: MediaQueryList | null = null

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

function onNarrow() {
  narrow.value = media?.matches ?? false
}

function chooseLocale(next: AppLocale) {
  localeStore.setLocale(next)
}

function chooseTheme(next: ThemePreference) {
  theme.setPreference(next)
  themeOpen.value = false
}

function onDocumentPointer(event: PointerEvent) {
  if (!themeOpen.value) return
  const target = event.target
  if (target instanceof Node && themeAnchor.value?.contains(target)) return
  themeOpen.value = false
}

function onDocumentKey(event: KeyboardEvent) {
  if (event.key === 'Escape') themeOpen.value = false
}

onMounted(() => {
  media = window.matchMedia('(max-width: 879px)')
  onNarrow()
  media.addEventListener('change', onNarrow)
  document.addEventListener('pointerdown', onDocumentPointer)
  document.addEventListener('keydown', onDocumentKey)
})

onUnmounted(() => {
  media?.removeEventListener('change', onNarrow)
  document.removeEventListener('pointerdown', onDocumentPointer)
  document.removeEventListener('keydown', onDocumentKey)
})

watch(narrow, (value) => {
  if (!value) navOpen.value = false
})

watch(
  () => route.fullPath,
  () => {
    navOpen.value = false
    themeOpen.value = false
  },
)

watchEffect(() => {
  const key = route.meta.titleKey
  const section = typeof key === 'string' ? t(key) : t('brand')
  document.title = `${section} · ${t('brand')}`
})
</script>

<template>
  <div class="shell">
    <a class="skip" href="#content">{{ t('skipToContent') }}</a>
    <aside class="sidebar">
      <SidebarNav />
    </aside>
    <div class="workspace">
      <header class="topbar">
        <div class="topbar-start">
          <IconButton class="menu-btn" :label="t('openMenu')" @click="navOpen = true">
            <Menu :size="18" :stroke-width="1.75" aria-hidden="true" />
          </IconButton>
          <span class="compact-brand">{{ t('brand') }}</span>
          <span class="machine">{{ t('machineLabel') }}</span>
        </div>
        <div class="topbar-end">
          <div class="segment" role="group" :aria-label="t('languageLabel')">
            <button type="button" :aria-pressed="localeStore.locale === 'en'" :aria-label="t('languageNameEn')" @click="chooseLocale('en')">
              {{ t('languageEn') }}
            </button>
            <button type="button" :aria-pressed="localeStore.locale === 'zh'" :aria-label="t('languageNameZh')" @click="chooseLocale('zh')">
              {{ t('languageZh') }}
            </button>
          </div>
          <div ref="themeAnchor" class="theme-anchor">
            <Tooltip :text="themeName">
              <IconButton
                :label="`${t('themeLabel')}: ${themeName}`"
                :aria-expanded="themeOpen"
                aria-haspopup="true"
                @click="themeOpen = !themeOpen"
              >
                <component :is="themeIcon" :size="18" :stroke-width="1.75" aria-hidden="true" />
              </IconButton>
            </Tooltip>
            <div v-if="themeOpen" class="theme-menu" role="group" :aria-label="t('themeLabel')">
              <button type="button" :aria-pressed="theme.preference === 'light'" @click="chooseTheme('light')">
                <Sun :size="16" :stroke-width="1.75" aria-hidden="true" />
                {{ t('themeLight') }}
              </button>
              <button type="button" :aria-pressed="theme.preference === 'dark'" @click="chooseTheme('dark')">
                <Moon :size="16" :stroke-width="1.75" aria-hidden="true" />
                {{ t('themeDark') }}
              </button>
              <button type="button" :aria-pressed="theme.preference === 'system'" @click="chooseTheme('system')">
                <Monitor :size="16" :stroke-width="1.75" aria-hidden="true" />
                {{ t('themeSystem') }}
              </button>
            </div>
          </div>
        </div>
      </header>
      <main id="content" class="main" tabindex="-1">
        <RouterView />
      </main>
    </div>
    <Dialog
      v-model:open="navOpen"
      variant="drawer"
      hide-title
      :title="t('navLabel')"
      :close-label="t('closeMenu')"
    >
      <SidebarNav @navigate="navOpen = false" />
    </Dialog>
    <Toast />
  </div>
</template>
