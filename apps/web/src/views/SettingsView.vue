<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { passwordAcceptable } from '@/auth/strength'
import Card from '@/components/Card.vue'
import Button from '@/components/Button.vue'
import InlineText from '@/components/InlineText.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import PasswordMeter from '@/components/PasswordMeter.vue'
import Select from '@/components/Select.vue'
import Switch from '@/components/Switch.vue'
import Table from '@/components/Table.vue'
import Tabs from '@/components/Tabs.vue'
import type { AppLocale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore, type ThemePreference } from '@/stores/theme'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const localeStore = useLocaleStore()
const theme = useThemeStore()
const auth = useAuthStore()
const toast = useToastStore()
const tab = ref('general')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const confirmTouched = ref(false)
const passwordError = ref('')

const tabs = computed(() => [
  { id: 'general', label: t('settingsGeneral') },
  { id: 'account', label: t('accountTab') },
  { id: 'about', label: t('settingsAbout') },
])

const confirmError = computed(() => {
  if (!confirmTouched.value || confirmPassword.value.length === 0) return ''
  return confirmPassword.value === newPassword.value ? '' : t('setupMismatch')
})

const canChange = computed(() => {
  if (auth.pending) return false
  if (currentPassword.value.length === 0) return false
  if (!passwordAcceptable(newPassword.value)) return false
  return confirmPassword.value === newPassword.value
})

async function changePassword() {
  confirmTouched.value = true
  passwordError.value = ''
  if (auth.pending || !canChange.value) return
  const result = await auth.changePassword(currentPassword.value, newPassword.value)
  if (result === 'ok') {
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    confirmTouched.value = false
    toast.show(t('passwordChanged'))
    return
  }
  if (result === 'invalid') passwordError.value = t('passwordWrong')
  else if (result === 'same') passwordError.value = t('passwordSame')
  else if (result === 'weak') passwordError.value = t('passwordWeak')
  else if (result === 'rate') passwordError.value = t('signInRate')
  else passwordError.value = t('signInNetwork')
}

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
      <template #account>
        <Card>
          <h2 class="side-title">{{ t('passwordTitle') }}</h2>
          <p class="auth-lead">{{ t('passwordLead') }}</p>
          <p v-if="auth.username" class="signed-in">{{ t('signedInAs', { name: auth.username }) }}</p>
          <form class="auth-form" @submit.prevent="changePassword">
            <p v-if="passwordError" class="auth-banner" role="alert">{{ passwordError }}</p>
            <Input
              v-model="currentPassword"
              :label="t('currentPassword')"
              type="password"
              name="current-password"
              autocomplete="current-password"
              required
            />
            <div class="auth-password">
              <Input
                v-model="newPassword"
                :label="t('newPassword')"
                type="password"
                name="new-password"
                autocomplete="new-password"
                required
              />
              <PasswordMeter :password="newPassword" />
            </div>
            <Input
              v-model="confirmPassword"
              :label="t('confirmPassword')"
              :error="confirmError"
              type="password"
              name="confirm-password"
              autocomplete="new-password"
              required
              @blur="confirmTouched = true"
            />
            <Button type="submit" variant="primary" :disabled="auth.pending">
              {{ t('changePassword') }}
            </Button>
          </form>
          <p class="reset-hint">
            <InlineText :text="t('forgotPassword')" />
          </p>
        </Card>
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
