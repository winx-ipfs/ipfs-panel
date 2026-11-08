<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { passwordAcceptable, usernameAcceptable } from '@/auth/strength'
import BrandMark from '@/components/BrandMark.vue'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Input from '@/components/Input.vue'
import PasswordMeter from '@/components/PasswordMeter.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const confirm = ref('')
const setupToken = ref('')
const usernameTouched = ref(false)
const confirmTouched = ref(false)

const usernameError = computed(() => {
  if (!usernameTouched.value || username.value.length === 0) return ''
  return usernameAcceptable(username.value) ? '' : t('setupUsernameInvalid')
})

const confirmError = computed(() => {
  if (!confirmTouched.value || confirm.value.length === 0) return ''
  return confirm.value === password.value ? '' : t('setupMismatch')
})

const banner = computed(() => {
  if (auth.banner === 'network') return t('signInNetwork')
  if (auth.banner === 'invalid') return t('passwordWeak')
  if (auth.banner === 'forbidden') return auth.setupTokenRequired ? t('setupTokenMissing') : t('signInForbidden')
  return ''
})

const canSubmit = computed(() => {
  if (auth.pending) return false
  if (!usernameAcceptable(username.value)) return false
  if (!passwordAcceptable(password.value)) return false
  if (confirm.value !== password.value) return false
  if (auth.setupTokenRequired && setupToken.value.trim().length === 0) return false
  return true
})

async function submit() {
  usernameTouched.value = true
  confirmTouched.value = true
  if (auth.pending || !canSubmit.value) return
  const ok = await auth.setup(username.value, password.value, setupToken.value.trim())
  if (ok) {
    await router.replace('/')
    return
  }
  if (auth.status === 'anonymous') await router.replace({ name: 'login' })
}
</script>

<template>
  <div class="gate-column">
    <div class="gate-brand">
      <span class="gate-badge">
        <BrandMark />
      </span>
      <span class="gate-name">{{ t('brand') }}</span>
      <span class="gate-tag">{{ t('tagline') }}</span>
    </div>
    <Card class="auth-card">
      <h1>{{ t('setupTitle') }}</h1>
      <p class="auth-lead">{{ t('setupLead') }}</p>
      <form class="auth-form" @submit.prevent="submit">
        <p v-if="banner" class="auth-banner" role="alert">{{ banner }}</p>
        <Input
          v-model="username"
          :label="t('setupUsername')"
          :hint="t('setupUsernameHint')"
          :error="usernameError"
          name="username"
          autocomplete="username"
          :spellcheck="false"
          required
          @blur="usernameTouched = true"
        />
        <div class="auth-password">
          <Input
            v-model="password"
            :label="t('setupPassword')"
            type="password"
            name="new-password"
            autocomplete="new-password"
            :show-password-label="t('showPassword')"
            :hide-password-label="t('hidePassword')"
            required
          />
          <PasswordMeter :password="password" />
        </div>
        <Input
          v-model="confirm"
          :label="t('setupConfirm')"
          :error="confirmError"
          type="password"
          name="confirm-password"
          autocomplete="new-password"
          :show-password-label="t('showPassword')"
          :hide-password-label="t('hidePassword')"
          required
          @blur="confirmTouched = true"
        />
        <Input
          v-if="auth.setupTokenRequired"
          v-model="setupToken"
          :label="t('setupToken')"
          :hint="t('setupTokenHint')"
          name="setup-token"
          autocomplete="off"
          :spellcheck="false"
        />
        <Button class="auth-submit" type="submit" variant="primary" :disabled="!canSubmit">
          {{ t('setupAction') }}
        </Button>
      </form>
    </Card>
    <p class="gate-foot">{{ t('setupFoot') }}</p>
  </div>
</template>
