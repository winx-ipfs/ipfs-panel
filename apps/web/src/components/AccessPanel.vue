<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  fetchAccess,
  fetchSessions,
  revokeOtherSessions,
  revokeSession,
} from '@/access/api'
import type { AccessSession, AccessSnapshot } from '@/access/model'
import Button from '@/components/Button.vue'
import Dialog from '@/components/Dialog.vue'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'

const { t, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const snap = ref<AccessSnapshot | null>(null)
const sessions = ref<AccessSession[]>([])
const loading = ref(true)
const failed = ref(false)
const dialog = ref(false)
const others = ref(false)
const target = ref<AccessSession | null>(null)
const busy = ref(false)

const banner = computed(() => {
  if (!snap.value) return null
  if (snap.value.remote && snap.value.insecure) return { tone: 'is-danger', text: t('accessInsecure') }
  if (snap.value.remote) return { tone: 'is-warning', text: t('accessRemote') }
  if (snap.value.trustProxy) return { tone: 'is-warning', text: t('accessProxy') }
  return null
})

const tlsLabel = computed(() => {
  if (!snap.value) return ''
  if (snap.value.tls === 'auto') return t('accessTLSAuto')
  if (snap.value.tls === 'file') return t('accessTLSFile')
  return t('accessTLSOff')
})

const hasOthers = computed(() => sessions.value.some((item) => !item.current))

function formatWhen(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

async function load() {
  loading.value = true
  failed.value = false
  const [access, listed] = await Promise.all([fetchAccess(), fetchSessions()])
  loading.value = false
  if (!access.snap || !listed.sessions) {
    failed.value = true
    return
  }
  snap.value = access.snap
  sessions.value = listed.sessions
}

function ask(row: AccessSession) {
  others.value = false
  target.value = row
  dialog.value = true
}

function askOthers() {
  others.value = true
  target.value = null
  dialog.value = true
}

async function copyFingerprint() {
  const value = snap.value?.fingerprint
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const area = document.createElement('textarea')
    area.value = value
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
  toast.show(t('accessCopied'))
}

async function confirm() {
  if (busy.value) return
  busy.value = true
  if (others.value) {
    const result = await revokeOtherSessions()
    busy.value = false
    if (result.status === 204) {
      dialog.value = false
      await load()
    }
    return
  }
  const row = target.value
  if (!row) {
    busy.value = false
    return
  }
  const result = await revokeSession(row.id)
  busy.value = false
  if (result.status !== 204) return
  dialog.value = false
  if (row.current) {
    auth.markSignedOut()
    await router.push({ name: 'login' })
    return
  }
  await load()
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="access-stack" :data-access="snap?.insecure ? 'insecure' : snap?.remote ? 'remote' : 'local'">
    <p v-if="loading" class="access-note">{{ t('accessLoading') }}</p>
    <p v-else-if="failed" class="access-banner is-warning" role="alert">{{ t('accessError') }}</p>
    <template v-else-if="snap">
      <p v-if="banner" class="access-banner" :class="banner.tone" role="status">{{ banner.text }}</p>
      <p v-else class="access-note">{{ t('accessLocal') }}</p>
      <section>
        <h2 class="access-caption">{{ t('accessSection') }}</h2>
        <div class="access-group">
          <div class="access-row">
            <span class="access-label">{{ t('accessListen') }}</span>
            <span class="access-value access-addresses mono">
              <span v-for="addr in snap.listen" :key="addr">{{ addr }}</span>
            </span>
          </div>
          <div v-if="snap.reachable.length" class="access-row">
            <span class="access-label">{{ t('accessReachable') }}</span>
            <span class="access-value access-addresses mono">
              <span v-for="addr in snap.reachable" :key="addr">{{ addr }}</span>
            </span>
          </div>
          <div class="access-row">
            <span class="access-label">{{ t('accessTLS') }}</span>
            <span class="access-value">{{ tlsLabel }}</span>
          </div>
          <div v-if="snap.fingerprint" class="access-row">
            <span class="access-label">{{ t('accessFingerprint') }}</span>
            <span class="access-value access-addresses">
              <span class="access-fp">{{ snap.fingerprint }}</span>
              <Button variant="quiet" @click="copyFingerprint">{{ t('accessCopy') }}</Button>
            </span>
          </div>
          <div class="access-row">
            <span class="access-label">{{ t('accessTrust') }}</span>
            <span class="access-value">{{ snap.trustProxy ? t('accessTrustOn') : t('accessTrustOff') }}</span>
          </div>
          <div v-if="snap.hosts.length" class="access-row">
            <span class="access-label">{{ t('accessHosts') }}</span>
            <span class="access-value mono">{{ snap.hosts.join(', ') }}</span>
          </div>
        </div>
      </section>
      <section>
        <h2 class="access-caption">{{ t('accessSessions') }}</h2>
        <div class="access-group">
          <div v-for="row in sessions" :key="row.id" class="access-row access-session">
            <div class="access-session-copy">
              <span>{{ row.current ? t('accessThisBrowser') : t('accessOtherBrowser') }}</span>
              <span class="access-meta">{{ t('accessFrom', { ip: row.ip || '—' }) }} · {{ formatWhen(row.seen) }}</span>
            </div>
            <Button variant="quiet" @click="ask(row)">{{ t('accessRevoke') }}</Button>
          </div>
          <p v-if="sessions.length === 0" class="access-row access-note">{{ t('accessEmptySessions') }}</p>
        </div>
      </section>
      <div v-if="hasOthers" class="access-actions">
        <Button variant="quiet" @click="askOthers">{{ t('accessRevokeOthers') }}</Button>
      </div>
      <p class="access-note">{{ t('accessKubo') }}</p>
      <p class="access-note">{{ t('accessRestart') }}</p>
    </template>
    <Dialog
      v-model:open="dialog"
      :title="others ? t('accessRevokeOthersTitle') : t('accessRevokeTitle')"
      :close-label="t('closeDialog')"
    >
      <p>
        {{
          others
            ? t('accessRevokeOthersBody')
            : target?.current
              ? t('accessRevokeCurrent')
              : t('accessRevokeOther')
        }}
      </p>
      <div class="choice">
        <Button variant="quiet" data-autofocus @click="dialog = false">{{ t('accessCancel') }}</Button>
        <Button variant="primary" :disabled="busy" @click="confirm">{{ t('accessRevokeConfirm') }}</Button>
      </div>
    </Dialog>
  </div>
</template>
