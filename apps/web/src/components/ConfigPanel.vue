<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Button from '@/components/Button.vue'
import Dialog from '@/components/Dialog.vue'
import EmptyState from '@/components/EmptyState.vue'
import JsonCode from '@/components/JsonCode.vue'
import Skeleton from '@/components/Skeleton.vue'
import { diffLines, type DiffRow } from '@/config/diff'
import { configErrorKey, jsonErrorLine, sceneDiverged, sceneTitleKey, type ConfigDocument } from '@/config/model'
import { useConfigStore } from '@/stores/config'
import { useNodeStore } from '@/stores/node'
import { useToastStore } from '@/stores/toast'

const props = defineProps<{ active: boolean }>()

const { t } = useI18n()
const store = useConfigStore()
const node = useNodeStore()
const toast = useToastStore()

const text = ref('')
const loaded = ref('')
const scene = ref<string | null>(null)
const sceneKeys = ref<string[]>([])
const backup = ref(false)
const loading = ref(true)
const stopped = ref(false)
const error = ref('')
const detail = ref('')
const diffOpen = ref(false)
const restartOpen = ref(false)
const restoreOpen = ref(false)
const leaveOpen = ref(false)
const savedDiverged = ref(false)
const savedScene = ref<string | null>(null)
let leaveDecision: ((value: boolean) => void) | null = null

const dirty = computed(() => text.value !== loaded.value)
const errorLine = computed(() => (text.value.length === 0 ? null : jsonErrorLine(text.value)))
const invalid = computed(() => errorLine.value != null)
const canSave = computed(() => props.active && !loading.value && !stopped.value && !invalid.value && dirty.value && !store.saving)
const diverged = computed(() => sceneDiverged(loaded.value, text.value, scene.value, sceneKeys.value))
const rows = computed<DiffRow[] | 'too_large'>(() => diffLines(loaded.value, text.value))
const diffTooLarge = computed(() => rows.value === 'too_large')
const diffRows = computed(() => (rows.value === 'too_large' ? [] : rows.value))
const state = computed(() => {
  if (loading.value) return 'loading'
  if (stopped.value) return 'stopped'
  if (error.value && !text.value) return 'error'
  if (restartOpen.value) return 'saved'
  if (diffOpen.value) return 'diff'
  if (invalid.value) return 'invalid'
  if (dirty.value) return 'dirty'
  return 'ready'
})

function sceneName(id: string | null): string {
  if (!id) return ''
  const key = sceneTitleKey(id)
  return key === 'configNoScene' ? id : t(key)
}

function applyDoc(doc: ConfigDocument) {
  text.value = doc.text
  loaded.value = doc.text
  scene.value = doc.scene
  sceneKeys.value = doc.sceneKeys
  backup.value = doc.backup
  error.value = ''
  detail.value = ''
  stopped.value = false
}

async function load() {
  loading.value = true
  error.value = ''
  const shot = await devShot()
  if (shot === 'loading') return
  if (shot === 'stopped') {
    loading.value = false
    stopped.value = true
    return
  }
  if (shot === 'error') {
    loading.value = false
    error.value = 'unavailable'
    return
  }
  if (shot) {
    const doc = shotDocument(shot)
    applyDoc(doc)
    if (shot === 'dirty') text.value = doc.text.replace('"Enabled": true', '"Enabled": false')
    if (shot === 'diff') text.value = doc.text.replace('"autoclient"', '"auto"')
    if (shot === 'invalid') text.value = `${doc.text.trimEnd().slice(0, -1)},\n}\n`
    loading.value = false
    if (shot === 'diff') diffOpen.value = true
    if (shot === 'saved') restartOpen.value = true
    return
  }
  const result = await store.load()
  loading.value = false
  if (!result.ok) {
    if (result.code === 'node_not_running') {
      stopped.value = true
      return
    }
    error.value = result.code
    detail.value = result.detail
    return
  }
  applyDoc(result.doc)
}

