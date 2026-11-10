<script setup lang="ts">
import { FolderOpen, LayoutGrid, Pin, Server, Settings } from '@lucide/vue'
import { RouterLink, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BrandMark from '@/components/BrandMark.vue'
import { useAuthStore } from '@/stores/auth'

const emit = defineEmits<{ navigate: [] }>()
const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

async function logout() {
  emit('navigate')
  await auth.logout()
  await router.push({ name: 'login' })
}

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
    <RouterLink v-slot="{ href, navigate, isActive }" to="/files" custom>
      <a
        :href="href"
        class="nav-link"
        :class="{ active: isActive }"
        :aria-current="isActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <FolderOpen :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ t('navFiles') }}</span>
      </a>
    </RouterLink>
    <RouterLink v-slot="{ href, navigate, isActive }" to="/pins" custom>
      <a
        :href="href"
        class="nav-link"
        :class="{ active: isActive }"
        :aria-current="isActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <Pin :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ t('navPins') }}</span>
      </a>
    </RouterLink>
    <RouterLink v-slot="{ href, navigate, isActive }" to="/scenes" custom>
      <a
        :href="href"
        class="nav-link"
        :class="{ active: isActive }"
        :aria-current="isActive ? 'page' : undefined"
        @click="go(navigate, $event)"
      >
        <LayoutGrid :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="nav-text">{{ t('navScenes') }}</span>
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
    <button type="button" class="nav-logout" @click="logout">{{ t('logout') }}</button>
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
