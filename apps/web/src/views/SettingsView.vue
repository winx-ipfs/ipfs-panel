<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@/components/Card.vue'
import Button from '@/components/Button.vue'
import PageHeader from '@/components/PageHeader.vue'
import Select from '@/components/Select.vue'
import Switch from '@/components/Switch.vue'
import Table from '@/components/Table.vue'
import Tabs from '@/components/Tabs.vue'
import type { AppLocale } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore, type ThemePreference } from '@/stores/theme'

const { t } = useI18n()
const localeStore = useLocaleStore()
const theme = useThemeStore()
const tab = ref('general')

const tabs = computed(() => [
  { id: 'general', label: t('settingsGeneral') },
  { id: 'about', label: t('settingsAbout') },
])

const language = computed({
  get: () => localeStore.locale,
  set: (value: string) => {
    if (value === 'en' || value === 'zh') localeStore.setLocale(value as AppLocale)
  },
})

const languageOptions = computed(() => [
  { value: 'en', label: t('languageNameEn') },
  { value: 'zh', label: t('languageNameZh') },
])

const followDevice = computed({
  get: () => theme.preference === 'system',
  set: (value: boolean) => {
    theme.setPreference(value ? 'system' : theme.resolved)
  },
})

function choose(next: ThemePreference) {
  theme.setPreference(next)
}
</script>

<template>
  <div class="page">
    <PageHeader :title="t('settingsTitle')" :description="t('settingsDescription')" />
    <Tabs v-model="tab" :tabs="tabs" :label="t('settingsTabs')">
      <template #general>
        <div class="stack">
          <Card>
            <Select v-model="language" :label="t('settingsLanguage')" :options="languageOptions" />
          </Card>
          <Card>
            <Switch v-model="followDevice" :label="t('themeFollow')" :hint="t('themeFollowHint')" />
            <div v-if="!followDevice" class="choice">
              <Button
                :variant="theme.preference === 'light' ? 'primary' : 'quiet'"
                :aria-pressed="theme.preference === 'light'"
                @click="choose('light')"
              >
                {{ t('themeLight') }}
              </Button>
              <Button
                :variant="theme.preference === 'dark' ? 'primary' : 'quiet'"
                :aria-pressed="theme.preference === 'dark'"
                @click="choose('dark')"
              >
                {{ t('themeDark') }}
              </Button>
            </div>
          </Card>
        </div>
      </template>
      <template #about>
        <Card>
          <p class="about-lead">{{ t('aboutBody') }}</p>
          <Table :caption="t('settingsAbout')" caption-hidden>
            <tbody>
              <tr>
                <th scope="row">{{ t('aboutProduct') }}</th>
                <td>ipfs-panel</td>
              </tr>
              <tr>
                <th scope="row">{{ t('aboutPortal') }}</th>
                <td>
                  <a href="https://winx.run" target="_blank" rel="noopener noreferrer">winx.run</a>
                </td>
              </tr>
              <tr>
                <th scope="row">{{ t('aboutScope') }}</th>
                <td>{{ t('aboutScopeValue') }}</td>
              </tr>
            </tbody>
          </Table>
        </Card>
      </template>
    </Tabs>
  </div>
</template>