function shotDocument(shot: string): ConfigDocument {
  return {
    text: sample,
    scene: shot === 'saved' ? null : 'quiet',
    sceneKeys: ['Routing.Type', 'Addresses.Gateway'],
    backup: shot === 'saved' || shot === 'ready' || shot === 'dirty' || shot === 'diff' || shot === 'invalid',
  }
}

async function devShot(): Promise<string | null> {
  if (!import.meta.env.DEV) return null
  const mod = await import('@/dev/mockConfig')
  return mod.configShot(window.location.search)
}

function revert() {
  text.value = loaded.value
  error.value = ''
  detail.value = ''
}

function openDiff() {
  if (!canSave.value) return
  diffOpen.value = true
}

async function confirmSave() {
  const leaving = scene.value
  const result = await store.save(text.value)
  diffOpen.value = false
  if (!result.ok) {
    error.value = result.code
    detail.value = result.detail
    if (result.code === 'node_not_running') stopped.value = true
    return
  }
  savedDiverged.value = result.doc.diverged
  savedScene.value = leaving
  applyDoc(result.doc)
  if (result.doc.restart === 'needed') restartOpen.value = true
  else toast.show(t('configUnchanged'))
}

async function confirmRestore() {
  restoreOpen.value = false
  const result = await store.restore()
  if (!result.ok) {
    error.value = result.code
    detail.value = result.detail
    return
  }
  savedDiverged.value = result.doc.diverged
  applyDoc(result.doc)
  if (result.doc.restart === 'needed') restartOpen.value = true
  else toast.show(t('configUnchanged'))
}

async function restart() {
  restartOpen.value = false
  await node.restart()
  toast.show(t('configRestarted'))
}

function onKey(event: KeyboardEvent) {
  if (!props.active) return
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    openDiff()
  }
}

function onUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  leaveOpen.value = true
  return new Promise<boolean>((resolve) => {
    leaveDecision = resolve
  })
})

function answerLeave(discard: boolean) {
  const decide = leaveDecision
  leaveDecision = null
  leaveOpen.value = false
  decide?.(discard)
}

watch(leaveOpen, (open) => {
  if (open || !leaveDecision) return
  const decide = leaveDecision
  leaveDecision = null
  decide(false)
})

onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('beforeunload', onUnload)
  void load()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('beforeunload', onUnload)
})

const sample = `{
  "Addresses": {
    "API": "/ip4/127.0.0.1/tcp/5001",
    "Gateway": "/ip4/127.0.0.1/tcp/8081"
  },
  "Discovery": {
    "MDNS": {
      "Enabled": true
    }
  },
  "Identity": {
    "PeerID": "12D3KooWRjSZta6ExiKPiP5miEzXn2J4bM4cT7aB9sample"
  },
  "Routing": {
    "Type": "autoclient"
  }
}
`
</script>

