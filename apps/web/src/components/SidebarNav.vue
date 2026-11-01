<script setup lang="ts">
import { FolderOpen, LayoutGrid, Pin, Server, Settings } from '@lucide/vue'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BrandMark from '@/components/BrandMark.vue'

const emit = defineEmits<{ navigate: [] }>()
const { t } = useI18n()

const later = computed(() => [
  { to: '/files', label: t('navFiles'), icon: FolderOpen },
  { to: '/pins', label: t('navPins'), icon: Pin },
  { to: '/scenes', label: t('navScenes'), icon: LayoutGrid },
])

function go(navigate: (event: MouseEvent) => void, event: MouseEvent) {
  navigate(event)
  emit('navigate')
}
</script>

<template>
  <div class="brand">
    <span class="brand-badge">
      <BrandMark />
    </span>
    <span class="brand-copy">
      <span class="brand-name">{{ t('brand') }}</span>
      <span class="brand-tag">{{ t('tagline') }}</span>
    </span>
  </div>
  <nav class="nav" :aria-label="t('navLabel')">
    <RouterLink v-slot="{ href, navigate, isExactActive }" to="/" custom>
      <a
        :href="href"
        class="nav-link"
        :class="{ active: isExactActive }"
        :aria-current="isExactActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <Server :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ t('navNode') }}</span>
      </a>
    </RouterLink>
    <p class="nav-label">{{ t('navLater') }}</p>
    <RouterLink v-for="item in later" :key="item.to" v-slot="{ href, navigate, isActive }" :to="item.to" custom>
      <a
        :href="href"
        class="nav-link soon"
        :class="{ active: isActive }"
        :aria-current="isActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <component :is="item.icon" :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ item.label }}</span>
        <span class="soon-badge">{{ t('navSoon') }}</span>
      </a>
    </RouterLink>
    <RouterLink v-slot="{ href, navigate, isActive }" to="/settings" custom>
      <a
        :href="href"
        class="nav-link is-settings"
        :class="{ active: isActive }"
        :aria-current="isActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <Settings :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ t('navSettings') }}</span>
      </a>
    </RouterLink>
  </nav>
  <div class="sidebar-foot">
    <a
      class="portal"
      href="https://winx.run"
      target="_blank"
      rel="noopener noreferrer"
      :aria-label="t('portalLabel')"
    >
      winx.run
    </a>
  </div>
</template>
