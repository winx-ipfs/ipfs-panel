<script setup lang="ts">
import { Menu } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref, watch, watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterView, useRoute } from 'vue-router'
import BrandMark from '@/components/BrandMark.vue'
import Dialog from '@/components/Dialog.vue'
import IconButton from '@/components/IconButton.vue'
import LocaleThemeControls from '@/components/LocaleThemeControls.vue'
import SidebarNav from '@/components/SidebarNav.vue'
import Toast from '@/components/Toast.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const route = useRoute()
const auth = useAuthStore()

const navOpen = ref(false)
const narrow = ref(false)
let media: MediaQueryList | null = null

const authReady = computed(() => auth.status !== 'loading')
const gate = computed(() => !authReady.value || route.name === 'login' || route.name === 'setup')

function onNarrow() {
  narrow.value = media?.matches ?? false
}

onMounted(() => {
  media = window.matchMedia('(max-width: 879px)')
  onNarrow()
  media.addEventListener('change', onNarrow)
})

onUnmounted(() => {
  media?.removeEventListener('change', onNarrow)
})

watch(narrow, (value) => {
  if (!value) navOpen.value = false
})

watch(
  () => route.fullPath,
  () => {
    navOpen.value = false
  },
)

watchEffect(() => {
  const key = route.meta.titleKey
  const section = typeof key === 'string' ? t(key) : t('brand')
  document.title = `${section} · ${t('brand')}`
})
</script>

<template>
  <div v-if="gate" class="gate">
    <a class="skip" href="#content">{{ t('skipToContent') }}</a>
    <header class="gate-bar">
      <LocaleThemeControls />
    </header>
    <main id="content" class="gate-main" tabindex="-1">
      <RouterView v-if="authReady" />
      <div v-else class="gate-column gate-loading">
        <span class="gate-badge">
          <BrandMark />
        </span>
        <p class="sr-only" role="status">{{ t('authLoading') }}</p>
      </div>
    </main>
  </div>
  <div v-else class="shell">
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
          <LocaleThemeControls />
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