<template>
  <section class="config-panel" :data-state="state">
    <div class="config-head">
      <span class="config-expert">{{ t('configExpert') }}</span>
      <p>{{ t('configLead') }}</p>
    </div>

    <div v-if="loading" class="config-skeleton" aria-busy="true">
      <p class="sr-only" role="status">{{ t('configLoading') }}</p>
      <Skeleton width="28%" height="18px" />
      <Skeleton width="86%" height="14px" />
      <Skeleton width="100%" height="320px" />
    </div>

    <EmptyState v-else-if="stopped" :title="t('configStoppedTitle')" :description="t('configStoppedBody')">
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('configStoppedAction') }}</RouterLink>
      </template>
    </EmptyState>

    <template v-else>
      <p v-if="error" class="config-error" role="alert">
        {{ t(configErrorKey(error)) }}
        <span v-if="detail" class="config-detail">{{ detail }}</span>
      </p>
      <template v-if="text">
        <p v-if="scene" class="config-note">{{ t('configScene', { scene: sceneName(scene) }) }}</p>
        <p v-else class="config-note">{{ t('configNoScene') }}</p>
      </template>

      <div v-if="text" class="config-work">
        <div class="config-toolbar">
          <Button variant="primary" :disabled="!canSave" @click="openDiff">{{ t('configSave') }}</Button>
          <Button variant="quiet" :disabled="!dirty || store.saving" @click="revert">{{ t('configRevert') }}</Button>
          <Button v-if="backup" variant="quiet" :disabled="store.saving" @click="restoreOpen = true">
            {{ t('configRestore') }}
          </Button>
          <span class="config-keys">{{ t('configSaveKey') }}</span>
        </div>
        <p class="config-status" role="status">
          <template v-if="invalid">{{ t('configInvalid', { line: errorLine }) }}</template>
          <template v-else>{{ t('configValid') }} {{ dirty ? t('configDirty') : t('configClean') }}</template>
        </p>
        <div class="config-editor">
          <JsonCode v-model="text" :label="t('configLead')" :invalid-label="t('configInvalid', { line: errorLine ?? 1 })" :read-only="store.saving" />
        </div>
        <p class="config-footnote">{{ t('configPrivate') }}</p>
      </div>

      <div v-else class="config-actions">
        <Button variant="primary" @click="load">{{ t('configRetry') }}</Button>
      </div>
    </template>

    <Dialog v-model:open="diffOpen" :title="t('configDiffTitle')" :close-label="t('closeDialog')" scrollable>
      <p>{{ t('configDiffBody') }}</p>
      <p v-if="diverged && scene" class="config-note">{{ t('configDiffScene', { scene: sceneName(scene) }) }}</p>
      <p v-if="diffTooLarge" class="config-note">{{ t('configDiffLarge') }}</p>
      <div v-else class="config-diff" role="region" :aria-label="t('configDiffTitle')">
        <p v-if="diffRows.every((row) => row.kind === 'same' || row.kind === 'skip')" class="config-note">{{ t('configDiffEmpty') }}</p>
        <div
          v-for="(row, index) in diffRows"
          :key="index"
          class="config-diff-line"
          :class="row.kind === 'add' ? 'is-add' : row.kind === 'del' ? 'is-del' : row.kind === 'skip' ? 'is-skip' : ''"
        >
          <template v-if="row.kind === 'skip'">… {{ row.count }}</template>
          <template v-else>
            <span aria-hidden="true">{{ row.kind === 'add' ? '+' : row.kind === 'del' ? '−' : ' ' }}</span>
            <span>{{ row.text }}</span>
          </template>
        </div>
      </div>
      <div class="config-actions">
        <Button variant="quiet" data-autofocus @click="diffOpen = false">{{ t('configCancel') }}</Button>
        <Button variant="primary" :disabled="store.saving" @click="confirmSave">{{ t('configConfirm') }}</Button>
      </div>
    </Dialog>

    <Dialog v-model:open="restartOpen" :title="t('configSavedTitle')" :close-label="t('closeDialog')">
      <p>{{ t('configSavedBody') }}</p>
      <p v-if="savedDiverged && savedScene" class="config-note">{{ t('configDiffScene', { scene: sceneName(savedScene) }) }}</p>
      <div class="config-actions">
        <Button variant="quiet" data-autofocus @click="restartOpen = false">{{ t('configDismiss') }}</Button>
        <Button variant="primary" @click="restart">{{ t('configRestart') }}</Button>
      </div>
    </Dialog>

    <Dialog v-model:open="restoreOpen" :title="t('configRestoreTitle')" :close-label="t('closeDialog')">
      <p>{{ t('configRestoreBody') }}</p>
      <div class="config-actions">
        <Button variant="quiet" data-autofocus @click="restoreOpen = false">{{ t('configCancel') }}</Button>
        <Button variant="primary" :disabled="store.saving" @click="confirmRestore">{{ t('configRestoreConfirm') }}</Button>
      </div>
    </Dialog>

    <Dialog v-model:open="leaveOpen" :title="t('configLeaveTitle')" :close-label="t('closeDialog')">
      <p>{{ t('configLeaveBody') }}</p>
      <div class="config-actions">
        <Button variant="quiet" data-autofocus @click="answerLeave(false)">{{ t('configLeaveStay') }}</Button>
        <Button variant="primary" @click="answerLeave(true)">{{ t('configLeaveDiscard') }}</Button>
      </div>
    </Dialog>
  </section>
</template>
