<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { safeNext } from '@/auth/guard'
import BrandMark from '@/components/BrandMark.vue'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import InlineText from '@/components/InlineText.vue'
import Input from '@/components/Input.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')

const banner = computed(() => {
  if (auth.banner === 'invalid') return t('signInError')
  if (auth.banner === 'rate') return t('signInRate')
  if (auth.banner === 'network') return t('signInNetwork')
  if (auth.banner === 'forbidden') return t('signInForbidden')
  return ''
})

async function submit() {
  if (auth.pending) return
  if (username.value.length === 0 || password.value.length === 0) return
  const ok = await auth.login(username.value, password.value)
  if (!ok) {
    if (auth.status === 'setup') await router.replace({ name: 'setup' })
    return
  }
  await router.replace(safeNext(route.query.redirect) ?? '/')
}

async function retry() {
  await auth.retry()
  if (auth.status === 'setup') await router.replace({ name: 'setup' })
  if (auth.status === 'authenticated') await router.replace(safeNext(route.query.redirect) ?? '/')
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
      <h1>{{ t('signInTitle') }}</h1>
      <p class="auth-lead">{{ t('signInLead') }}</p>
      <form class="auth-form" @submit.prevent="submit">
        <p v-if="banner" class="auth-banner" role="alert">{{ banner }}</p>
        <Input
          v-model="username"
          :label="t('setupUsername')"
          name="username"
          autocomplete="username"
          :spellcheck="false"
          required
        />
        <Input
          v-model="password"
          :label="t('setupPassword')"
          type="password"
          name="password"
          autocomplete="current-password"
          required
        />
        <Button class="auth-submit" type="submit" variant="primary" :disabled="auth.pending">
          {{ t('signInAction') }}
        </Button>
        <Button v-if="auth.banner === 'network'" type="button" variant="quiet" @click="retry">
          {{ t('nodeRetry') }}
        </Button>
      </form>
    </Card>
    <p class="gate-foot">
      <InlineText :text="t('forgotPassword')" />
    </p>
  </div>
</template>
